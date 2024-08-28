package main

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"

	"github.com/rakutentech/shibuya/shibuya/engines/agentserver"
)

const (
	locustConfig = "locust.conf"
)

var tmpl = `
headless = true
users = {{ .Concurrency }}
run-time = {{ .Duration }}
`

func writeConfig(edc *agentserver.EngineDataConfig) error {
	file, err := os.Create(locustConfig)
	if err != nil {
		return err
	}
	edc.Duration = fmt.Sprintf("%sm", edc.Duration)
	t, err := template.New("locust").Parse(tmpl)
	if err != nil {
		return err
	}
	return t.Execute(file, edc)
}

func preStartPreparation(edc *agentserver.EngineDataConfig) error {
	if err := writeConfig(edc); err != nil {
		return err
	}
	return nil
}

// result.csv a dummy flag for instructing locust to dump results into csv
// Actual output will be controlled by the .py file
var startCommand = *exec.Command("locust", "--csv", "result.csv")

func findResultFile(fileID int) string {
	return "1.csv"
}

func main() {
	aso := agentserver.AgentServerOptions{
		PrePrestartPreparation: preStartPreparation,
		StartCommand:           &startCommand,
		StopCommand:            nil,
		ResultFileFunc:         findResultFile,
	}
	c := make(chan struct{})
	agentserver.StartAgentServer(aso)
	c <- struct{}{}
}
