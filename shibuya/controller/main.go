package controller

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/model"
	"github.com/rakutentech/shibuya/shibuya/scheduler"
	smodel "github.com/rakutentech/shibuya/shibuya/scheduler/model"
	"github.com/rakutentech/shibuya/shibuya/utils"
	log "github.com/sirupsen/logrus"
)

type Controller struct {
	ApiNewClients      chan *ApiMetricStream
	ApiStreamClients   map[string]map[string]chan *ApiMetricStreamEvent
	ApiMetricStreamBus chan *ApiMetricStreamEvent
	ApiClosingClients  chan *ApiMetricStream
	filePath           string
	httpClient         *http.Client
	schedulerKind      string
	Scheduler          scheduler.EngineScheduler
}

func NewController() *Controller {
	c := &Controller{
		filePath: "/test-data",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		ApiMetricStreamBus: make(chan *ApiMetricStreamEvent),
		ApiClosingClients:  make(chan *ApiMetricStream),
		ApiNewClients:      make(chan *ApiMetricStream),
		ApiStreamClients:   make(map[string]map[string]chan *ApiMetricStreamEvent),
	}
	c.schedulerKind = config.SC.ExecutorConfig.Cluster.Kind
	c.Scheduler = scheduler.NewEngineScheduler(config.SC.ExecutorConfig.Cluster)
	return c
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
	go c.fetchEngineMetrics()
	if !config.SC.DistributedMode {
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

func (c *Controller) streamToApi() {
	for {
		select {
		case item := <-c.ApiNewClients:
			collectionID := item.CollectionID
			clientID := item.ClientID
			if m, ok := c.ApiStreamClients[collectionID]; !ok {
				m = make(map[string]chan *ApiMetricStreamEvent)
				m[clientID] = item.StreamClient
				c.ApiStreamClients[collectionID] = m
			} else {
				m[clientID] = item.StreamClient
			}
			log.Printf("A client %s connects to collection %s, start streaming", clientID, collectionID)
		case item := <-c.ApiClosingClients:
			collectionID := item.CollectionID
			clientID := item.ClientID
			m := c.ApiStreamClients[collectionID]
			close(item.StreamClient)
			delete(m, clientID)
			log.Printf("Client %s disconnect from the API for collection %s.", clientID, collectionID)
		case event := <-c.ApiMetricStreamBus:
			streamClients, ok := c.ApiStreamClients[event.CollectionID]
			if !ok {
				continue
			}
			for _, streamClient := range streamClients {
				streamClient <- event
			}
		}
	}
}

// This is used for tracking all the running plans
// So even when Shibuya controller restarts, the tests can resume
type RunningPlan struct {
	ep         *model.ExecutionPlan
	collection *model.Collection
}

func (c *Controller) calNodesRequired(enginesNum int) int64 {
	masterCPU, _ := strconv.ParseFloat(config.SC.ExecutorConfig.JmeterContainer.CPU, 64)
	enginePerNode := math.Floor(float64(config.SC.ExecutorConfig.Cluster.NodeCPUSpec) / masterCPU)
	nodesRequired := math.Ceil(float64(enginesNum) / enginePerNode)
	return int64(nodesRequired)
}

func (c *Controller) DeployCollection(collection *model.Collection) error {
	eps, err := collection.GetExecutionPlans()
	if err != nil {
		return err
	}
	nodesCount := int64(0)
	enginesCount := 0
	vu := 0
	for _, e := range eps {
		enginesCount += e.Engines
		vu += e.Engines * e.Concurrency
	}
	if config.SC.ExecutorConfig.Cluster.OnDemand {
		nodesCount = c.calNodesRequired(enginesCount)
		operator := NewGCPOperator(collection.ID, nodesCount)
		err := operator.prepareNodes()
		if err != nil {
			return err
		}
	}
	sid := ""
	if project, err := model.GetProject(collection.ProjectID); err == nil {
		sid = project.SID
	}
	if err := collection.NewLaunchEntry(sid, config.SC.Context, int64(enginesCount), nodesCount, int64(vu)); err != nil {
		return err
	}
	err = utils.Retry(func() error {
		return c.Scheduler.ExposeProject(collection.ProjectID)
	}, nil)
	if err != nil {
		return err
	}
	if err = c.Scheduler.CreateCollectionScraper(collection.ID); err != nil {
		return err
	}
	// we will assume collection deployment will always be successful
	// For some large deployments, it might take more than 1 min to finish, which could result 504 at gateway side
	// So we do not wait for the deployment to be finished.
	go func() {
		var wg sync.WaitGroup
		now_ := time.Now()
		for _, e := range eps {
			wg.Add(1)
			go func(ep *model.ExecutionPlan) {
				defer wg.Done()
				pc := NewPlanController(ep, collection, c.Scheduler)
				utils.Retry(func() error {
					return pc.deploy()
				}, nil)
			}(e)
		}
		wg.Wait()
		duration := time.Now().Sub(now_)
		log.Infof("All engines deployment are finished for collection %d, total duration: %.2f seconds",
			collection.ID, duration.Seconds())
	}()
	return nil
}

func (c *Controller) CollectionStatus(collection *model.Collection) (*smodel.CollectionStatus, error) {
	eps, err := collection.GetExecutionPlans()
	if err != nil {
		return nil, err
	}
	cs, err := c.Scheduler.CollectionStatus(collection.ProjectID, collection.ID, eps)
	if err != nil {
		return nil, err
	}
	if config.SC.ExecutorConfig.Cluster.OnDemand {
		operator := NewGCPOperator(collection.ID, 0)
		info := operator.GCPNodesInfo()
		cs.PoolStatus = "LAUNCHED"
		if info != nil {
			cs.PoolSize = info.Size
			cs.PoolStatus = info.Status
		}
	}
	if config.SC.DevMode {
		cs.PoolSize = 100
		cs.PoolStatus = "running"
	}
	return cs, nil
}

func (c *Controller) PurgeNodes(collection *model.Collection) error {
	if config.SC.ExecutorConfig.Cluster.OnDemand {
		operator := NewGCPOperator(collection.ID, int64(0))
		if err := operator.destroyNodes(); err != nil {
			return err
		}
		// we don't bill for on-demand cluster as for now.
		//collection.MarkUsageFinished()
		return nil
	}
	return nil
}
