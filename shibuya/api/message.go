package api

import "net/http"

type JSONMessage struct {
	Message string `json:"message"`
}

func makeRespMessage(message string) *JSONMessage {
	return &JSONMessage{
		Message: message,
	}
}

func makeFailMessage(w http.ResponseWriter, message string, statusCode int) {
	messageObj := makeRespMessage(message)
	renderJSON(w, statusCode, messageObj)
}
