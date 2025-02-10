package controller

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	cdrclient "github.com/rakutentech/shibuya/shibuya/coordinator/client"
	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
	"github.com/rakutentech/shibuya/shibuya/model"
	smodel "github.com/rakutentech/shibuya/shibuya/scheduler/model"
	"github.com/rakutentech/shibuya/shibuya/utils"
	log "github.com/sirupsen/logrus"
)

func prepareCollection(collection *model.Collection) []*enginesModel.EngineDataConfig {
	planCount := len(collection.ExecutionPlans)
	edc := enginesModel.EngineDataConfig{
		EngineData: map[string]*model.ShibuyaFile{},
	}
	engineDataConfigs := edc.DeepCopies(planCount)
	for i := 0; i < planCount; i++ {
		for _, d := range collection.Data {
			sf := model.ShibuyaFile{
				Filename:     d.Filename,
				Filepath:     d.Filepath,
				TotalSplits:  1,
				CurrentSplit: 0,
			}
			if collection.CSVSplit {
				sf.TotalSplits = planCount
				sf.CurrentSplit = i
			}
			engineDataConfigs[i].EngineData[sf.Filename] = &sf
		}
	}
	return engineDataConfigs
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
	sid := ""
	if project, err := model.GetProject(collection.ProjectID); err == nil {
		sid = project.SID
	}
	if err := collection.NewLaunchEntry(sid, c.sc.Context, int64(enginesCount), nodesCount, int64(vu)); err != nil {
		return err
	}
	service, err := c.Scheduler.ExposeProject(collection.ProjectID)
	if err != nil {
		return err
	}
	if err = c.Scheduler.CreateCollectionScraper(collection.ID); err != nil {
		log.Error(err)
		return err
	}
	serviceIP := service.Spec.ClusterIP
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
				pc := NewPlanController(ep, collection, c.Scheduler, c.httpClient, c.sc)
				utils.Retry(func() error {
					return pc.deploy(serviceIP)
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

func (c *Controller) TriggerCollection(collection *model.Collection) error {
	var err error
	// Get all the execution plans within the collection
	// Execution plans are the buiding block of a collection.
	// They define the concurrent/duration etc
	// All the pre-fetched resources will go alone with the collection object
	collection.ExecutionPlans, err = collection.GetExecutionPlans()
	if err != nil {
		return err
	}
	engineDataConfigs := prepareCollection(collection)
	for _, ep := range collection.ExecutionPlans {
		plan, err := model.GetPlan(ep.PlanID)
		if err != nil {
			return err
		}
		if plan.TestFile == nil {
			return fmt.Errorf("Triggering plan aborted. There is no Test file in this plan %d", plan.ID)
		}
	}
	runID, err := collection.StartRun()
	if err != nil {
		return err
	}
	planEngineDataConfigs := make(map[int64]enginesModel.PlanEnginesConfig, len(collection.ExecutionPlans))
	plans := make([]*model.Plan, len(collection.ExecutionPlans))
	for i, ep := range collection.ExecutionPlans {
		pc := NewPlanController(ep, collection, c.Scheduler, c.httpClient, c.sc)
		plan, err := model.GetPlan(ep.PlanID)
		if err != nil {
			return err
		}
		plan.TestFile.Content, err = c.storageClient.Download(plan.TestFile.Filepath)
		if err != nil {
			return err
		}
		planEngineDataConfig, err := pc.prepare(plan, engineDataConfigs[i], runID, c.storageClient)
		if err != nil {
			return err
		}
		plans[i] = plan
		pec := enginesModel.PlanEnginesConfig{
			Kind:          plan.Kind,
			Name:          plan.Name,
			Duration:      strconv.Itoa(ep.Duration),
			Concurrency:   strconv.Itoa(ep.Concurrency),
			Rampup:        strconv.Itoa(ep.Rampup),
			EnginesConfig: planEngineDataConfig,
		}
		planEngineDataConfigs[ep.PlanID] = pec
	}
	ingressIP, err := c.Scheduler.GetIngressUrl(collection.ProjectID)
	if err != nil {
		return err
	}
	for _, d := range collection.Data {
		log.Infof("Downloading file %s", d.Filename)
		content, err := c.storageClient.Download(d.Filepath)
		if err != nil {
			return fmt.Errorf("Could not download file %v, link %s", err, d.Filepath)
		}
		d.Content = content
	}
	apiKey, err := c.Scheduler.GetProjectAPIKey(collection.ProjectID)
	if err != nil {
		return err
	}
	ro := cdrclient.ReqOpts{
		Endpoint: ingressIP,
		APIKey:   apiKey,
	}
	if err := c.cdrclient.TriggerCollection(ro, collection, planEngineDataConfigs, plans); err != nil {
		return err
	}
	for _, ep := range collection.ExecutionPlans {
		if err := model.AddRunningPlan(c.sc.Context, collection.ID, ep.PlanID); err != nil {
			return err
		}
	}
	collection.NewRun(runID)
	return nil
}

func (c *Controller) TermCollection(collection *model.Collection, force bool) (e error) {
	eps, err := collection.GetExecutionPlans()
	if err != nil {
		return err
	}
	currRunID, err := collection.GetCurrentRun()
	if err != nil {
		return err
	}
	defer func() {
		for _, ep := range eps {
			model.DeleteRunningPlan(collection.ID, ep.PlanID)
		}
		collection.StopRun()
		collection.RunFinish(currRunID)
	}()
	externalIP, err := c.Scheduler.GetIngressUrl(collection.ProjectID)
	if err != nil {
		return err
	}
	apiKey, err := c.Scheduler.GetProjectAPIKey(collection.ProjectID)
	if err != nil {
		return err
	}
	ro := cdrclient.ReqOpts{
		Endpoint: externalIP,
		APIKey:   apiKey,
	}
	if err := c.cdrclient.TermCollection(ro, collection.ID, eps); err != nil {
		return err
	}
	return e
}

func (c *Controller) TermAndPurgeCollection(collection *model.Collection) (err error) {
	// This is a force remove so we ignore the errors happened at test termination
	defer func() {
		// This is a bit tricky. We only set the error to the outer scope to not nil when e is not nil
		// Otherwise the nil will override the err value in the main func.
		if e := c.calculateUsage(collection); e != nil {
			err = e
		}
	}()
	c.TermCollection(collection, true)
	if err = c.Scheduler.PurgeCollection(collection.ID); err != nil {
		return err
	}

	return err
}

// In this func, we firstly need to check whether the coordinator, scraper is deployed
// Then we need to check
func (c *Controller) CollectionStatus(collection *model.Collection) (*smodel.CollectionStatus, error) {
	eps, err := collection.GetExecutionPlans()
	if err != nil {
		return nil, err
	}
	numberOfEngines := 0
	for _, ep := range eps {
		numberOfEngines += ep.Engines
	}
	cs, err := c.Scheduler.CollectionStatus(collection.ProjectID, collection.ID, eps)
	if err != nil {
		return nil, err
	}
	ingressIP, err := c.Scheduler.GetIngressUrl(collection.ProjectID)
	if err != nil || ingressIP == "" {
		return cs, nil
	}
	apiKey, err := c.Scheduler.GetProjectAPIKey(collection.ProjectID)
	if err != nil {
		return cs, nil
	}
	ro := cdrclient.ReqOpts{
		Endpoint: ingressIP,
		APIKey:   apiKey,
	}
	if err := c.cdrclient.Healthcheck(ro, collection, numberOfEngines); err != nil {
		return cs, nil
	}
	for _, ps := range cs.Plans {
		// TODO! now, for simplicity, we combine the logic together.
		ps.EnginesReachable = ps.Engines == ps.EnginesDeployed && cs.ScraperDeployed
		rp, err := model.GetRunningPlan(collection.ID, ps.PlanID)
		if err != nil {
			continue
		}
		ps.StartedTime = rp.StartedTime
		ps.InProgress = true
	}
	if c.sc.DevMode {
		cs.PoolSize = 100
		cs.PoolStatus = "running"
	}
	return cs, nil
}

func (c *Controller) SubscribeCollection(collection *model.Collection) ([]*Engine, error) {
	eps, err := collection.GetExecutionPlans()
	if err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	connectedEngines := []*Engine{}
	for _, executionPlan := range eps {
		wg.Add(1)
		go func(ep *model.ExecutionPlan) {
			defer wg.Done()
			pc := NewPlanController(ep, collection, c.Scheduler, c.httpClient, c.sc)
			engines, err := pc.subscribe()
			if err != nil {
				return
			}
			connectedEngines = append(connectedEngines, engines...)
		}(executionPlan)
	}
	wg.Wait()
	return connectedEngines, nil
}

func (c *Controller) calculateUsage(collection *model.Collection) error {
	eps, err := collection.GetExecutionPlans()
	if err != nil {
		return err
	}
	vu := 0
	for _, ep := range eps {
		vu += ep.Engines * ep.Concurrency
	}
	return collection.MarkUsageFinished(c.sc.Context, int64(vu))
}
