package agentserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/hpcloud/tail"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	cdrclient "github.com/rakutentech/shibuya/shibuya/coordinator/client"
	"github.com/rakutentech/shibuya/shibuya/coordinator/payload"
	"github.com/rakutentech/shibuya/shibuya/coordinator/storage"
	"github.com/rakutentech/shibuya/shibuya/engines/containerstats"
	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
	httproute "github.com/rakutentech/shibuya/shibuya/http/route"
	"github.com/rakutentech/shibuya/shibuya/scheduler/k8s"
	"github.com/reqfleet/pubsub/client"
	"github.com/reqfleet/pubsub/messages"
)

var (
	STDERR = "/dev/stderr"
)

type AgentServer struct {
	incomingClients chan chan string
	closingClients  chan chan string
	clients         map[chan string]struct{}
	bus             chan string
	fileId          int
	process         *os.Process
	ctx             context.Context
	cancel          context.CancelFunc
	cdrclient       *cdrclient.Client
	options         AgentServerOptions
	reqOpts         cdrclient.ReqOpts
	reader          io.ReadCloser
	writer          io.Writer
	logger          *log.Entry
	angentDir       AgentDir
	runID           int64
	mu              sync.RWMutex
}

func NewAgentServer(opts AgentServerOptions) *AgentServer {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	reader, writer, _ := os.Pipe()
	mw := io.MultiWriter(writer, os.Stderr)
	as := &AgentServer{
		incomingClients: make(chan chan string),
		closingClients:  make(chan chan string),
		clients:         make(map[chan string]struct{}),
		bus:             make(chan string),
		options:         opts,
		cdrclient:       cdrclient.NewClient(httpClient),
		reqOpts:         opts.EngineMeta.MakeReqOpts(),
		reader:          reader,
		writer:          mw,
		logger:          opts.Logger,
		angentDir:       NewAgentDirHandler(""),
	}
	log.SetOutput(mw)
	go as.listenForSubscribers()
	return as
}

func (as *AgentServer) makePromMetrics(line string) {
	metricParser := as.options.MetricParser
	metric, err := metricParser(line)
	// we need to pass the engine meta(project, collection, plan), especially run id
	// Run id is generated at controller side
	if err != nil {
		return
	}
	em := as.options.EngineMeta
	metric.CollectionID = em.CollectionID
	metric.PlanID = em.PlanID
	metric.EngineID = fmt.Sprintf("%d", em.EngineID)
	metric.RunID = fmt.Sprintf("%d", as.runID)

	metric.ToPrometheus()
}

func (as *AgentServer) listenForSubscribers() {
	for {
		select {
		case s := <-as.incomingClients:
			// A new client has connected.
			// Register their message channel
			as.clients[s] = struct{}{}
			as.logger.Infof("Metric subscriber added. %d registered subscribers", len(as.clients))
		case s := <-as.closingClients:
			// A client has dettached and we want to
			// stop sending them messages.
			delete(as.clients, s)
			close(s)
			as.logger.Infof("Metric subscriber removed. %d registered subscribers", len(as.clients))
		case event := <-as.bus:
			// We got a new event from the outside!
			// Send eveent to all connected clients
			as.makePromMetrics(event)
			for clientMessageChan := range as.clients {
				clientMessageChan <- event
			}
		}
	}
}

func (as *AgentServer) tailFunc(filepath string) {
	var t *tail.Tail
	var err error
	for {
		t, err = tail.TailFile(filepath, tail.Config{MustExist: true, Follow: true, Poll: true})
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		break
	}
	as.logger.Infof("Start tailing result file %s", filepath)
	for {
		select {
		case <-as.ctx.Done():
			t.Stop()
			as.logger.Infof("Stop tailing the result file %s", filepath)
			return
		case line := <-t.Lines:
			as.bus <- line.Text
		}
	}
}

