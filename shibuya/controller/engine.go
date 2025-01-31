package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/scheduler"
	smodel "github.com/rakutentech/shibuya/shibuya/scheduler/model"

	es "github.com/iandyh/eventsource"
	log "github.com/sirupsen/logrus"
)

type shibuyaEngine interface {
	subscribe(runID int64, apiKey string) error
	readMetrics() chan *shibuyaMetric
	closeStream()
	updateEngineUrl(url string)
}

type engineType struct{}

var JmeterEngineType engineType

type shibuyaMetric struct {
	threads      float64
	latency      float64
	label        string
	status       string
	raw          string
	collectionID string
	planID       string
	engineID     string
	runID        string
}

const enginePlanRoot = "/test-data"

type baseEngine struct {
	name         string
	serviceName  string
	ingressName  string
	engineUrl    string
	ingressClass string
	collectionID int64
	planID       int64
	projectID    int64
	ID           int
	stream       *es.Stream
	cancel       context.CancelFunc
	runID        int64
	httpClient   *http.Client
	*config.ExecutorContainer
}

func (be *baseEngine) makeBaseUrl() string {
	base := "%s/%s"
	if strings.Contains(be.engineUrl, "http") {
		return base
	}
	return "https://" + base
}

func (be *baseEngine) subscribe(runID int64, apiKey string) error {
	base := be.makeBaseUrl()
	streamUrl := fmt.Sprintf(base, be.engineUrl, "stream")
	req, err := http.NewRequest("GET", streamUrl, nil)
	if err != nil {
		return err
	}
	log.Printf("Subscribing to engine url %s", streamUrl)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	httpClient := &http.Client{
		Transport: be.httpClient.Transport,
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bear %s", apiKey))
	stream, err := es.SubscribeWith("", httpClient, req)
	if err != nil {
		cancel()
		return err
	}
	be.stream = stream
	be.cancel = cancel
	be.runID = runID
	return nil
}

func (be *baseEngine) closeStream() {
	be.cancel()
	be.stream.Close()
}

func (be *baseEngine) readMetrics() chan *shibuyaMetric {
	log.Println("BaseEngine does not readMetrics(). Use an engine type.")
	return nil
}

func (be *baseEngine) updateEngineUrl(url string) {
	be.engineUrl = url
}

func findEngineConfig(et engineType, sc config.ShibuyaConfig) *config.ExecutorContainer {
	switch et {
	case JmeterEngineType:
		return sc.ExecutorConfig.JmeterContainer.ExecutorContainer
	}
	return nil
}

func generateEngines(enginesRequired int, planID, collectionID, projectID int64,
	et engineType, httpClient *http.Client) (engines []shibuyaEngine, err error) {
	for i := 0; i < enginesRequired; i++ {
		engineC := &baseEngine{
			ID:           i,
			projectID:    projectID,
			collectionID: collectionID,
			planID:       planID,
			httpClient:   httpClient,
		}
		var e shibuyaEngine
		switch et {
		case JmeterEngineType:
			e = NewJmeterEngine(engineC)
		default:
			return nil, makeWrongEngineTypeError()
		}
		engines = append(engines, e)
	}
	return engines, nil
}

func generateEnginesWithUrl(enginesRequired int, planID, collectionID, projectID int64, et engineType, scheduler scheduler.EngineScheduler,
	httpClient *http.Client) (engines []shibuyaEngine, err error) {
	engines, err = generateEngines(enginesRequired, planID, collectionID, projectID, et, httpClient)
	if err != nil {
		return nil, err
	}
	engineUrls, err := scheduler.FetchEngineUrlsByPlan(collectionID, planID, &smodel.EngineOwnerRef{
		ProjectID:    projectID,
		EnginesCount: len(engines),
	})
	// This could happen during purging as there are still some engines lingering in the scheduler
	if len(engineUrls) != len(engines) {
		return nil, errors.New("Engines in scheduler does not match")
	}
	for i, e := range engines {
		url := engineUrls[i]
		e.updateEngineUrl(url)
	}
	return engines, nil
}
