package metrics

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
)

func ParseRawMetrics(rawLine string) (enginesModel.ShibuyaMetric, error) {
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
