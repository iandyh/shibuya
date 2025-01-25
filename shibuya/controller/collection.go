package controller

import (
	"fmt"
	"sync"

	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
	"github.com/rakutentech/shibuya/shibuya/model"
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
			return fmt.Errorf("Triggering plan aborted. There is no Test file (.jmx) in this plan %d", plan.ID)
		}
	}
	runID, err := collection.StartRun()
	if err != nil {
		return err
	}
	planEngineDataConfigs := make(map[int64][]*enginesModel.EngineDataConfig, len(collection.ExecutionPlans))
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
		planEngineDataConfigs[ep.PlanID] = planEngineDataConfig
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
	if err := c.cdrclient.TriggerCollection(ingressIP, collection, planEngineDataConfigs, plans); err != nil {
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

func (c *Controller) SubscribeCollection(collection *model.Collection) ([]shibuyaEngine, error) {
	eps, err := collection.GetExecutionPlans()
	if err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	connectedEngines := []shibuyaEngine{}
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
	if err := c.cdrclient.TermCollection(externalIP, collection.ID, eps); err != nil {
		return err
	}
	return e
}
