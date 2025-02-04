package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/controller"
	httproute "github.com/rakutentech/shibuya/shibuya/http/route"
	"github.com/rakutentech/shibuya/shibuya/object_storage"
)

type ShibuyaAPI struct {
	sc         config.ShibuyaConfig
	objStorage object_storage.StorageInterface
	ctr        *controller.Controller
}

type ShibuyaAPIComponent interface {
	Router() *httproute.Router
}

func renderJSON(w http.ResponseWriter, status int, content interface{}) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(content)
}

func NewAPIServer(sc config.ShibuyaConfig) *ShibuyaAPI {
	c := &ShibuyaAPI{
		ctr:        controller.NewController(sc),
		objStorage: object_storage.CreateObjStorageClient(sc),
		sc:         sc,
	}
	c.ctr.StartRunning()
	return c
}

func (s *ShibuyaAPI) Router() *httproute.Router {
	projectAPI := NewProjectAPI(s.sc)
	planAPI := NewPlanAPI(s.sc, s.objStorage)
	collectionAPI := NewCollectionAPI(s.sc, s.objStorage, s.ctr)
	fileAPI := NewFileAPI(s.objStorage)
	usageAPI := NewUsageAPI()
	adminAPI := NewAdminAPI(s.sc.Context)
	apiComponents := []ShibuyaAPIComponent{
		projectAPI,
		planAPI,
		collectionAPI,
		fileAPI,
		usageAPI,
		adminAPI,
	}
	apiRouter := httproute.NewRouter("api router", "/api")
	for _, ac := range apiComponents {
		apiRouter.Mount(ac.Router())
	}
	for _, r := range apiRouter.GetRoutes() {
		// TODO! We don't require auth for usage endpoint for now.
		if strings.Contains(r.Path, "usage") {
			continue
		}
		r.HandlerFunc = authRequired(r.HandlerFunc, s.sc.AuthConfig)
	}
	return apiRouter
}
