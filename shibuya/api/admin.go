package api

import (
	"fmt"
	"net/http"

	"github.com/rakutentech/shibuya/shibuya/model"
	smodel "github.com/rakutentech/shibuya/shibuya/scheduler/model"
)

type AdminAPI struct {
	PathHandler
	ctx string
}

func NewAdminAPI(ctx string) *AdminAPI {
	return &AdminAPI{
		PathHandler: PathHandler{
			Path: "/api/admin",
		},
		ctx: ctx,
	}
}

type AdminCollectionResponse struct {
	RunningCollections []*model.RunningPlan `json:"running_collections"`
	NodePools          smodel.AllNodesInfo  `json:"node_pools"`
}

func (aa *AdminAPI) collectRoutes() Routes {
	return Routes{
		{
			Name:        "Get running collections by admin",
			Method:      "GET",
			Path:        fmt.Sprintf("%s/collections", aa.Path),
			HandlerFunc: aa.collectionAdminGetHandler,
		},
	}
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
