package api

import (
	"net/http"

	httproute "github.com/rakutentech/shibuya/shibuya/http/route"
	"github.com/rakutentech/shibuya/shibuya/model"
	smodel "github.com/rakutentech/shibuya/shibuya/scheduler/model"
)

type AdminAPI struct {
	ctx string
}

func NewAdminAPI(ctx string) *AdminAPI {
	aa := &AdminAPI{
		ctx: ctx,
	}
	return aa
}

type AdminCollectionResponse struct {
	RunningCollections []*model.RunningPlan `json:"running_collections"`
	NodePools          smodel.AllNodesInfo  `json:"node_pools"`
}

func (aa *AdminAPI) Router() *httproute.Router {
	router := httproute.NewRouter("admin", "admin")
	routes := httproute.Routes{
		{
			Name:        "Get running collections by admin",
			Method:      "GET",
			Path:        "collections",
			HandlerFunc: aa.collectionAdminGetHandler,
		},
	}
	router.AddRoutes(routes)
	return router
}

func (aa *AdminAPI) collectionAdminGetHandler(w http.ResponseWriter, r *http.Request) {
	collections, err := model.GetRunningCollections(aa.ctx)
	if err != nil {
		handleErrors(w, err)
		return
	}
	acr := new(AdminCollectionResponse)
	acr.RunningCollections = collections
	renderJSON(w, http.StatusOK, acr)
}
