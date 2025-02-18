package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	payload "github.com/rakutentech/shibuya/shibuya/coordinator/payload"
	"github.com/rakutentech/shibuya/shibuya/coordinator/storage"
	"github.com/rakutentech/shibuya/shibuya/coordinator/upstream"
	httptoken "github.com/rakutentech/shibuya/shibuya/http/auth/token"
	httproute "github.com/rakutentech/shibuya/shibuya/http/route"

	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
	pubsub "github.com/reqfleet/pubsub/server"
)

type APIServer struct {
	// client used for engine progress check
	httpClient   *http.Client
	apiKey       string
	pubsubServer *pubsub.PubSubServer
	inventory    *upstream.Inventory
}

func NewAPIServer(server *pubsub.PubSubServer, inventory *upstream.Inventory, apiKey string) *APIServer {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	s := &APIServer{pubsubServer: server, inventory: inventory, apiKey: apiKey, httpClient: client}
	return s
}

func engineProgress(endpoint, apiKey string, httpClient *http.Client) bool {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/progress", endpoint), nil)
	if err != nil {
		return false
	}
	req.Header.Set(httptoken.AuthHeader, fmt.Sprintf("%s %s", httptoken.BEARER_PREFIX, apiKey))
	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	switch resp.StatusCode {
	case http.StatusNoContent:
		return true
	}
	return false
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
}

func (s *APIServer) collectionProgressHandler(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("collection_id")
	pid := r.PathValue("plan_id")
	endpointsByPlan := s.inventory.GetPlanEndpoints(cid, pid)
	results := make(chan bool, len(endpointsByPlan))
	var wg sync.WaitGroup
	for _, ep := range endpointsByPlan {
		wg.Add(1)
		go func(ep string) {
			defer wg.Done()
			results <- engineProgress(ep, s.apiKey, s.httpClient)
		}(ep)
	}
	wg.Wait()
	running := true
	for i := 0; i < len(endpointsByPlan); i++ {
		running = running && <-results
	}
	if running {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	close(results)
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
