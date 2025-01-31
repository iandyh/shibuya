package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	_ "go.uber.org/automaxprocs"

	cdrclient "github.com/rakutentech/shibuya/shibuya/coordinator/client"
	payload "github.com/rakutentech/shibuya/shibuya/coordinator/payload"
	"github.com/rakutentech/shibuya/shibuya/coordinator/storage"
	"github.com/rakutentech/shibuya/shibuya/scheduler/k8s"

	"github.com/rakutentech/shibuya/shibuya/engines/containerstats"
	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
	"github.com/reqfleet/pubsub/client"
	"github.com/reqfleet/pubsub/messages"

	"github.com/hpcloud/tail"
)

var (
	AGENT_ROOT       = os.Getenv("AGENT_ROOT")
	RESULT_ROOT      = path.Join(AGENT_ROOT, "/test-result")
	TEST_DATA_FOLDER = "/test-data"
	PROPERTY_FILE    = path.Join(AGENT_ROOT, "/test-conf/shibuya.properties")
	JMETER_BIN_FOLER = path.Join(AGENT_ROOT, "/apache-jmeter-3.3/bin")
	JMETER_BIN       = "jmeter"
	STDERR           = "/dev/stderr"
	JMX_FILENAME     = "modified.jmx"
)

var (
	JMETER_EXECUTABLE = path.Join(JMETER_BIN_FOLER, JMETER_BIN)
	JMETER_SHUTDOWN   = path.Join(JMETER_BIN_FOLER, "stoptest.sh")
	JMX_FILEPATH      = path.Join(TEST_DATA_FOLDER, JMX_FILENAME)
)

type ShibuyaWrapper struct {
	newClients     chan chan string
	closingClients chan chan string
	clients        map[chan string]bool
	closeSignal    chan int
	Bus            chan string
	logCounter     int
	httpClient     *http.Client
	wg             sync.WaitGroup
	pidLock        sync.RWMutex
	handlerLock    sync.RWMutex
	currentPid     int
	//stderr         io.ReadCloser
	reader        io.ReadCloser
	writer        io.Writer
	buffer        []byte
	runID         int
	collectionID  string
	planID        string
	engineID      int
	coordinatorIP string
	cdrclient     *cdrclient.Client
	reqOpts       cdrclient.ReqOpts
}

func findCollectionIDPlanID() (string, string) {
	return os.Getenv("collection_id"), os.Getenv("plan_id")
}

