package main

import (
	"log"

	_ "go.uber.org/automaxprocs"

	"github.com/rakutentech/shibuya/shibuya/engines/agentserver"
	"github.com/rakutentech/shibuya/shibuya/engines/jmeter/metrics"
)

var (
	JMETER_BIN_FOLER  = "/apache-jmeter-3.3/bin"
	JMETER_BIN        = "jmeter"
	JMX_FILENAME      = "modified.jmx"
	RESULT_FILE_NAME  = "kpi.jtl"
	agentDir          = agentserver.NewAgentDirHandler("")
	PROPERTY_FILE     = agentDir.ConfFilesDir().Filepath("shibuya.properties")
	JMETER_EXECUTABLE = agentDir.Dir().Filepath(JMETER_BIN_FOLER, JMETER_BIN)
	JMETER_SHUTDOWN   = agentDir.Dir().Filepath(JMETER_BIN_FOLER, "stoptest.sh")
	JMX_FILEPATH      = agentDir.TestFilesDir().Filepath(JMX_FILENAME)
	RESULT_FILE       = agentDir.ResultFilesDir().ResultFile(RESULT_FILE_NAME)
)

func main() {
	engineMeta := agentserver.FetchEngineMeta()
	startCommand := agentserver.Command{
		Command: JMETER_EXECUTABLE,
		Args: []string{"-n", "-t", JMX_FILEPATH, "-l", RESULT_FILE, "-q",
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
		StartCommand: startCommand,
		ResultFile:   RESULT_FILE,
	}
	as := agentserver.MakeAgentServer(options)
	if err := as.Run(); err != nil {
		log.Fatal(err)
	}
}
