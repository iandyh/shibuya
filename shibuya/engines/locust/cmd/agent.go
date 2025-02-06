package main

import (
	"log"

	"github.com/rakutentech/shibuya/shibuya/engines/agentserver"
	"github.com/rakutentech/shibuya/shibuya/engines/locust/metrics"
)

const (
	TEST_FILE_NAME   = "locustfile.py"
	CONF_FILE_NAME   = "locust.conf"
	RESULT_FILE_NAME = "result.csv"
)

var (
	agentDir    = agentserver.NewAgentDirHandler("")
	CONF_FILE   = agentDir.ConfFilesDir().Filepath(CONF_FILE_NAME)
	TEST_FILE   = agentDir.TestFilesDir().Filepath(TEST_FILE_NAME)
	RESULT_FILE = agentDir.ResultFilesDir().ResultFile(RESULT_FILE_NAME)
)

func main() {
	engineMeta := agentserver.FetchEngineMeta()
	// result.csv in this command is just a placehoder. The actual results are stored in the
	// RESULT_FIFE defined above
	startCommand := agentserver.Command{
		Command: "locust",
		Args:    []string{"-f", TEST_FILE, "--csv", "result.csv", "--config", CONF_FILE},
	}
	options := agentserver.AgentServerOptions{
		TestFileName: TEST_FILE_NAME,
		StartCommand: startCommand,
		EngineMeta:   engineMeta,
		MetricParser: metrics.ParseRawMetrics,
		ConfFileName: CONF_FILE_NAME,
		ResultFile:   RESULT_FILE,
	}
	as := agentserver.MakeAgentServer(options)
	if err := as.Run(); err != nil {
		log.Fatal(err)
	}
}
