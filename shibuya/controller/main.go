package controller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rakutentech/shibuya/shibuya/config"
	cdrclient "github.com/rakutentech/shibuya/shibuya/coordinator/client"
	"github.com/rakutentech/shibuya/shibuya/model"
	"github.com/rakutentech/shibuya/shibuya/object_storage"
	"github.com/rakutentech/shibuya/shibuya/scheduler"
	log "github.com/sirupsen/logrus"
)

type Controller struct {
	readingEngineRecords   sync.Map
	ApiNewClients          chan *ApiMetricStream
	ApiClosingClients      chan *ApiMetricStream
	filePath               string
	httpClient             *http.Client
	schedulerKind          string
	Scheduler              scheduler.EngineScheduler
	clientStreamingWorkers int
	sc                     config.ShibuyaConfig
	cdrclient              *cdrclient.Client
	storageClient          object_storage.StorageInterface
}

func NewController(sc config.ShibuyaConfig) *Controller {
	pool := x509.NewCertPool()
	pool.AddCert(sc.CAPair.Cert)
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		},
	}
	c := &Controller{
		filePath:               "/test-data",
		httpClient:             httpClient,
		ApiClosingClients:      make(chan *ApiMetricStream),
		ApiNewClients:          make(chan *ApiMetricStream),
		clientStreamingWorkers: 5,
		sc:                     sc,
		cdrclient:              cdrclient.NewClient(httpClient),
		storageClient:          object_storage.CreateObjStorageClient(sc),
	}

	c.schedulerKind = sc.ExecutorConfig.Cluster.Kind
	c.Scheduler = scheduler.NewEngineScheduler(sc)
	return c
}

type subscribeState struct {
	cancelfunc     context.CancelFunc
	ctx            context.Context
	readingEngines []shibuyaEngine
	readyToClose   chan struct{}
}

type ApiMetricStream struct {
	CollectionID string
	StreamClient chan *ApiMetricStreamEvent
	ClientID     string
}

type ApiMetricStreamEvent struct {
	CollectionID string `json:"collection_id"`
	Raw          string `json:"metrics"`
	PlanID       string `json:"plan_id"`
}

func (c *Controller) StartRunning() {
	go c.streamToApi()
	if !c.sc.DistributedMode {
		log.Info("Controller is running in non-distributed mode!")
		go c.IsolateBackgroundTasks()
	}
}

// In distributed mode, the func will be running as a standalone process
// In non-distributed mode, the func will be run as a goroutine.
func (c *Controller) IsolateBackgroundTasks() {
	go c.AutoPurgeDeployments()
	go c.CheckRunningThenTerminate()
	c.AutoPurgeProjectIngressController()
}

func (c *Controller) handleStreamForClient(item *ApiMetricStream) error {
	log.Printf("New Incoming connection :%s", item.ClientID)
	cid, err := strconv.ParseInt(item.CollectionID, 10, 64)
	if err != nil {
		return err
	}
	collection, err := model.GetCollection(cid)
	if err != nil {
		return err
	}
	readingEngines, err := c.SubscribeCollection(collection)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	ss := subscribeState{
		cancelfunc:     cancel,
		ctx:            ctx,
		readingEngines: readingEngines,
		readyToClose:   make(chan struct{}),
	}
	c.readingEngineRecords.Store(item.ClientID, ss)
	go func(readingEngines []shibuyaEngine) {
		var wg sync.WaitGroup
		for _, engine := range readingEngines {
			wg.Add(1)
			go func(e shibuyaEngine) {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					case metric := <-e.readMetrics():
						if metric != nil {
							item.StreamClient <- &ApiMetricStreamEvent{
								CollectionID: metric.collectionID,
								PlanID:       metric.planID,
								Raw:          metric.raw,
							}
						}
					}
				}
			}(engine)
		}
		wg.Wait()
		ss.readyToClose <- struct{}{}
	}(readingEngines)
	return nil
}

func (c *Controller) streamToApi() {
	workerQueue := make(chan *ApiMetricStream)
	for i := 0; i < c.clientStreamingWorkers; i++ {
		go func() {
			for item := range workerQueue {
				if err := c.handleStreamForClient(item); err != nil {
					log.Error(err)
				}
			}
		}()
	}
	for {
		select {
		case item := <-c.ApiNewClients:
			workerQueue <- item
		case item := <-c.ApiClosingClients:
			clientID := item.ClientID
			collectionID := item.CollectionID
			if t, ok := c.readingEngineRecords.Load(clientID); ok {
				ss := t.(subscribeState)
				for _, e := range ss.readingEngines {
					go func(e shibuyaEngine) {
						e.closeStream()
					}(e)
				}
				ss.cancelfunc()
				<-ss.readyToClose
				close(item.StreamClient)
				c.readingEngineRecords.Delete(clientID)
				log.Printf("Client %s disconnect from the API for collection %s.", clientID, collectionID)
			}
		}
	}
}
