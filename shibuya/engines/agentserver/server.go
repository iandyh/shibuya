package agentserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/hpcloud/tail"
)

type ShibuyaAgentHandler http.HandlerFunc

const (
	RESULT_ROOT      = "/test-result"
	TEST_DATA_FOLDER = "/test-data"
	STDERR           = "/dev/stderr"
)

type AgentServer struct {
	incomingClients chan chan string
	closingClients  chan chan string
	clients         map[chan string]struct{}
	bus             chan string
	fileId          int
	mux             *http.ServeMux
	// A queue that ensure only one test is being managed the agent
	// All subsequent requests will be blocked until it's finished
	executionQueue chan *exec.Cmd
	process        *os.Process
	ctx            context.Context
	cancel         context.CancelFunc
}

type PathToHandler struct {
	Path    string
	Handler ShibuyaAgentHandler
}

type AgentHandlers []PathToHandler

type EngineDataConfig struct {
	Duration    string `json:"duration"`
	Concurrency string `json:"concurrency"`
	Rampup      string `json:"rampup"`
}

func NewAgentServer() *AgentServer {
	as := &AgentServer{
		incomingClients: make(chan chan string),
		closingClients:  make(chan chan string),
		clients:         make(map[chan string]struct{}),
		executionQueue:  make(chan *exec.Cmd),
		bus:             make(chan string),
	}
	go as.listenForSubscribers()
	go as.run()
	return as
}

func (as *AgentServer) run() {
	for {
		select {
		case command := <-as.executionQueue:
			// command will wait for the shutdown signal. Once it's done, the command
			// func should finish
			if err := command.Start(); err != nil {
				log.Println(err)
			} else {
				as.ctx, as.cancel = context.WithCancel(context.Background())
				as.process = command.Process
				// Increase the fileid for next run
				as.fileId += 1
				if err := command.Wait(); err == nil {
					log.Printf("shibuya-agent: Command is finished")
				}
			}
		}
	}
}

func (as *AgentServer) setHandlers(handlers AgentHandlers) {
	as.mux = http.NewServeMux()
	for _, ah := range handlers {
		as.mux.HandleFunc(ah.Path, ah.Handler)
	}
}

func (as *AgentServer) listenForSubscribers() {
	for {
		select {
		case s := <-as.incomingClients:
			// A new client has connected.
			// Register their message channel
			as.clients[s] = struct{}{}
			log.Printf("shibuya-agent: Metric subscriber added. %d registered subscribers", len(as.clients))
		case s := <-as.closingClients:
			// A client has dettached and we want to
			// stop sending them messages.
			delete(as.clients, s)
			close(s)
			log.Printf("shibuya-agent: Metric subscriber removed. %d registered subscribers", len(as.clients))
		case event := <-as.bus:
			// We got a new event from the outside!
			// Send event to all connected clients
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
	log.Printf("shibuya-agent: Start tailing result file %s", filepath)
	for {
		select {
		case <-as.ctx.Done():
			t.Stop()
			return
		case line := <-t.Lines:
			as.bus <- line.Text
		}
	}
}

func (as *AgentServer) makeFullResultPath(filename string) string {
	return path.Join(RESULT_ROOT, filename)
}

func (as *AgentServer) StartHandler(
	preStartPreparation func(edc *EngineDataConfig) error,
	command *exec.Cmd,
	resultFileFunc func(fileID int) string) ShibuyaAgentHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		// maybe we should return 200 here.
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if preStartPreparation != nil {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer r.Body.Close()
			edc := &EngineDataConfig{}
			if err := json.Unmarshal(body, edc); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := preStartPreparation(edc); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		filename := resultFileFunc(as.fileId)
		// move this function to the worker
		go as.tailFunc(as.makeFullResultPath(filename))
		as.executionQueue <- command
	}
}

func (as *AgentServer) StopHandler(command *exec.Cmd) ShibuyaAgentHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if as.process == nil {
			return
		}
		// what happen if cancel is the previous ctx?
		as.cancel()
		log.Printf("shibuya-agent: Shutting down process %d", as.process.Pid)
		if command == nil {
			if err := as.process.Kill(); err != nil {
				log.Println(err)
			}
			return
		}
		if err := command.Run(); err != nil {
			log.Println(err)
			return
		}
		for {
			if as.process == nil {
				break
			}
			time.Sleep(time.Second * 2)
		}
	}
}

func (as *AgentServer) ProgressHandler(w http.ResponseWriter, r *http.Request) {
	if as.process == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
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

type AgentServerOptions struct {
	PrePrestartPreparation func(edf *EngineDataConfig) error
	StartCommand           *exec.Cmd
	StopCommand            *exec.Cmd
	ResultFileFunc         func(fileID int) string
}

func StartAgentServer(options AgentServerOptions) (*AgentServer, error) {
	as := NewAgentServer()
	handlers := AgentHandlers{
		PathToHandler{
			Path: "/start",
			Handler: as.StartHandler(
				options.PrePrestartPreparation,
				options.StartCommand,
				options.ResultFileFunc),
		},
		PathToHandler{
			Path:    "/stop",
			Handler: as.StopHandler(options.StopCommand),
		},
		PathToHandler{
			Path:    "/progress",
			Handler: as.ProgressHandler,
		},
		PathToHandler{
			Path:    "/stream",
			Handler: as.StreamHandler,
		},
	}
	as.setHandlers(handlers)
	go func() {
		http.ListenAndServe(":8080", as.mux)
	}()
	return as, nil
}

func findResultFile(fileID int) string {
	return fmt.Sprintf("kpi-%d.jtl", fileID)
}

func startAgent(startCommand, stopCommand *exec.Cmd) (*AgentServer, error) {
	options := AgentServerOptions{
		StartCommand:   startCommand,
		StopCommand:    stopCommand,
		ResultFileFunc: findResultFile,
	}
	return StartAgentServer(options)
}