func NewServer(coordinatorIP, engineName string) (sw *ShibuyaWrapper) {
	// Instantiate a broker
	sw = &ShibuyaWrapper{
		coordinatorIP:  coordinatorIP,
		newClients:     make(chan chan string),
		closingClients: make(chan chan string),
		clients:        make(map[chan string]bool),
		closeSignal:    make(chan int),
		logCounter:     0,
		Bus:            make(chan string),
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
	sw.cdrclient = cdrclient.NewClient(sw.httpClient)
	engineID, err := k8s.ExtractEngineIDFromName(engineName)
	if err != nil {
		log.Fatal(err)
	}
	sw.engineID = engineID
	sw.collectionID, sw.planID = findCollectionIDPlanID()
	sw.reqOpts = cdrclient.ReqOpts{
		Endpoint: sw.coordinatorIP,
		APIKey:   os.Getenv("api_key"),
	}
	reader, writer, _ := os.Pipe()
	mw := io.MultiWriter(writer, os.Stderr)
	sw.reader = reader
	sw.writer = mw
	log.SetOutput(mw)
	// Set it running - listening and broadcasting events
	go sw.listen()
	go sw.readOutput()
	return
}

func (sw *ShibuyaWrapper) readOutput() {
	rd := bufio.NewReader(sw.reader)
	for {
		line, _, err := rd.ReadLine()
		if err != nil {
			continue
		}
		line = append(line, '\n')
		sw.buffer = append(sw.buffer, line...)
	}
}

func parseRawMetrics(rawLine string) (enginesModel.ShibuyaMetric, error) {
	line := strings.Split(rawLine, "|")
	// We use char "|" as the separator in jmeter jtl file. If some users somehow put another | in their label name
	// we could end up a broken split. For those requests, we simply ignore otherwise the process will crash.
	// With current jmeter setup, we are expecting 12 items to be presented in the JTL file after split.
	// The column in the JTL files are:
	// timeStamp|elapsed|label|responseCode|responseMessage|threadName|success|bytes|grpThreads|allThreads|Latency|Connect
	if len(line) < 12 {
		log.Printf("line length was less than required. Raw line is %s", rawLine)
		return enginesModel.ShibuyaMetric{}, fmt.Errorf("line length was less than required. Raw line is %s", rawLine)
	}
	label := line[2]
	status := line[3]
	threads, _ := strconv.ParseFloat(line[9], 64)
	latency, err := strconv.ParseFloat(line[10], 64)
	if err != nil {
		return enginesModel.ShibuyaMetric{}, err
	}
	return enginesModel.ShibuyaMetric{
		Threads: threads,
		Label:   label,
		Status:  status,
		Latency: latency,
		Raw:     rawLine,
	}, nil
}

func (sw *ShibuyaWrapper) makePromMetrics(line string) {
	metric, err := parseRawMetrics(line)
	// we need to pass the engine meta(project, collection, plan), especially run id
	// Run id is generated at controller side
	if err != nil {
		return
	}
	metric.CollectionID = sw.collectionID
	metric.PlanID = sw.planID
	metric.EngineID = fmt.Sprintf("%d", sw.engineID)
	metric.RunID = fmt.Sprintf("%d", sw.runID)

	metric.ToPrometheus()
}

func (sw *ShibuyaWrapper) listen() {
	for {
		select {
		case s := <-sw.newClients:
			// A new client has connected.
			// Register their message channel
			sw.clients[s] = true
			log.Printf("shibuya-agent: Metric subscriber added. %d registered subscribers", len(sw.clients))
		case s := <-sw.closingClients:
			// A client has dettached and we want to
			// stop sending them messages.
			delete(sw.clients, s)
			close(s)
			log.Printf("shibuya-agent: Metric subscriber removed. %d registered subscribers", len(sw.clients))
		case event := <-sw.Bus:
			// We got a new event from the outside!
			// Send event to all connected clients
			sw.makePromMetrics(event)
			for clientMessageChan, _ := range sw.clients {
				clientMessageChan <- event
			}
		}
	}
}

func (sw *ShibuyaWrapper) makeLogFile() string {
	filename := fmt.Sprintf("kpi-%d.jtl", sw.logCounter)
	return path.Join(RESULT_ROOT, filename)
}

func (sw *ShibuyaWrapper) tailJemeter() {
	var t *tail.Tail
	var err error
	logFile := sw.makeLogFile()
	for {
		t, err = tail.TailFile(logFile, tail.Config{MustExist: true, Follow: true, Poll: true})
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		break
	}
	// It's not thread safe. But we should be ok since we don't perform tests in parallel.
	sw.logCounter += 1
	log.Printf("shibuya-agent: Start tailing JTL file %s", logFile)
	for {
		select {
		case <-sw.closeSignal:
			t.Stop()
			return
		case line := <-t.Lines:
			sw.Bus <- line.Text
		}
	}
}

func (sw *ShibuyaWrapper) streamHandler(w http.ResponseWriter, r *http.Request) {
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
	sw.newClients <- messageChan
	// Listen to connection close and un-register messageChan
	notify := w.(http.CloseNotifier).CloseNotify()

	go func() {
		<-notify
		sw.closingClients <- messageChan
	}()

	for message := range messageChan {
		if message == "" {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", message)
		flusher.Flush()
	}
}

func (sw *ShibuyaWrapper) setPid(pid int) {
	sw.pidLock.Lock()
	defer sw.pidLock.Unlock()

	sw.currentPid = pid
}

func (sw *ShibuyaWrapper) getPid() int {
	sw.pidLock.RLock()
	defer sw.pidLock.RUnlock()

	return sw.currentPid
}

func (sw *ShibuyaWrapper) runCommand() int {
	log.Printf("shibuya-agent: Start to run plan")
	logFile := sw.makeLogFile()
	cmd := exec.Command(JMETER_EXECUTABLE, "-n", "-t", JMX_FILEPATH, "-l", logFile,
		"-q", PROPERTY_FILE, "-G", PROPERTY_FILE, "-j", STDERR)
	cmd.Stderr = sw.writer
	err := cmd.Start()
	if err != nil {
		log.Println(err)
		return 0
	}
	pid := cmd.Process.Pid
	sw.setPid(pid)
	go func() {
		cmd.Wait()
		log.Printf("shibuya-agent: Shutdown is finished, resetting pid to zero")
		sw.setPid(0)
		sw.cdrclient.ReportProgress(sw.reqOpts, sw.collectionID, sw.planID, sw.engineID, false)
	}()
	return pid
}

func cleanTestData() error {
	files, err := os.ReadDir(TEST_DATA_FOLDER)
	if err != nil {
		return err
	}
	for _, file := range files {
		f := path.Join(TEST_DATA_FOLDER, file.Name())
		if err := os.Remove(f); err != nil {
			return err
		}
	}
	return nil
}

func saveToDisk(filename string, file []byte) error {
	filePath := filepath.Join(TEST_DATA_FOLDER, filepath.Base(filename))
	if err := ioutil.WriteFile(filePath, file, 0777); err != nil {
		return err
	}
	return nil
}

func (sw *ShibuyaWrapper) stdoutHandler(w http.ResponseWriter, r *http.Request) {
	w.Write(sw.buffer)
}

func (sw *ShibuyaWrapper) handleStart(planID string, payload *payload.EngineMessage) {
	cleanTestData()
	pf := storage.NewPlanFiles("", sw.collectionID, sw.planID)
	client := sw.cdrclient
	content, err := client.FetchFile(sw.reqOpts, pf.TestFilePath(payload.TestFile))
	if err != nil {
		return
	}
	if err := saveToDisk(JMX_FILEPATH, content); err != nil {
		return
	}
	for dt := range payload.DataFiles {
		content, err := client.FetchFile(sw.reqOpts, pf.EngineDataPath(dt, sw.engineID))
		if err != nil {
			return
		}
		if err := saveToDisk(dt, content); err != nil {
			log.Println(err)
		}
	}
	if pid := sw.runCommand(); pid != 0 {
		if err := client.ReportProgress(sw.reqOpts, sw.collectionID, planID, sw.engineID, true); err != nil {
			log.Println(err)
		}
		go sw.tailJemeter()
	}
}

func (sw *ShibuyaWrapper) handleStop() {
	log.Println("Shutting down jmeter")
	pid := sw.getPid()
	if pid == 0 {
		return
	}
	log.Printf("shibuya-agent: Shutting down Jmeter process %d", sw.getPid())
	cmd := exec.Command(JMETER_SHUTDOWN)
	cmd.Run()
	for {
		if sw.getPid() == 0 {
			break
		}
		time.Sleep(time.Second * 2)
	}
	sw.closeSignal <- 1
}

// This func reports the cpu/memory usage of the engine
// It will run when the engine is started until it's finished.
func (sw *ShibuyaWrapper) reportOwnMetrics(interval time.Duration) error {
	prev := uint64(0)
	engineNumber := strconv.Itoa(sw.engineID)
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
		enginesModel.CpuGauge.WithLabelValues(sw.collectionID,
			sw.planID, engineNumber).Set(float64(used))
		enginesModel.MemGauge.WithLabelValues(sw.collectionID,
			sw.planID, engineNumber).Set(float64(memoryUsage))
	}
}

func main() {
	sw := NewServer(os.Getenv("coordinator_ip"), os.Getenv("engine_name"))
	go func() {
		if err := sw.reportOwnMetrics(5 * time.Second); err != nil {
			// if the engine is having issues with reading stats from cgroup
			// we should fast fail to detect the issue. It could be due to
			// kernel change
			log.Fatal(err)
		}
	}()

	log.Println("Coordinator ip: ", sw.coordinatorIP)
	client := &client.PubSubClient{Addr: fmt.Sprintf("%s:2416", sw.coordinatorIP)}
	var msgChan chan messages.Message
	var err error
	for {
		time.Sleep(2 * time.Second)
		msgChan, _, err = client.Subscribe(fmt.Sprintf("collection:%s", sw.collectionID), &payload.Payload{})
		if err != nil {
			continue
		}
		break
	}
	go func() {
		for msg := range msgChan {
			pl := msg.(*payload.Payload)
			planMsg := pl.PlanMessage[sw.planID]
			switch pl.Verb {
			case "start":
				sw.handleStart(sw.planID, planMsg)
			case "stop":
				_, ok := pl.PlanMessage[sw.planID]
				if !ok {
					continue
				}
				sw.handleStop()
			}
		}
	}()
	http.HandleFunc("/stream", sw.streamHandler)
	http.HandleFunc("/output", sw.stdoutHandler)
	http.HandleFunc("/metrics", promhttp.Handler().ServeHTTP)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
