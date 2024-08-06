package controller

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/beevik/etree"
	"github.com/rakutentech/shibuya/shibuya/config"
	controllerModel "github.com/rakutentech/shibuya/shibuya/controller/model"
	"github.com/rakutentech/shibuya/shibuya/model"
	sos "github.com/rakutentech/shibuya/shibuya/object_storage"
	"github.com/rakutentech/shibuya/shibuya/scheduler"
	utils "github.com/rakutentech/shibuya/shibuya/utils"
	log "github.com/sirupsen/logrus"
)

type PlanController struct {
	ep         *model.ExecutionPlan
	collection *model.Collection
	scheduler  scheduler.EngineScheduler
}

func NewPlanController(ep *model.ExecutionPlan, collection *model.Collection, scheduler scheduler.EngineScheduler) *PlanController {
	return &PlanController{
		ep:         ep,
		collection: collection,
		scheduler:  scheduler,
	}
}

func (pc *PlanController) deploy() error {
	engineConfig := findEngineConfig(JmeterEngineType)
	if err := pc.scheduler.DeployPlan(pc.collection.ProjectID, pc.collection.ID, pc.ep.PlanID,
		pc.ep.Engines, engineConfig); err != nil {
		return err
	}
	return nil
}

func GetThreadGroups(planDoc *etree.Document) ([]*etree.Element, error) {
	jtp := planDoc.SelectElement("jmeterTestPlan")
	if jtp == nil {
		return nil, errors.New("Missing Jmeter Test plan in jmx")
	}
	ht := jtp.SelectElement("hashTree")
	if ht == nil {
		return nil, errors.New("Missing hash tree inside Jmeter test plan in jmx")
	}
	ht = ht.SelectElement("hashTree")
	if ht == nil {
		return nil, errors.New("Missing hash tree inside hash tree in jmx")
	}
	tgs := ht.SelectElements("ThreadGroup")
	stgs := ht.SelectElements("SetupThreadGroup")
	tgs = append(tgs, stgs...)
	return tgs, nil
}

func parseTestPlan(file []byte) (*etree.Document, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(file); err != nil {
		return nil, err
	}
	return doc, nil
}

func modifyJMX(file []byte, threads, duration, rampTime string) ([]byte, error) {
	planDoc, err := parseTestPlan(file)
	if err != nil {
		return nil, err
	}
	durationInt, err := strconv.Atoi(duration)
	if err != nil {
		return nil, err
	}
	// it includes threadgroups and setupthreadgroups
	threadGroups, err := GetThreadGroups(planDoc)
	if err != nil {
		return nil, err
	}
	for _, tg := range threadGroups {
		children := tg.ChildElements()
		for _, child := range children {
			attrName := child.SelectAttrValue("name", "")
			switch attrName {
			case "ThreadGroup.duration":
				child.SetText(strconv.Itoa(durationInt * 60))
			case "ThreadGroup.scheduler":
				child.SetText("true")
			case "ThreadGroup.num_threads":
				child.SetText(threads)
			case "ThreadGroup.ramp_time":
				child.SetText(rampTime)
			}
		}
	}
	return planDoc.WriteToBytes()
}

func (pc *PlanController) prepareJMX(file []byte, threads, duration, rampTime string) ([]byte, error) {
	modified, err := modifyJMX(file, threads, duration, rampTime)
	if err != nil {
		return []byte{}, err
	}
	return modified, nil
}

