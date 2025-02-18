package agentserver

import "net/http"

func (as *AgentServer) handleProcessCheck(w http.ResponseWriter, _ *http.Request) {
	if as.getProcess() != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}
