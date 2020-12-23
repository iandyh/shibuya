package model

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	es "github.com/iandyh/eventsource"
)

// var engineHttpClient = &http.Client{
// 	Timeout: 30 * time.Second,
// }

var engineHttpClient = &http.Client{
	Timeout: 30 * time.Second,
}

type Engine struct {
	CollectionID int64
	PlanID       int64
	ProjectID    int64
	ID           int
	EngineUrl    string
	RunID        int64
	Stream       *es.Stream
	Cancel       context.CancelFunc
}

// type workerEngine interface {
// 	Subscribe() error
// }
func NewEngine(projectID, collectionID, planID, runID int64, engineUrl string) *Engine {
	return &Engine{
		ProjectID:    projectID,
		CollectionID: collectionID,
		PlanID:       planID,
		RunID:        runID,
		EngineUrl:    engineUrl,
	}
}

func (e *Engine) Subscribe() error {
	streamUrl := fmt.Sprintf("http://%s/%s", e.EngineUrl, "stream")
	req, err := http.NewRequest("GET", streamUrl, nil)
	if err != nil {
		return err
	}
	log.Printf("Subscribing to engine url %s", streamUrl)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	httpClient := &http.Client{}
	stream, err := es.SubscribeWith("", httpClient, req)
	if err != nil {
		cancel()
		return err
	}
	e.Stream = stream
	e.Cancel = cancel
	return nil
}

func (e *Engine) ReadMetrics() {
	go func() {
		defer func() {
			e.Stream.Close()
			e.Cancel()
		}()
	outer:
		for {
			select {
			case ev, ok := <-e.Stream.Events:
				if !ok {
					break outer
				}
				raw := ev.Data()
				line := strings.Split(raw, "|")

				//label := line[2]
				//status := line[3]
				//threads, _ := strconv.ParseFloat(line[9], 64)
				_, err := strconv.ParseFloat(line[10], 64)
				if err != nil {
					continue outer // no csv headers
				}
				// ch <- &shibuyaMetric{
				// 	threads:      threads,
				// 	label:        label,
				// 	status:       status,
				// 	latency:      latency,
				// 	raw:          raw,
				// 	collectionID: strconv.FormatInt(e.collectionID, 10),
				// 	planID:       strconv.FormatInt(e.planID, 10),
				// 	engineID:     strconv.FormatInt(int64(e.ID), 10),
				// 	runID:        strconv.FormatInt(e.runID, 10),
				// }
			case <-e.Stream.Errors:
				break outer
			}
		}
	}()
}

func (e *Engine) Terminate() error {
	stopUrl := fmt.Sprintf("http://%s/stop", e.EngineUrl)
	resp, err := engineHttpClient.Post(stopUrl, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