func (pc *PlanController) prepare(plan *model.Plan, edc *controllerModel.EngineDataConfig) []*controllerModel.EngineDataConfig {
	storageClient := sos.Client.Storage
	edc.Duration = strconv.Itoa(pc.ep.Duration)
	edc.Concurrency = strconv.Itoa(pc.ep.Concurrency)
	edc.Rampup = strconv.Itoa(pc.ep.Rampup)
	engineDataConfigs := edc.DeepCopies(pc.ep.Engines)
	TestPlanFile, err := storageClient.Download(plan.TestFile.Filepath)
	if err != nil {
		return engineDataConfigs
	}
	modifiedPlan, err := pc.prepareJMX(TestPlanFile, edc.Concurrency, edc.Duration, edc.Rampup)
	if err != nil {
		return engineDataConfigs
	}
	planData := make(map[string][]byte)
	for _, d := range plan.Data {
		file, err := storageClient.Download(d.Filepath)
		if err != nil {
			return engineDataConfigs
		}
		planData[d.Filename] = file
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
		plan.TestFile.FileContent = modifiedPlan
		engineDataConfigs[i].EngineData[plan.TestFile.Filename] = plan.TestFile
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
			splittedFile, err := utils.SplitCSV(planData[d.Filename], sf.TotalSplits, sf.CurrentSplit)
			if err != nil {
				//TODO: handle the error here
				continue
			}
			sf.FileContent = splittedFile
			engineDataConfigs[i].EngineData[d.Filename] = &sf
		}
	}
	return engineDataConfigs
}

func (pc *PlanController) trigger(engineDataConfig *controllerModel.EngineDataConfig) error {
	plan, err := model.GetPlan(pc.ep.PlanID)
	if err != nil {
		return err
	}

	engineDataConfigs := pc.prepare(plan, engineDataConfig)
	engines, err := generateEnginesWithUrl(pc.ep.Engines, pc.ep.PlanID, pc.collection.ID, pc.collection.ProjectID,
		JmeterEngineType, pc.scheduler)
	if err != nil {
		return err
	}
	errs := make(chan error, len(engines))
	defer close(errs)
	planErrors := []error{}
	for i, engine := range engines {
		go func(engine shibuyaEngine, i int) {
			if err := engine.trigger(engineDataConfigs[i]); err != nil {
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

func makePlanEngineKey(collectionID, planID int64, engineID int) string {
	return fmt.Sprintf("%s-%d-%d-%d", config.SC.Context, collectionID, planID, engineID)
}

func (pc *PlanController) subscribe(connectedEngines *sync.Map, readingEngines chan shibuyaEngine) error {
	ep := pc.ep
	collection := pc.collection
	engines, err := generateEnginesWithUrl(ep.Engines, ep.PlanID, collection.ID, collection.ProjectID,
		JmeterEngineType, pc.scheduler)
	if err != nil {
		return err
	}
	runID, err := collection.GetCurrentRun()
	if err != nil {
		return err
	}
	for _, engine := range engines {
		go func(engine shibuyaEngine, runID int64) {
			//After this step, the engine instance has states including stream client
			err := engine.subscribe(runID)
			if err != nil {
				return
			}
			key := makePlanEngineKey(collection.ID, ep.PlanID, engine.EngineID())
			if _, loaded := connectedEngines.LoadOrStore(key, engine); !loaded {
				readingEngines <- engine
				log.Printf("Engine %s is subscribed", key)
				return
			}
			// This might be triggered by some cases that multiple streams are being estabalished at the same time
			// for example, when the plan was broken and later replaced by a working one without purging the engines
			// In this case, we only mainain the first stream and close the current one
			engine.closeStream()
			log.Printf("Duplicate stream of engine %s is closed", key)
		}(engine, runID)
	}
	log.Printf("Subscribe to Plan %d", ep.PlanID)
	return nil
}

// TODO. we can use the cached clients here.
func (pc *PlanController) progress() bool {
	r := true
	ep := pc.ep
	collection := pc.collection
	engines, err := generateEnginesWithUrl(ep.Engines, ep.PlanID, collection.ID, collection.ProjectID, JmeterEngineType, pc.scheduler)
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

func (pc *PlanController) term(force bool, connectedEngines *sync.Map) error {
	var wg sync.WaitGroup
	ep := pc.ep
	for i := 0; i < ep.Engines; i++ {
		key := makePlanEngineKey(pc.collection.ID, ep.PlanID, i)
		item, ok := connectedEngines.Load(key)
		if ok {
			wg.Add(1)
			engine := item.(shibuyaEngine)
			go func(engine shibuyaEngine) {
				defer wg.Done()
				engine.terminate(force)
				connectedEngines.Delete(key)
				log.Printf("Engine %s is terminated", key)
			}(engine)
		}
	}
	wg.Wait()
	model.DeleteRunningPlan(pc.collection.ID, ep.PlanID)
	return nil
}
