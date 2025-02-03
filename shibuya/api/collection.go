package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/controller"
	httproute "github.com/rakutentech/shibuya/shibuya/http/route"
	"github.com/rakutentech/shibuya/shibuya/model"
	"github.com/rakutentech/shibuya/shibuya/object_storage"
	utils "github.com/rakutentech/shibuya/shibuya/utils"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type CollectionAPI struct {
	objStorage object_storage.StorageInterface
	ctr        *controller.Controller
	sc         config.ShibuyaConfig
}

func NewCollectionAPI(sc config.ShibuyaConfig, objStorage object_storage.StorageInterface, ctr *controller.Controller) *CollectionAPI {
	ca := &CollectionAPI{
		objStorage: objStorage,
		ctr:        ctr,
		sc:         sc,
	}
	return ca
}

func (ca *CollectionAPI) Router() *httproute.Router {
	router := httproute.NewRouter("collection api", "/collections")
	router.AddRoutes(httproute.Routes{
		{
			Name:        "Create a collection",
			Method:      "POST",
			HandlerFunc: ca.collectionCreateHandler,
		},
		{
			Name:        "Get a collection",
			Method:      "GET",
			Path:        "{collection_id}",
			HandlerFunc: ca.collectionGetHandler,
		},
		{
			Name:        "Update a collection",
			Method:      "PUT",
			Path:        "{collection_id}",
			HandlerFunc: ca.collectionUpdateHandler,
		},
		{
			Name:        "Delete a collection",
			Method:      "DELETE",
			Path:        "{collection_id}",
			HandlerFunc: ca.collectionDeleteHandler,
		},

		{
			Name:        "Upload a collection files",
			Method:      "PUT",
			Path:        "{collection_id}/files",
			HandlerFunc: ca.collectionFilesUploadHandler,
		},
		{
			Name:        "Get a collection files",
			Method:      "GET",
			Path:        "{collection_id}/files",
			HandlerFunc: ca.collectionFilesGetHandler,
		},
		{
			Name:        "Delete a collection files",
			Method:      "DELETE",
			Path:        "{collection_id}/files",
			HandlerFunc: ca.collectionFilesDeleteHandler,
		},

		{
			Name:        "Update a collection config",
			Method:      "PUT",
			Path:        "{collection_id}/config",
			HandlerFunc: ca.collectionUploadHandler,
		},
		{
			Name:        "GET a collection config as a file",
			Method:      "GET",
			Path:        "{collection_id}/config",
			HandlerFunc: ca.collectionConfigGetHandler,
		},

		{
			Name:        "GET collection engine details",
			Method:      "GET",
			Path:        "{collection_id}/details",
			HandlerFunc: ca.collectionEnginesDetailHandler,
		},

		// Below is the collection launch/trigger/stop/purge action apis
		{
			Name:        "Launch a collection",
			Method:      "POST",
			Path:        "{collection_id}/deploy",
			HandlerFunc: ca.collectionDeploymentHandler,
		},
		{
			Name:        "Trigger a collection",
			Method:      "POST",
			Path:        "{collection_id}/trigger",
			HandlerFunc: ca.collectionTriggerHandler,
		},
		{
			Name:        "Stop a collection",
			Method:      "POST",
			Path:        "{collection_id}/stop",
			HandlerFunc: ca.collectionTermHandler,
		},
		{
			Name:        "Purge a collection",
			Method:      "POST",
			Path:        "{collection_id}/purge",
			HandlerFunc: ca.collectionPurgeHandler,
		},

		{
			Name:        "Get collection runs",
			Method:      "GET",
			Path:        "{collection_id}/runs",
			HandlerFunc: ca.runGetHandler,
		},
		{
			Name:        "Get collection runs",
			Method:      "GET",
			Path:        "{collection_id}/runs/{run_id}",
			HandlerFunc: ca.runGetHandler,
		},
		{
			Name:        "Delete a collection run",
			Method:      "DELETE",
			Path:        "{collection_id}/runs/{run_id}",
			HandlerFunc: ca.runDeleteHandler,
		},
		{
			Name:        "Delete collection runs",
			Method:      "DELETE",
			Path:        "{collection_id}/runs",
			HandlerFunc: ca.runDeleteHandler,
		},

		{
			Name:        "Get collection status",
			Method:      "GET",
			Path:        "{collection_id}/status",
			HandlerFunc: ca.collectionStatusHandler,
		},

		{
			Name:        "Stream collection metrics",
			Method:      "GET",
			Path:        "{collection_id}/stream",
			HandlerFunc: ca.streamCollectionMetrics,
		},
		{
			Name:        "Handle a collection plan log",
			Method:      "GET",
			Path:        "{collection_id}/logs/{plan_id}",
			HandlerFunc: ca.planLogHandler,
		},
	})
	return router
}

