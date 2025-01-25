package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rakutentech/shibuya/shibuya/coordinator/payload"
	"github.com/rakutentech/shibuya/shibuya/coordinator/storage"
	"github.com/rakutentech/shibuya/shibuya/engines/jmeter"
	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
)

type FormFileKey string

var (
	keyPattern = "data:%s"
)

func (ffk FormFileKey) makeDataKey(kind string) string {
	return fmt.Sprintf(keyPattern, kind)
}

func (ffk FormFileKey) MakeCollectionDataKey() string {
	return fmt.Sprintf("%s:%s", ffk.makeDataKey("collection"), ffk)
}

func (ffk FormFileKey) MakePlanDataKey() string {
	return fmt.Sprintf("%s:%s", ffk.makeDataKey("plan"), ffk)
}

func (ffk FormFileKey) MakeTestFileKey() string {
	return fmt.Sprintf("test:%s", ffk)
}

func (ffk FormFileKey) IsCollectionData() bool {
	return strings.HasPrefix(string(ffk), ffk.makeDataKey("collection"))
}

func (ffk FormFileKey) IsPlanData() bool {
	return strings.HasPrefix(string(ffk), ffk.makeDataKey("plan"))
}

func (ffk FormFileKey) IsTestFile() bool {
	return strings.HasPrefix(string(ffk), "test:")
}

func (ffk FormFileKey) PlanID() string {
	if ffk.IsTestFile() {
		t := strings.Split(string(ffk), ":")
		if len(t) != 2 {
			return ""
		}
		return t[1]
	}
	if ffk.IsPlanData() {
		t := strings.Split(string(ffk), ":")
		if len(t) != 3 {
			return ""
		}
		return t[2]
	}
	return ""
}

func handlePlanData(pf *storage.PlanFiles, filename string, fileBytes []byte,
	edc []*enginesModel.EngineDataConfig, planPayload payload.PlanMessage) error {
	if err := pf.StoreDataFile(filename, fileBytes, edc); err != nil {
		return err
	}
	payload := planPayload[pf.PlanID]
	payload.DataFiles[filename] = struct{}{}
	return nil
}

func makeStartPayload(r *http.Request, dataConfig map[string][]*enginesModel.EngineDataConfig,
	planStorage map[string]*storage.PlanFiles, pl *payload.Payload) (*payload.Payload, error) {
	formdata := r.MultipartForm
	payloadByPlan := pl.PlanMessage
	for fileKey := range formdata.File {
		file, fileHeader, err := r.FormFile(fileKey)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		var todos []*storage.PlanFiles
		ffk := FormFileKey(fileKey)
		if ffk.IsCollectionData() {
			for _, pf := range planStorage {
				todos = append(todos, pf)
			}
		}
		if ffk.IsPlanData() {
			planID := ffk.PlanID()
			pf := planStorage[planID]
			todos = append(todos, pf)
		}
		for _, pf := range todos {
			if err := handlePlanData(pf, fileHeader.Filename, fileBytes, dataConfig[pf.PlanID], payloadByPlan); err != nil {
				return nil, err
			}
		}
		if ffk.IsTestFile() {
			planID := ffk.PlanID()
			engineCfg := dataConfig[planID][0]
			modified, err := jmeter.ModifyJMX(fileBytes, engineCfg.Concurrency, engineCfg.Duration, engineCfg.Rampup)
			if err != nil {
				return nil, err
			}
			pf := planStorage[planID]
			if err := pf.StoreTestPlan(fileHeader.Filename, modified); err != nil {
				return nil, err
			}
			payload := payloadByPlan[planID]
			payload.TestFile = fileHeader.Filename
		}
	}
	return pl, nil
}
