package metrics_test

import (
	"testing"

	"github.com/rakutentech/shibuya/shibuya/engines/locust/metrics"
	"github.com/stretchr/testify/assert"
)

func TestMetricParsing(t *testing.T) {
	line := "2025-02-08 11:06:24,1543,/asdf,0,KO,locust,unknown,false,404 Client Error: Not Found for url: /asdf,1552,0,1,1,0,0,0"
	metric, err := metrics.ParseRawMetrics(line)
	assert.Nil(t, err)
	assert.Equal(t, "404", metric.Status)
	assert.Equal(t, float64(1), metric.Threads)
	assert.Equal(t, "/asdf", metric.Label)
	assert.Equal(t, float64(1543), metric.Latency)
}
