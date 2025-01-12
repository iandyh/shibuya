package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/controller"
	"github.com/rakutentech/shibuya/shibuya/object_storage"
)

type ShibuyaAPI struct {
	sc         config.ShibuyaConfig
	objStorage object_storage.StorageInterface
	ctr        *controller.Controller
}

type APIhandler interface {
	collectRoutes() Routes
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

func (s *ShibuyaAPI) InitRoutes() Routes {
	routes := make(Routes, 0)
	projectAPI := NewProjectAPI(s.sc)
	planAPI := NewPlanAPI(s.sc, s.objStorage)
	collectionAPI := NewCollectionAPI(s.sc, s.objStorage, s.ctr)
	fileAPI := NewFileAPI(s.objStorage)
	usageAPI := NewUsageAPI()
	adminAPI := NewAdminAPI(s.sc.Context)
	apiHandlers := []APIhandler{
		projectAPI,
		planAPI,
		collectionAPI,
		fileAPI,
		usageAPI,
		adminAPI,
	}
	for _, h := range apiHandlers {
		routes = append(routes, h.collectRoutes()...)
	}
	for _, r := range routes {
		// TODO! We don't require auth for usage endpoint for now.
		if strings.Contains(r.Path, "usage") {
			continue
		}
		r.HandlerFunc = authRequired(r.HandlerFunc, s.sc.AuthConfig)
	}
	return routes
}
