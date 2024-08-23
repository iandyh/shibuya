package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	_ "go.uber.org/automaxprocs"

	"github.com/rakutentech/shibuya/shibuya/engines/agentserver"
	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
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

func findResultFile(fileID int) string {
	return fmt.Sprintf("kpi-%d.jtl", fileID)
}

func main() {
	engineMeta := agentserver.FetchEngineMeta()
	startCommand := agentserver.Command{
		Command: JMETER_EXECUTABLE,
		Args: []string{"-n", "-t", JMX_FILEPATH, "-q",
			PROPERTY_FILE, "-G", PROPERTY_FILE, "-j", STDERR},
	}
	stopCommand := &agentserver.Command{
		Command: JMETER_SHUTDOWN,
	}
	options := agentserver.AgentServerOptions{
		TestFileName:   JMX_FILENAME,
		EngineMeta:     engineMeta,
		MetricParser:   parseRawMetrics,
		StopCommand:    stopCommand,
		ExtraArgs:      []string{"-l"},
		RunCommand:     startCommand,
		ResultFileFunc: findResultFile,
	}
	_, err := agentserver.StartAgentServer(options)
	if err != nil {
		log.Fatal(err)
	}
}
