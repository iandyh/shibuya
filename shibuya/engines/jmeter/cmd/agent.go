package main

import (
	"fmt"
	"log"

	_ "go.uber.org/automaxprocs"

	"github.com/rakutentech/shibuya/shibuya/engines/agentserver"
	"github.com/rakutentech/shibuya/shibuya/engines/jmeter/metrics"
)

var (
	JMETER_BIN_FOLER  = "/apache-jmeter-3.3/bin"
	JMETER_BIN        = "jmeter"
	JMX_FILENAME      = "modified.jmx"
	agentDir          = agentserver.NewAgentDirHandler("")
	PROPERTY_FILE     = agentDir.ConfFilesDir().Filepath("shibuya.properties")
	JMETER_EXECUTABLE = agentDir.Dir().Filepath(JMETER_BIN_FOLER, JMETER_BIN)
	JMETER_SHUTDOWN   = agentDir.Dir().Filepath(JMETER_BIN_FOLER, "stoptest.sh")
	JMX_FILEPATH      = agentDir.TestFilesDir().Filepath(JMX_FILENAME)
)

func main() {
	engineMeta := agentserver.FetchEngineMeta()
	startCommand := agentserver.Command{
		Command: JMETER_EXECUTABLE,
		Args: []string{"-n", "-t", JMX_FILEPATH, "-q",
			PROPERTY_FILE, "-G", PROPERTY_FILE, "-j", agentserver.STDERR},
	}
	stopCommand := &agentserver.Command{
		Command: JMETER_SHUTDOWN,
	}
	options := agentserver.AgentServerOptions{
		TestFileName: JMX_FILENAME,
		EngineMeta:   engineMeta,
		MetricParser: metrics.ParseRawMetrics,
		StopCommand:  stopCommand,
		ExtraArgs:    []string{"-l"},
		StartCommand: startCommand,
		ResultFileFunc: func(fileID int) string {
			return fmt.Sprintf("kpi-%d.jtl", fileID)
		},
	}
	_, err := agentserver.StartAgentServer(options)
	if err != nil {
		log.Fatal(err)
	}
}
