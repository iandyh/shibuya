package controller

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/rakutentech/shibuya/shibuya/config"
	cdrclient "github.com/rakutentech/shibuya/shibuya/coordinator/client"
	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
	"github.com/rakutentech/shibuya/shibuya/model"
	"github.com/rakutentech/shibuya/shibuya/object_storage"
	"github.com/rakutentech/shibuya/shibuya/scheduler"
	_ "github.com/rakutentech/shibuya/shibuya/utils"
	log "github.com/sirupsen/logrus"
)

type PlanController struct {
	ep         *model.ExecutionPlan
	collection *model.Collection
	scheduler  scheduler.EngineScheduler
	sc         config.ShibuyaConfig
	httpClient *http.Client
}

func NewPlanController(ep *model.ExecutionPlan, collection *model.Collection, scheduler scheduler.EngineScheduler, httpClient *http.Client, sc config.ShibuyaConfig) *PlanController {
	return &PlanController{
		ep:         ep,
		collection: collection,
		scheduler:  scheduler,
		sc:         sc,
		httpClient: httpClient,
	}
}

func (pc *PlanController) deploy(serviceIP string) error {
	engineConfig := findEngineConfig(JmeterEngineType, pc.sc)
	if err := pc.scheduler.DeployPlan(pc.collection.ProjectID, pc.collection.ID, pc.ep.PlanID,
		pc.ep.Engines, serviceIP, engineConfig); err != nil {
		return err
	}
	return nil
}

func (pc *PlanController) prepare(plan *model.Plan, edc *enginesModel.EngineDataConfig, runID int64, storageClient object_storage.StorageInterface) ([]*enginesModel.EngineDataConfig, error) {
	edc.Duration = strconv.Itoa(pc.ep.Duration)
	edc.Concurrency = strconv.Itoa(pc.ep.Concurrency)
	edc.Rampup = strconv.Itoa(pc.ep.Rampup)
	engineDataConfigs := edc.DeepCopies(pc.ep.Engines)
	var err error
	for _, pf := range plan.Data {
		pf.Content, err = storageClient.Download(pf.Filepath)
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < pc.ep.Engines; i++ {
		// we split the data inherited from collection if the plan specifies split too
		if pc.ep.CSVSplit {
			for _, ed := range engineDataConfigs[i].EngineData {
				ed.TotalSplits *= pc.ep.Engines
				ed.CurrentSplit = (ed.CurrentSplit * pc.ep.Engines) + i
			}
		}
		// Add test file to all engines
		engineDataConfigs[i].EngineData[plan.TestFile.Filename] = plan.TestFile
		engineDataConfigs[i].RunID = runID
		engineDataConfigs[i].EngineID = i
		// add all data uploaded in plans. This will override common data if same filename already exists
		for _, d := range plan.Data {
			sf := model.ShibuyaFile{
				Filename:     d.Filename,
				Filepath:     d.Filepath,
				TotalSplits:  1,
				CurrentSplit: 0,
			}
			if pc.ep.CSVSplit {
				sf.TotalSplits = pc.ep.Engines
				sf.CurrentSplit = i
			}
			engineDataConfigs[i].EngineData[d.Filename] = &sf
		}
	}
	return engineDataConfigs, nil
}

func (pc *PlanController) subscribe() ([]shibuyaEngine, error) {
	ep := pc.ep
	collection := pc.collection
	engines, err := generateEnginesWithUrl(ep.Engines, ep.PlanID, collection.ID, collection.ProjectID,
		JmeterEngineType, pc.scheduler, pc.httpClient)
	if err != nil {
		return nil, err
	}
	runID, err := collection.GetCurrentRun()
	if err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	readingEngines := []shibuyaEngine{}
	for _, engine := range engines {
		wg.Add(1)
		go func(engine shibuyaEngine, runID int64) {
			defer wg.Done()
			//After this step, the engine instance has states including stream client
			err := engine.subscribe(runID)
			if err != nil {
				return
			}
			readingEngines = append(readingEngines, engine)
		}(engine, runID)
	}
	wg.Wait()
	log.Printf("Subscribe to Plan %d", ep.PlanID)
	return readingEngines, err
}

func (pc *PlanController) UnSubscribe() {

}

func (pc *PlanController) progress(cdrclient *cdrclient.Client, externalIP string) bool {
	if err := cdrclient.ProgressCheck(externalIP, pc.collection.ID, pc.ep.PlanID); err == nil {
		return true
	}
	return false
}

// TODO: what was the past around force?
func (pc *PlanController) term(cdrclient *cdrclient.Client) error {
	ep := pc.ep
	ingressIP, err := pc.scheduler.GetIngressUrl(pc.collection.ProjectID)
	if err != nil {
		return err
	}
	if err := cdrclient.TermPlan(ingressIP, pc.collection.ID, pc.ep.PlanID); err != nil {
		return err
	}
	model.DeleteRunningPlan(pc.collection.ID, ep.PlanID)
	return nil
}