func getCollection(collectionID string) (*model.Collection, error) {
	cid, err := strconv.Atoi(collectionID)
	if err != nil {
		return nil, makeInvalidResourceError("collection_id")
	}
	collection, err := model.GetCollection(int64(cid))
	if err != nil {
		return nil, err
	}
	return collection, nil
}

func hasInvalidDiff(curr, updated []*model.ExecutionPlan) (bool, string) {
	if len(updated) != len(curr) {
		return true, "You cannot add/remove plans while have engines deployed"
	}
	currCache := make(map[int64]*model.ExecutionPlan)
	for _, item := range curr {
		currCache[item.PlanID] = item
	}
	for _, item := range updated {
		currPlan, ok := currCache[item.PlanID]
		if !ok {
			return true, "You cannot add a new plan while having engines deployed"
		}
		if currPlan.Engines != item.Engines {
			return true, "You cannot change engine numbers while having engines deployed"
		}
		if currPlan.Concurrency != item.Concurrency {
			return true, "You cannot change concurrency while having engines deployed"
		}
	}
	return false, ""
}

func (ca *CollectionAPI) collectionCreateHandler(w http.ResponseWriter, r *http.Request) {
	account := r.Context().Value(accountKey).(*model.Account)
	r.ParseForm()
	collectionName := r.Form.Get("name")
	if collectionName == "" {
		handleErrors(w, makeInvalidRequestError("collection name cannot be empty"))
		return
	}
	projectID := r.Form.Get("project_id")
	project, err := getProject(projectID)
	if err != nil {
		handleErrors(w, err)
		return
	}
	if r := hasProjectOwnership(project, account, ca.sc.AuthConfig); !r {
		handleErrors(w, makeProjectOwnershipError())
		return
	}
	collectionID, err := model.CreateCollection(collectionName, project.ID)
	if err != nil {
		handleErrors(w, err)
		return
	}
	collection, err := model.GetCollection(int64(collectionID))
	if err != nil {
		handleErrors(w, err)
		return
	}
	renderJSON(w, http.StatusOK, collection)
}

