package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/scheduler"
	smodel "github.com/rakutentech/shibuya/shibuya/scheduler/model"

	es "github.com/iandyh/eventsource"
	log "github.com/sirupsen/logrus"
)

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

type Engine struct {
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

func (e *Engine) makeBaseUrl() string {
	base := "%s/%s"
	if strings.Contains(e.engineUrl, "http") {
		return base
	}
	return "https://" + base
}

func (e *Engine) subscribe(runID int64, apiKey string) error {
	base := e.makeBaseUrl()
	streamUrl := fmt.Sprintf(base, e.engineUrl, "stream")
	req, err := http.NewRequest("GET", streamUrl, nil)
	if err != nil {
		return err
	}
	log.Printf("Subscribing to engine url %s", streamUrl)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	httpClient := &http.Client{
		Transport: e.httpClient.Transport,
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	stream, err := es.SubscribeWith("", httpClient, req)
	if err != nil {
		cancel()
		return err
	}
	e.stream = stream
	e.cancel = cancel
	e.runID = runID
	return nil
}

func (e *Engine) closeStream() {
	e.cancel()
	e.stream.Close()
}

func (e *Engine) readMetrics() chan *shibuyaMetric {
	ch := make(chan *shibuyaMetric)
	go func() {
	outer:
		for {
			select {
			case ev, ok := <-e.stream.Events:
				if !ok {
					break outer
				}
				raw := ev.Data()
				ch <- &shibuyaMetric{
					raw:          raw,
					collectionID: strconv.FormatInt(e.collectionID, 10),
					planID:       strconv.FormatInt(e.planID, 10),
					engineID:     strconv.FormatInt(int64(e.ID), 10),
					runID:        strconv.FormatInt(e.runID, 10),
				}
			case _, ok := <-e.stream.Errors:
				if !ok {
					break outer
				}
			}
		}
		close(ch)
	}()
	return ch
}

func (e *Engine) updateEngineUrl(url string) {
	e.engineUrl = url
}

func generateEngines(enginesRequired int, planID, collectionID, projectID int64, httpClient *http.Client) (engines []*Engine, err error) {
	for i := 0; i < enginesRequired; i++ {
		engineC := &Engine{
			ID:           i,
			projectID:    projectID,
			collectionID: collectionID,
			planID:       planID,
			httpClient:   httpClient,
		}
		engines = append(engines, engineC)
	}
	return engines, nil
}

func generateEnginesWithUrl(enginesRequired int, planID, collectionID, projectID int64, scheduler scheduler.EngineScheduler,
	httpClient *http.Client) (engines []*Engine, err error) {
	engines, err = generateEngines(enginesRequired, planID, collectionID, projectID, httpClient)
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
