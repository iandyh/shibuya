package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	payload "github.com/rakutentech/shibuya/shibuya/coordinator/payload"
	"github.com/rakutentech/shibuya/shibuya/coordinator/storage"
	"github.com/rakutentech/shibuya/shibuya/coordinator/upstream"
	httproute "github.com/rakutentech/shibuya/shibuya/http/route"

	"github.com/rakutentech/shibuya/shibuya/coordinator/planprogress"
	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
	pubsub "github.com/reqfleet/pubsub/server"
)

type APIServer struct {
	pubsubServer *pubsub.PubSubServer
	planProgress *planprogress.PlanProgress
	inventory    *upstream.Inventory
}

func NewAPIServer(server *pubsub.PubSubServer, planProgress *planprogress.PlanProgress, inventory *upstream.Inventory) *APIServer {
	s := &APIServer{pubsubServer: server, planProgress: planProgress, inventory: inventory}
	return s
}

func (s *APIServer) Router() *httproute.Router {
	collectionRoutes := httproute.Routes{
		{
			Name:        "collection",
			Method:      "POST",
			Path:        "{collection_id}",
			HandlerFunc: s.collectionTriggerHandler,
		},
		{
			Name:        "collection",
			Method:      "GET",
			Path:        "{collection_id}",
			HandlerFunc: s.collectionHealthCheckHandler,
		},
		{
			Name:        "stop collection",
			Method:      "DELETE",
			Path:        "{collection_id}",
			HandlerFunc: s.collectionTermHandler,
		},
		{
			Name:        "collection plan running status",
			Method:      "GET",
			Path:        "{collection_id}/{plan_id}",
			HandlerFunc: s.collectionProgressHandler,
		},
		{
			Name:        "Term a plan",
			Method:      "DELETE",
			Path:        "/{collection_id}/{plan_id}",
			HandlerFunc: s.planTerminationHandler,
		},
		{
			Name:        "report engine running status",
			Method:      "PUT",
			Path:        "/{collection_id}/{plan_id}/{engine_id}",
			HandlerFunc: s.engineReportProgressHandler,
		},
	}
	collectionRouter := &httproute.Router{
		Name: "collection handlers",
		Path: "/collections",
	}
	collectionRouter.AddRoutes(collectionRoutes)
	apiRouter := &httproute.Router{
		Name: "api",
		Path: "/api",
	}
	apiRouter.Mount(collectionRouter)
	return apiRouter
}

func (s *APIServer) collectionTriggerHandler(w http.ResponseWriter, r *http.Request) {
	collectionID := r.PathValue("collection_id")
	if err := r.ParseMultipartForm(100 << 20); err != nil { // limit your max input length!
		http.Error(w, "Unable to parse multipart form", http.StatusBadRequest)
		return
	}
	formdata := r.MultipartForm
	engineData := formdata.Value["engine_data"]

	dataConfig := make(map[string]enginesModel.PlanEnginesConfig)
	if err := json.Unmarshal([]byte(engineData[0]), &dataConfig); err != nil {
		http.Error(w, "Error parsing JSON", http.StatusBadRequest)
		return
	}
	pl := &payload.Payload{
		Verb:        "start",
		PlanMessage: make(payload.PlanMessage),
	}
	planStorage := make(map[string]*storage.PlanFiles, len(dataConfig))
	payloadByPlan := pl.PlanMessage
	totalEngines := 0
	for planID, planConfig := range dataConfig {
		enginesConfig := planConfig.EnginesConfig
		planStorage[planID] = storage.NewPlanFiles("", collectionID, planID)
		payloadByPlan[planID] = &payload.EngineMessage{
			Verb:      "start",
			DataFiles: make(map[string]struct{}),
			RunID:     enginesConfig[0].RunID,
		}
		p := planprogress.NewProgress(collectionID, planID)
		s.planProgress.Add(p)
		totalEngines += len(enginesConfig)
	}
	topic := fmt.Sprintf("collection:%s", collectionID)
	connectedEngines := s.pubsubServer.NumberOfClients(topic)
	if totalEngines != connectedEngines {
		http.Error(w,
			fmt.Sprintf("engine number match. connedted engines %d, required engines %d",
				connectedEngines, totalEngines), http.StatusConflict)
		return
	}
	pl, err := makeStartPayload(r, dataConfig, planStorage, pl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.pubsubServer.Broadcast(topic, pl); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// TODO shall we wait for all the plans to be running?
}

func (s *APIServer) collectionProgressHandler(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("collection_id")
	pid := r.PathValue("plan_id")
	prgs, ok := s.planProgress.Get(cid, pid)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	query := r.URL.Query()
	v := query.Get("engines")
	engines, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if prgs.Len() != int(engines) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if !prgs.IsRunning() {
		w.WriteHeader(http.StatusNotFound)
		return
	}
}

func (s *APIServer) planTerminationHandler(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("collection_id")
	pid := r.PathValue("plan_id")
	pm := make(payload.PlanMessage)
	pm[pid] = &payload.EngineMessage{}
	payload := &payload.Payload{
		PlanMessage: pm,
		Verb:        "stop",
	}
	if err := s.pubsubServer.Broadcast(fmt.Sprintf("collection:%s", cid), payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *APIServer) engineReportProgressHandler(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("collection_id")
	pid := r.PathValue("plan_id")
	eid, err := findEngineID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.ParseForm()
	running, err := strconv.ParseBool(r.Form.Get("running"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	prgs, ok := s.planProgress.Get(cid, pid)
	if !ok {
		prgs = planprogress.NewProgress(cid, pid)
	}
	engineID := int(eid)
	prgs.SetEngineStatus(engineID, running)
}

func (s *APIServer) collectionHealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	collectionID := r.PathValue("collection_id")
	topic := fmt.Sprintf("collection:%s", collectionID)
	query := r.URL.Query()
	v := query.Get("engines")
	engines, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if s.pubsubServer.NumberOfClients(topic) == int(engines) &&
		s.inventory.GetEndpointsCountByCollection(collectionID) == int(engines) {
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s *APIServer) collectionTermHandler(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("collection_id")
	q := r.URL.Query()
	plans := strings.Split(q.Get("plans"), ",")
	topic := fmt.Sprintf("collection:%s", cid)
	p := &payload.Payload{Verb: "stop"}
	p.PlanMessage = make(payload.PlanMessage, len(plans))
	for _, pid := range plans {
		p.PlanMessage[pid] = &payload.EngineMessage{}
	}
	if err := s.pubsubServer.Broadcast(topic, p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var wg sync.WaitGroup
	for pid := range p.PlanMessage {
		wg.Add(1)
		go func(pid string) {
			s.planProgress.TermPlan(cid, pid)
			wg.Done()
		}(pid)
		wg.Wait()
	}
}

func findObj(r *http.Request, key string) (int64, error) {
	t := r.PathValue(key)
	tid, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return 0, err
	}
	return tid, nil
}

func findEngineID(r *http.Request) (int64, error) {
	return findObj(r, "engine_id")
}
