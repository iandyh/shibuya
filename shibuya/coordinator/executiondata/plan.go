package executiondata

import (
	"fmt"

	"github.com/rakutentech/shibuya/shibuya/coordinator/payload"
	"github.com/rakutentech/shibuya/shibuya/coordinator/storage"
	"github.com/rakutentech/shibuya/shibuya/engines/jmeter"
	"github.com/rakutentech/shibuya/shibuya/engines/locust"
	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
	"github.com/rakutentech/shibuya/shibuya/model"
)

var TestFileHandlerByPlanKind = map[model.PlanKind]func(*storage.PlanFiles, string, string, []byte,
	enginesModel.PlanEnginesConfig) error{
	model.JmeterPlan: jmeter.MakeTestPlan,
	model.LocustPlan: locust.MakeTestPlan,
}

func HandlePlanData(pf *storage.PlanFiles, filename string, fileBytes []byte,
	edc []*enginesModel.EngineDataConfig, planPayload payload.PlanMessage) error {
	if err := pf.StoreDataFile(filename, fileBytes, edc); err != nil {
		return err
	}
	payload := planPayload[pf.PlanID]
	payload.DataFiles[filename] = struct{}{}
	return nil
}

func HandlePlanTestFile(pf *storage.PlanFiles, pec enginesModel.PlanEnginesConfig, filename string, fileBytes []byte) error {
	handlerFunc, ok := TestFileHandlerByPlanKind[pec.Kind]
	if !ok {
		return fmt.Errorf("%s is not supported", string(pec.Kind))
	}
	return handlerFunc(pf, pec.Name, filename, fileBytes, pec)
}
