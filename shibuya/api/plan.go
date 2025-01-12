package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/model"
	"github.com/rakutentech/shibuya/shibuya/object_storage"
)

type PlanAPI struct {
	PathHandler
	objStorage object_storage.StorageInterface
	sc         config.ShibuyaConfig
}

func NewPlanAPI(sc config.ShibuyaConfig, objStorage object_storage.StorageInterface) *PlanAPI {
	return &PlanAPI{
		PathHandler: PathHandler{
			Path: "/api/plans",
		},
		objStorage: objStorage,
		sc:         sc,
	}
}

func (pa *PlanAPI) collectRoutes() Routes {
	return Routes{
		{
			Name:        "Create a plan",
			Method:      "POST",
			Path:        pa.Path,
			HandlerFunc: pa.planCreateHandler,
		},
		{
			Name:        "Get a plan",
			Method:      "GET",
			Path:        fmt.Sprintf("%s/{plan_id}", pa.Path),
			HandlerFunc: pa.planGetHandler,
		},
		{
			Name:        "Update a plan",
			Method:      "PUT",
			Path:        pa.Path,
			HandlerFunc: pa.planUpdateHandler,
		},
		{
			Name:        "Delete a plan",
			Method:      "DELETE",
			Path:        pa.Path,
			HandlerFunc: pa.planDeleteHandler,
		},
		{
			Name:        "Get a plan files",
			Method:      "GET",
			Path:        fmt.Sprintf("%s/{plan_id}/files", pa.Path),
			HandlerFunc: pa.planFilesGetHandler,
		},
		{
			Name:        "Upload files in a plan",
			Method:      "PUT",
			Path:        fmt.Sprintf("%s/{plan_id}/files", pa.Path),
			HandlerFunc: pa.planFilesUploadHandler,
		},
		{
			Name:        "Delete files in a plan",
			Method:      "DELETE",
			Path:        fmt.Sprintf("%s/{plan_id}/files", pa.Path),
			HandlerFunc: pa.planFilesDeleteHandler,
		},
	}
}

func getPlan(planID string) (*model.Plan, error) {
	pid, err := strconv.Atoi(planID)
	if err != nil {
		return nil, makeInvalidResourceError("plan_id")
	}
	plan, err := model.GetPlan(int64(pid))
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func (pa *PlanAPI) planCreateHandler(w http.ResponseWriter, r *http.Request) {
	account := r.Context().Value(accountKey).(*model.Account)
	r.ParseForm()
	projectID := r.Form.Get("project_id")
	project, err := getProject(projectID)
	if err != nil {
		handleErrors(w, err)
		return
	}
	if r := hasProjectOwnership(project, account, pa.sc.AuthConfig); !r {
		handleErrors(w, makeProjectOwnershipError())
		return
	}
	name := r.Form.Get("name")
	if name == "" {
		handleErrors(w, makeInvalidRequestError("plan name cannot be empty"))
		return
	}
	planID, err := model.CreatePlan(name, project.ID)
	if err != nil {
		handleErrors(w, err)
		return
	}
	plan, err := model.GetPlan(planID)
	if err != nil {
		handleErrors(w, err)
	}
	renderJSON(w, http.StatusOK, plan)
}

func (pa *PlanAPI) planGetHandler(w http.ResponseWriter, r *http.Request) {
	plan, err := getPlan(r.PathValue("plan_id"))
	if err != nil {
		handleErrors(w, err)
		return
	}
	renderJSON(w, http.StatusOK, plan)
}

func (pa *PlanAPI) planUpdateHandler(w http.ResponseWriter, _ *http.Request) {
	renderJSON(w, http.StatusNotImplemented, nil)
}

func (pa *PlanAPI) planDeleteHandler(w http.ResponseWriter, r *http.Request) {
	account := r.Context().Value(accountKey).(*model.Account)
	plan, err := getPlan(r.PathValue("plan_id"))
	if err != nil {
		handleErrors(w, err)
		return
	}
	project, err := model.GetProject(plan.ProjectID)
	if err != nil {
		handleErrors(w, err)
		return
	}
	if r := hasProjectOwnership(project, account, pa.sc.AuthConfig); !r {
		handleErrors(w, makeProjectOwnershipError())
		return
	}
	using, err := plan.IsBeingUsed()
	if err != nil {
		handleErrors(w, err)
		return
	}
	if using {
		handleErrors(w, makeInvalidRequestError("plan is being used"))
		return
	}
	plan.Delete(pa.objStorage)
}

func (pa *PlanAPI) planFilesUploadHandler(w http.ResponseWriter, r *http.Request) {
	plan, err := getPlan(r.PathValue("plan_id"))
	if err != nil {
		handleErrors(w, err)
		return
	}
	r.ParseMultipartForm(100 << 20) //parse 100 MB of data
	file, handler, err := r.FormFile("planFile")
	if err != nil {
		handleErrors(w, makeInvalidRequestError("Something wrong with file you uploaded"))
		return
	}
	err = plan.StoreFile(pa.objStorage, file, handler.Filename)
	if err != nil {
		// TODO need to handle the upload error here
		handleErrors(w, err)
		return
	}
	w.Write([]byte("success"))
}

func (pa *PlanAPI) planFilesGetHandler(w http.ResponseWriter, _ *http.Request) {
	renderJSON(w, http.StatusNotImplemented, nil)
}

func (pa *PlanAPI) planFilesDeleteHandler(w http.ResponseWriter, r *http.Request) {
	plan, err := getPlan(r.PathValue("plan_id"))
	if err != nil {
		handleErrors(w, err)
		return
	}
	r.ParseForm()
	filename := r.Form.Get("filename")
	if filename == "" {
		handleErrors(w, makeInvalidRequestError("plan file name cannot be empty"))
		return
	}
	err = plan.DeleteFile(pa.objStorage, filename)
	if err != nil {
		handleErrors(w, makeInternalServerError("Deletetion was unsuccessful"))
		return
	}
	w.Write([]byte("Deleted successfully"))
}