func (ca *CollectionAPI) collectionGetHandler(w http.ResponseWriter, r *http.Request) {
	collection, err := hasCollectionOwnership(r, ca.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	// we ignore errors here as the front end will do the retry
	collection.ExecutionPlans, _ = collection.GetExecutionPlans()
	collection.RunHistories, _ = collection.GetRuns()
	renderJSON(w, http.StatusOK, collection)
}

func (ca *CollectionAPI) collectionUpdateHandler(w http.ResponseWriter, _ *http.Request) {
	renderJSON(w, http.StatusNotImplemented, nil)
}

func (ca *CollectionAPI) collectionDeleteHandler(w http.ResponseWriter, r *http.Request) {
	collection, err := hasCollectionOwnership(r, ca.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	if ca.ctr.Scheduler.PodReadyCount(collection.ID) > 0 {
		handleErrors(w, makeInvalidRequestError("You cannot launch engines when there are engines already deployed"))
		return
	}
	runningPlans, err := model.GetRunningPlansByCollection(collection.ID)
	if err != nil {
		handleErrors(w, err)
		return
	}
	if len(runningPlans) > 0 {
		handleErrors(w, makeInvalidRequestError("You cannot delete the collection during testing period"))
		return
	}
	collection.Delete(ca.objStorage)
}

func (ca *CollectionAPI) collectionUploadHandler(w http.ResponseWriter, r *http.Request) {
	collection, err := hasCollectionOwnership(r, ca.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	e := new(model.ExecutionWrapper)
	r.ParseMultipartForm(1 << 20) //parse 1 MB of data
	file, _, err := r.FormFile("collectionYAML")
	if err != nil {
		handleErrors(w, makeInvalidResourceError("file"))
		return
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		handleErrors(w, makeInvalidRequestError("invalid file"))
		return
	}
	err = yaml.Unmarshal(raw, e)
	if err != nil {
		handleErrors(w, makeInvalidRequestError(err.Error()))
		return
	}
	if e.Content.CollectionID != collection.ID {
		handleErrors(w, makeInvalidRequestError("collection ID mismatch"))
		return
	}
	project, err := model.GetProject(collection.ProjectID)
	if err != nil {
		log.Error(err)
		handleErrors(w, err)
		return
	}
	totalEnginesRequired := 0
	for _, ep := range e.Content.Tests {
		plan, err := model.GetPlan(ep.PlanID)
		if err != nil {
			handleErrors(w, err)
			return
		}
		planProject, err := model.GetProject(plan.ProjectID)
		if err != nil {
			handleErrors(w, err)
			return
		}
		if project.ID != planProject.ID {
			handleErrors(w, makeInvalidRequestError("You can only add plan within the same project"))
			return
		}
		totalEnginesRequired += ep.Engines
	}
	sc := ca.sc
	if totalEnginesRequired > sc.ExecutorConfig.MaxEnginesInCollection {
		errMsg := fmt.Sprintf("You are reaching the resource limit of the cluster. Requesting engines: %d, limit: %d.",
			totalEnginesRequired, sc.ExecutorConfig.MaxEnginesInCollection)
		handleErrors(w, makeInvalidRequestError(errMsg))
		return
	}
	runningPlans, err := model.GetRunningPlansByCollection(collection.ID)
	if err != nil {
		handleErrors(w, err)
		return
	}
	if len(runningPlans) > 0 {
		handleErrors(w, makeInvalidRequestError("You cannot change the collection during testing period"))
		return
	}
	for _, ep := range e.Content.Tests {
		if ep.Engines <= 0 {
			handleErrors(w, makeInvalidRequestError("You cannot configure a plan with zero engine"))
			return
		}
	}
	if ca.ctr.Scheduler.PodReadyCount(collection.ID) > 0 {
		currentPlans, err := collection.GetExecutionPlans()
		if err != nil {
			handleErrors(w, err)
			return
		}
		if ok, message := hasInvalidDiff(currentPlans, e.Content.Tests); ok {
			handleErrors(w, makeInvalidRequestError(message))
			return
		}

	}
	err = collection.Store(e.Content)
	if err != nil {
		handleErrors(w, err)
	}
}

func (ca *CollectionAPI) collectionConfigGetHandler(w http.ResponseWriter, req *http.Request) {
	collection, err := hasCollectionOwnership(req, ca.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	eps, err := collection.GetExecutionPlans()
	if err != nil {
		handleErrors(w, err)
		return
	}
	for _, ep := range eps {
		plan, err := model.GetPlan(ep.PlanID)
		if err != nil {
			handleErrors(w, err)
			return
		}
		ep.Name = plan.Name
	}
	e := &model.ExecutionWrapper{
		Content: &model.ExecutionCollection{
			Name:         collection.Name,
			ProjectID:    collection.ProjectID,
			CollectionID: collection.ID,
			Tests:        eps,
			CSVSplit:     collection.CSVSplit,
		},
	}
	content, err := yaml.Marshal(e)
	if err != nil {
		handleErrors(w, err)
		return
	}
	r := bytes.NewReader(content)
	filename := fmt.Sprintf("%d.yaml", collection.ID)
	header := fmt.Sprintf("Attachment; filename=%s", filename)
	w.Header().Add("Content-Disposition", header)

	http.ServeContent(w, req, filename, time.Now(), r)
}

func (ca *CollectionAPI) collectionFilesGetHandler(w http.ResponseWriter, _ *http.Request) {
	renderJSON(w, http.StatusNotImplemented, nil)
}

func (ca *CollectionAPI) collectionFilesUploadHandler(w http.ResponseWriter, r *http.Request) {
	collection, err := hasCollectionOwnership(r, ca.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	r.ParseMultipartForm(100 << 20) //parse 100 MB of data
	file, handler, err := r.FormFile("collectionFile")
	if err != nil {
		handleErrors(w, makeInvalidRequestError("Something wrong with file you uploaded"))
		return
	}
	if err = collection.StoreFile(ca.objStorage, handler.Filename, file); err != nil {
		handleErrors(w, err)
		return
	}

	w.Write([]byte("success"))
}

func (ca *CollectionAPI) collectionFilesDeleteHandler(w http.ResponseWriter, r *http.Request) {
	collection, err := hasCollectionOwnership(r, ca.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	r.ParseForm()
	filename := r.Form.Get("filename")
	if filename == "" {
		handleErrors(w, makeInvalidRequestError("Collection file name cannot be empty"))
		return
	}
	err = collection.DeleteFile(ca.objStorage, filename)
	if err != nil {
		handleErrors(w, makeInternalServerError("Deletion was unsuccessful"))
		return
	}
	w.Write([]byte("Deleted successfully"))
}

func (ca *CollectionAPI) collectionEnginesDetailHandler(w http.ResponseWriter, r *http.Request) {
	collection, err := hasCollectionOwnership(r, ca.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	collectionDetails, err := ca.ctr.Scheduler.GetCollectionEnginesDetail(collection.ProjectID, collection.ID)
	if err != nil {
		handleErrors(w, err)
		return
	}
	renderJSON(w, http.StatusOK, collectionDetails)
}

func (ca *CollectionAPI) collectionDeploymentHandler(w http.ResponseWriter, r *http.Request) {
	collection, err := hasCollectionOwnership(r, ca.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	if err := ca.ctr.DeployCollection(collection); err != nil {
		var dbe *model.DBError
		if errors.As(err, &dbe) {
			handleErrors(w, makeInvalidRequestError(err.Error()))
			return
		}
		handleErrors(w, makeInternalServerError(err.Error()))
		return
	}
}

func (ca *CollectionAPI) collectionTriggerHandler(w http.ResponseWriter, r *http.Request) {
	collection, err := hasCollectionOwnership(r, ca.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	if err := ca.ctr.TriggerCollection(collection); err != nil {
		handleErrors(w, err)
		return
	}
}

func (ca *CollectionAPI) collectionTermHandler(w http.ResponseWriter, r *http.Request) {
	collection, err := hasCollectionOwnership(r, ca.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	if err := ca.ctr.TermCollection(collection, false); err != nil {
		handleErrors(w, makeInternalServerError(err.Error()))
		return
	}
}

func (ca *CollectionAPI) collectionPurgeHandler(w http.ResponseWriter, r *http.Request) {
	collection, err := hasCollectionOwnership(r, ca.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	if err = ca.ctr.TermAndPurgeCollection(collection); err != nil {
		handleErrors(w, err)
		return
	}
}

func (ca *CollectionAPI) runGetHandler(w http.ResponseWriter, _ *http.Request) {
	renderJSON(w, http.StatusNotImplemented, nil)
}

func (ca *CollectionAPI) runDeleteHandler(w http.ResponseWriter, _ *http.Request) {
	renderJSON(w, http.StatusNotImplemented, nil)
}

func (ca *CollectionAPI) collectionStatusHandler(w http.ResponseWriter, r *http.Request) {
	collection, err := hasCollectionOwnership(r, ca.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	collectionStatus, err := ca.ctr.CollectionStatus(collection)
	if err != nil {
		handleErrors(w, err)
		return
	}
	renderJSON(w, http.StatusOK, collectionStatus)
}

func (ca *CollectionAPI) streamCollectionMetrics(w http.ResponseWriter, r *http.Request) {
	collection, err := hasCollectionOwnership(r, ca.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	if cid, _ := collection.GetCurrentRun(); cid == 0 {
		msg := fmt.Sprintf("Current collection %d does not have any run", collection.ID)
		makeFailMessage(w, msg, http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientIP := retrieveClientIP(r)
	currentTime := time.Now().Unix()
	item := &controller.ApiMetricStream{
		StreamClient: make(chan *controller.ApiMetricStreamEvent),
		CollectionID: fmt.Sprintf("%d", collection.ID),
		ClientID:     fmt.Sprintf("%s-%v-%s", clientIP, currentTime, utils.RandStringRunes(6)),
	}
	ca.ctr.ApiNewClients <- item
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		ca.ctr.ApiClosingClients <- item
	}()
	for event := range item.StreamClient {
		if event == nil {
			continue
		}
		s, err := json.Marshal(event)
		if err != nil {
			fmt.Fprintf(w, "data:%v\n\n", err)
		} else {
			fmt.Fprintf(w, "data:%s\n\n", s)
		}
		flusher.Flush()
	}
}

func (ca *CollectionAPI) planLogHandler(w http.ResponseWriter, r *http.Request) {
	collectionID, err := strconv.Atoi(r.PathValue("collection_id"))
	if err != nil {
		handleErrors(w, makeInvalidResourceError("collection_id"))
		return
	}
	planID, err := strconv.Atoi(r.PathValue("plan_id"))
	if err != nil {
		handleErrors(w, makeInvalidResourceError("plan_id"))
		return
	}
	content, err := ca.ctr.Scheduler.DownloadPodLog(int64(collectionID), int64(planID))
	if err != nil {
		handleErrors(w, makeInvalidRequestError(err.Error()))
		return
	}
	m := make(map[string]string)
	m["c"] = content
	renderJSON(w, http.StatusOK, m)
}
