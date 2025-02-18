package agentserver

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpauth "github.com/rakutentech/shibuya/shibuya/http/auth"
	httproute "github.com/rakutentech/shibuya/shibuya/http/route"
)

func (as *AgentServer) HTTPRouter() *httproute.Router {
	router := httproute.NewRouter("agent http endpoints", "")
	router.AddRoutes(httproute.Routes{
		{
			Path:        "/progress",
			Method:      "GET",
			HandlerFunc: as.handleProcessCheck,
		},
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
	return router
}

func (as *AgentServer) startHTTPServer() error {
	router := as.HTTPRouter()
	handlers := http.Handler(router.Mux())
	// Running in http mode should be ok because engines are never directly exposed to public network
	return http.ListenAndServe(":8080",
		httpauth.AuthRequiredWithToken(handlers, as.options.EngineMeta.APIKey))
}

func (as *AgentServer) handleProcessCheck(w http.ResponseWriter, _ *http.Request) {
	if as.getProcess() != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusNotFound)
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