func (as *AgentServer) reportOwnMetrics(interval time.Duration) error {
	prev := uint64(0)
	engineMeta := as.options.EngineMeta
	engineID := engineMeta.EngineID
	collectionID := engineMeta.CollectionID
	planID := engineMeta.PlanID
	engineNumber := strconv.Itoa(engineID)
	for {
		time.Sleep(interval)
		cpuUsage, err := containerstats.ReadCPUUsage()
		if err != nil {
			return err
		}
		if prev == 0 {
			prev = cpuUsage
			continue
		}
		used := (cpuUsage - prev) / uint64(interval.Seconds()) / 1000
		prev = cpuUsage
		memoryUsage, err := containerstats.ReadMemoryUsage()
		if err != nil {
			return err
		}
		enginesModel.CpuGauge.WithLabelValues(collectionID,
			planID, engineNumber).Set(float64(used))
		enginesModel.MemGauge.WithLabelValues(collectionID,
			planID, engineNumber).Set(float64(memoryUsage))
	}
}

func (as *AgentServer) StreamHandler(w http.ResponseWriter, r *http.Request) {
	messageChan := make(chan string)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return

	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Signal the sw that we have a new connection
	as.incomingClients <- messageChan
	// Listen to connection close and un-register messageChan
	notify := w.(http.CloseNotifier).CloseNotify()

	go func() {
		<-notify
		as.closingClients <- messageChan
	}()

	for message := range messageChan {
		if message == "" {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", message)
		flusher.Flush()
	}
}

func (as *AgentServer) SubscribeToCoordinator() (chan messages.Message, error) {
	collectionID := as.options.EngineMeta.CollectionID
	client := &client.PubSubClient{Addr: fmt.Sprintf("%s:2416", as.reqOpts.Endpoint),
		Password: as.reqOpts.APIKey}
	var msgChan chan messages.Message
	var err error
	// TODO add timeout here
	for {
		time.Sleep(2 * time.Second)
		msgChan, _, err = client.Subscribe(fmt.Sprintf("collection:%s", collectionID), &payload.Payload{})
		if err != nil {
			continue
		}
		break
	}
	as.logger.Infof("Subscribe to coordinator at %s", as.reqOpts.Endpoint)
	return msgChan, nil
}

func (as *AgentServer) assignCtx(ctx context.Context, cancel context.CancelFunc) {
	as.mu.Lock()
	as.cancel = cancel
	as.ctx = ctx
	as.mu.Unlock()
}

func (as *AgentServer) stopTestByCancel() {
	as.mu.RLock()
	defer as.mu.RUnlock()

	if as.cancel == nil {
		return
	}
	as.cancel()
}

func (as *AgentServer) finishCommand() {
	for {
		select {
		case <-as.ctx.Done():
			defer func() {
				as.process = nil
				em := as.options.EngineMeta
				as.cdrclient.ReportProgress(as.reqOpts, em.CollectionID, em.PlanID, em.EngineID, false)
			}()
			stopCommand := as.options.StopCommand
			if stopCommand == nil && as.process != nil {
				as.logger.Infof("No stop command is provided. Going to kill the process %d", as.process.Pid)
				if err := as.process.Kill(); err != nil {
					as.logger.Error(err)
				}
				return
			}
			as.logger.Infof("Shutting down process %d", as.process.Pid)
			cmd := stopCommand.ToExec(nil)
			if err := cmd.Run(); err != nil {
				as.logger.Error(err)
			}
			return
		}
	}
}

func (as *AgentServer) runCommand(runID int64) error {
	// command will wait for the shutdown signal. Once it's done, the command
	// func should finish
	resultFileFunc := as.options.ResultFileFunc
	filename := as.angentDir.ResultFilesDir().resultFile(resultFileFunc(as.fileId))
	extraArgs := as.options.ExtraArgs
	extraArgs = append(extraArgs, filename)
	command := as.options.StartCommand.ToExec(extraArgs)
	as.logger.Infof("command is %s", command.String())
	command.Stderr = as.writer
	if err := command.Start(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	as.assignCtx(ctx, cancel)
	as.process = command.Process
	// Increase the fileid for next run
	as.fileId += 1
	as.runID = runID
	go as.tailFunc(filename)
	go as.finishCommand()
	em := as.options.EngineMeta
	as.cdrclient.ReportProgress(as.options.EngineMeta.MakeReqOpts(),
		em.CollectionID, em.PlanID, em.EngineID, true)
	go func() {
		command.Wait()
		// The command could be stopped earlier. Calling the cancel func will have no effect.
		as.cancel()
	}()
	return nil
}

func (as *AgentServer) handleStart(payload *payload.EngineMessage) error {
	if err := as.angentDir.TestFilesDir().reset(); err != nil {
		return err
	}
	engineMeta := as.options.EngineMeta
	collectionID := engineMeta.CollectionID
	planID := engineMeta.PlanID
	engineID := engineMeta.EngineID
	pf := storage.NewPlanFiles("", collectionID, planID)
	client := as.cdrclient
	content, err := client.FetchFile(as.reqOpts, pf.TestFilePath(payload.TestFile))
	if err != nil {
		return err
	}
	if err := as.angentDir.TestFilesDir().saveFile(as.options.TestFileName, content); err != nil {
		return err
	}
	for dt := range payload.DataFiles {
		content, err := client.FetchFile(as.reqOpts, pf.EngineDataPath(dt, engineID))
		if err != nil {
			return err
		}
		if err := as.angentDir.TestFilesDir().saveFile(dt, content); err != nil {
			return err
		}
	}
	return as.runCommand(payload.RunID)
}

func (as *AgentServer) ListenForEvents(msgChan chan messages.Message) {
	engineMeta := as.options.EngineMeta
	for msg := range msgChan {
		pl := msg.(*payload.Payload)
		planMsg := pl.PlanMessage[engineMeta.PlanID]
		switch pl.Verb {
		case "start":
			if err := as.handleStart(planMsg); err != nil {
				as.logger.Error(err)
			}
		case "stop":
			_, ok := pl.PlanMessage[engineMeta.PlanID]
			if !ok {
				continue
			}
			as.stopTestByCancel()
		}
	}
}

type EngineMeta struct {
	CoordinatorIP string
	CollectionID  string
	PlanID        string
	EngineID      int
	APIKey        string
}

func (em EngineMeta) MakeReqOpts() cdrclient.ReqOpts {
	return cdrclient.ReqOpts{
		Endpoint: em.CoordinatorIP,
		APIKey:   em.APIKey,
	}
}

type AgentServerOptions struct {
	EngineMeta     EngineMeta
	TestFileName   string
	StartCommand   Command
	ExtraArgs      []string
	StopCommand    *Command
	MetricParser   func(string) (enginesModel.ShibuyaMetric, error)
	ResultFileFunc func(fileID int) string
	Logger         *log.Entry
}

func StartAgentServer(options AgentServerOptions) (*AgentServer, error) {
	if options.Logger == nil {
		options.Logger = log.WithFields(log.Fields{
			"Source": "shibuya-agent",
		})
	}
	as := NewAgentServer(options)
	go func() {
		if err := as.reportOwnMetrics(5 * time.Second); err != nil {
			options.Logger.Fatal(err)
		}
	}()
	msgChan, err := as.SubscribeToCoordinator()
	if err != nil {
		return nil, err
	}
	go as.ListenForEvents(msgChan)
	router := httproute.NewRouter("agent http endpoints", "")
	router.AddRoutes(httproute.Routes{
		{
			Path:        "/stream",
			Method:      "GET",
			HandlerFunc: as.StreamHandler,
		},
		{
			Path:        "/metrics",
			Method:      "GET",
			HandlerFunc: promhttp.Handler().ServeHTTP,
		},
	})
	if err := http.ListenAndServe(":8080", router.Mux()); err != nil {
		options.Logger.Fatal(err)
	}
	return nil, nil
}

func FetchEngineMeta() EngineMeta {
	engineID, err := k8s.ExtractEngineIDFromName(os.Getenv("engine_name"))
	if err != nil {
		log.Fatal(err)
	}
	return EngineMeta{
		CoordinatorIP: os.Getenv("coordinator_ip"),
		CollectionID:  os.Getenv("collection_id"),
		PlanID:        os.Getenv("plan_id"),
		EngineID:      engineID,
		APIKey:        os.Getenv("api_key"),
	}
}
