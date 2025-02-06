package metrics

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
)

// JmeterListener could not produce correct status. For non-2xx requests, it will produce
// errors such as 404 Client Error, so we can simply return the first 3 chars as status code
// For 200 requests, the error message will be None.
func findStatus(failureMessage string) string {
	if failureMessage == "None" {
		return "200"
	}
	return failureMessage[:3]
}

func ParseRawMetrics(rawLine string) (enginesModel.ShibuyaMetric, error) {
	line := strings.Split(rawLine, ",")
	// The columns in the result.csv are:
	//timeStamp,elapsed,label,responseCode,responseMessage,threadName,dataType,success,failureMessage,bytes,sentBytes,grpThreads,allThreads,Latency,IdleTime,Connect
	if len(line) != 16 {
		return enginesModel.ShibuyaMetric{}, fmt.Errorf("line length was less than required. Raw line is %s", rawLine)
	}
	label := line[2]
	threads, err := strconv.ParseFloat(line[12], 64)
	if err != nil {
		return enginesModel.ShibuyaMetric{}, errors.New("Invalid thread number")
	}
	latency, err := strconv.ParseFloat(line[1], 64)
	if err != nil {
		return enginesModel.ShibuyaMetric{}, errors.New("Invalid latency value")
	}
	status := findStatus(line[8])
	return enginesModel.ShibuyaMetric{
		Threads: threads,
		Label:   label,
		Status:  status,
		Latency: latency,
		Raw:     rawLine,
	}, nil
}
