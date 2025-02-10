package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rakutentech/shibuya/shibuya/coordinator/executiondata"
	"github.com/rakutentech/shibuya/shibuya/coordinator/payload"
	"github.com/rakutentech/shibuya/shibuya/coordinator/storage"
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

func makeStartPayload(r *http.Request, dataConfig map[string]enginesModel.PlanEnginesConfig,
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
			if err := executiondata.HandlePlanData(pf, fileHeader.Filename, fileBytes, dataConfig[pf.PlanID].EnginesConfig, payloadByPlan); err != nil {
				return nil, err
			}
		}
		if ffk.IsTestFile() {
			planID := ffk.PlanID()
			if err := executiondata.HandlePlanTestFile(planStorage[planID], dataConfig[ffk.PlanID()],
				fileHeader.Filename, fileBytes); err != nil {
				return nil, err
			}
			payloadByPlan[planID].TestFile = fileHeader.Filename
		}
	}
	return pl, nil
}
