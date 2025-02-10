package locust

import (
	"html/template"
	"os"

	"github.com/rakutentech/shibuya/shibuya/coordinator/storage"
	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
)

var (
	tmpl = `
headless = true
users = {{ .Concurrency }}
run-time = {{ .Duration }}m
spawn-rate = {{ .Rampup }}
`

	JmeterListener = `
from locust_plugins.listeners import jmeter
@events.init.add_listener
def on_locust_init(environment, **kwargs):
    jmeter.JmeterListener(env=environment, testplan="{{ .PlanName }}",
                          flush_size=1, results_filename="/shibuya-agent/test-result/result.csv")
`
)

func writeConfig(filepath string, pec enginesModel.PlanEnginesConfig) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	t, err := template.New("locust").Parse(tmpl)
	if err != nil {
		return err
	}
	return t.Execute(file, pec)
}

// We append the jmeter listener to the end of test file
// This has two implications: 1. Locust will need to have locust-plugins. 2.
// The result.csv needs to be in the same path as in the cmd/agent.go
// Otherwise, the agent won't be able to find the test results.
func appendJmeterListener(filepath string, planName string) error {
	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	t, err := template.New("listener").Parse(JmeterListener)
	if err != nil {
		return err
	}
	data := map[string]string{
		"PlanName": planName,
	}
	return t.Execute(file, data)
}

func MakeTestPlan(pf *storage.PlanFiles, planName, filename string, fileBytes []byte, pec enginesModel.PlanEnginesConfig) error {
	if err := pf.StoreTestPlan(filename, fileBytes); err != nil {
		return err
	}
	if err := appendJmeterListener(pf.TestFilePath(filename), planName); err != nil {
		return err
	}
	if err := writeConfig(pf.TestFilePath("locust.conf"), pec); err != nil {
		return err
	}
	return nil
}
