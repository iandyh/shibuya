package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/rakutentech/shibuya/shibuya/config"
	controllermodel "github.com/rakutentech/shibuya/shibuya/controller/model"
	"github.com/rakutentech/shibuya/shibuya/model"
	"github.com/rakutentech/shibuya/shibuya/scheduler"
	_ "github.com/rakutentech/shibuya/shibuya/utils"
	log "github.com/sirupsen/logrus"
)

type PlanController struct {
	ep         *model.ExecutionPlan
	collection *model.Collection
	kcm        *scheduler.K8sClientManager
}

func NewPlanController(ep *model.ExecutionPlan, collection *model.Collection, kcm *scheduler.K8sClientManager) *PlanController {
	return &PlanController{
		ep:         ep,
		collection: collection,
		kcm:        kcm,
	}
}

func (pc *PlanController) deploy() error {
	engines, err := generateEngines(pc.ep.Engines, pc.ep.PlanID, pc.collection.ID, pc.collection.ProjectID,
		JmeterEngineType)
	if err != nil {
		return err
	}
	// We don't need concurrent operation here because the bottleneck will be getting public ip
	// not the deployment
	for _, engine := range engines {
		// need to move the jmeter image into earlier stage. k8s client should not care about the image
		if err := engine.deploy(pc.kcm); err != nil {
			return err
		}
	}
	return nil
}

func (pc *PlanController) prepare(plan *model.Plan, commonData map[string]*model.ShibuyaFile) (*ExecutionData, error) {
	err := plan.FetchPlanFiles()
	if err != nil {
		return &ExecutionData{}, err
	}
	splittedData := newExecutionData(pc.ep.Engines, pc.ep.CSVSplit)

	// first iterate through all the common data uploaded in collection
	for _, d := range commonData {
		if err := splittedData.PrepareExecutionData(d); err != nil {
			return &ExecutionData{}, err
		}
	}

	// then go through all data uploaded in plans. This will override common data if same filename already exists
	for _, d := range plan.Data {
		if err := splittedData.PrepareExecutionData(d); err != nil {
			return &ExecutionData{}, err
		}
	}
	// Add test file to all engines
	for i := 0; i < pc.ep.Engines; i++ {
		splittedData.files[i][plan.TestFile.Filename] = plan.TestFile
	}
	// dereference the data files so gc can remove them if not needed anymore
	plan.Data = []*model.ShibuyaFile{}
	return splittedData, nil
}

func (pc *PlanController) trigger(collectionExecutionData map[string]*model.ShibuyaFile) error {
	plan, err := model.GetPlan(pc.ep.PlanID)
	if err != nil {
		return err
	}
	planExecutionData, err := pc.prepare(plan, collectionExecutionData)
	if err != nil {
		return err
	}
	engines, err := generateEnginesWithUrl(pc.ep.Engines, pc.ep.PlanID, pc.collection.ID, pc.collection.ProjectID,
		JmeterEngineType, pc.kcm)
	if err != nil {
		return err
	}
	errs := make(chan error, len(engines))
	defer close(errs)
	planErrors := []error{}
	for i, engine := range engines {
		go func(engine shibuyaEngine, i int) {
			// Is it a good idea to pass the ep into engine?
			if err := engine.trigger(plan.TestFile.Filename, pc.ep, planExecutionData.files[i]); err != nil {
				errs <- err
				return
			}
			errs <- nil
		}(engine, i)
	}
	for i := 0; i < len(engines); i++ {
		if err := <-errs; err != nil {
			planErrors = append(planErrors, err)
		}
	}
	if len(planErrors) > 0 {
		return fmt.Errorf("Trigger plan errors:%v", planErrors)
	}
	log.Printf("Triggering for plan %d is finished", pc.ep.PlanID)
	return nil
}

func (pc *PlanController) subscribe(connectedEngines *sync.Map, readingEngines chan shibuyaEngine) error {
	ep := pc.ep
	collection := pc.collection
	engines, err := generateEnginesWithUrl(ep.Engines, ep.PlanID, collection.ID, collection.ProjectID,
		JmeterEngineType, pc.kcm)
	if err != nil {
		return err
	}
	runID, err := collection.GetCurrentRun()
	if err != nil {
		return err
	}
	rds := config.SC.RDS
	ctx := context.Background()
	for _, engine := range engines {
		t := &controllermodel.WorkLoad{
			Context:      config.SC.Context,
			EngineUrl:    engine.url(),
			RunID:        runID,
			CollectionID: collection.ID,
			PlanID:       ep.PlanID,
			ProjectID:    collection.ProjectID,
		}
		r, _ := json.Marshal(t)
		err := rds.LPush(ctx, controllermodel.SubscriptionQueue, string(r)).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

// TODO. we can use the cached clients here.
func (pc *PlanController) progress() bool {
	r := true
	ep := pc.ep
	collection := pc.collection
	engines, err := generateEnginesWithUrl(ep.Engines, ep.PlanID, collection.ID, collection.ProjectID, JmeterEngineType, pc.kcm)
	if errors.Is(err, scheduler.IngressError) {
		log.Error(err)
		return true
	} else if err != nil {
		return false
	}
	for _, engine := range engines {
		engineRunning := engine.progress()
		r = r && !engineRunning
	}
	return !r
}

func (pc *PlanController) term(force bool) error {
	ep := pc.ep
	collection := pc.collection
	if !force {
		engines, err := generateEnginesWithUrl(ep.Engines, ep.PlanID, collection.ID, collection.ProjectID, JmeterEngineType, pc.kcm)
		if err != nil {
			return err
		}
		for _, engine := range engines {
			rds := config.SC.RDS
			ctx := context.Background()
			t := &controllermodel.WorkLoad{
				EngineUrl: engine.url(),
				//RunID:        runID,
				CollectionID: collection.ID,
				PlanID:       ep.PlanID,
				ProjectID:    collection.ProjectID,
			}
			r, _ := json.Marshal(t)
			err := rds.LPush(ctx, controllermodel.TermQueue, string(r)).Err()
			if err != nil {
				return err
			}
		}
	}
	model.DeleteRunningPlan(collection.ID, ep.PlanID)
	return nil
}
