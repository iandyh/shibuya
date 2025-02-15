package api

import (
	"net/http"
	"strconv"

	"github.com/go-sql-driver/mysql"
	"github.com/rakutentech/shibuya/shibuya/config"
	httproute "github.com/rakutentech/shibuya/shibuya/http/route"
	"github.com/rakutentech/shibuya/shibuya/model"
	"github.com/rakutentech/shibuya/shibuya/object_storage"
)

type PlanAPI struct {
	objStorage object_storage.StorageInterface
	sc         config.ShibuyaConfig
}

func NewPlanAPI(sc config.ShibuyaConfig, objStorage object_storage.StorageInterface) *PlanAPI {
	pa := &PlanAPI{
		objStorage: objStorage,
		sc:         sc,
	}
	return pa
}

func (pa *PlanAPI) Router() *httproute.Router {
	router := httproute.NewRouter("plan api", "/plans")
	router.AddRoutes(httproute.Routes{
		{
			Name:        "Create a plan",
			Method:      "POST",
			HandlerFunc: pa.planCreateHandler,
		},
		{
			Name:        "Get a plan",
			Method:      "GET",
			Path:        "{plan_id}",
			HandlerFunc: pa.planGetHandler,
		},
		{
			Name:        "Update a plan",
			Method:      "PUT",
			HandlerFunc: pa.planUpdateHandler,
		},
		{
			Name:        "Delete a plan",
			Method:      "DELETE",
			Path:        "{plan_id}",
			HandlerFunc: pa.planDeleteHandler,
		},
		{
			Name:        "Get a plan files",
			Method:      "GET",
			Path:        "{plan_id}/files",
			HandlerFunc: pa.planFilesGetHandler,
		},
		{
			Name:        "Upload files in a plan",
			Method:      "PUT",
			Path:        "{plan_id}/files",
			HandlerFunc: pa.planFilesUploadHandler,
		},
		{
			Name:        "Delete files in a plan",
			Method:      "DELETE",
			Path:        "{plan_id}/files",
			HandlerFunc: pa.planFilesDeleteHandler,
		},
	})
	return router
}

func getPlan(r *http.Request, authConfig *config.AuthConfig) (*model.Plan, error) {
	planID := r.PathValue("plan_id")
	pid, err := strconv.Atoi(planID)
	if err != nil {
		return nil, makeInvalidResourceError("plan_id")
	}
	plan, err := model.GetPlan(int64(pid))
	if err != nil {
		return nil, err
	}
	account := r.Context().Value(accountKey).(*model.Account)
	if _, err := getProject(plan.ProjectID, account, authConfig); err != nil {
		return nil, makePlanOwnershipError()
	}
	return plan, nil
}

func (pa *PlanAPI) planCreateHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	project, err := getProjectFromForm(r, pa.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
		return
	}
	name := r.Form.Get("name")
	if name == "" {
		handleErrors(w, makeInvalidRequestError("plan name cannot be empty"))
		return
	}
	kind := r.Form.Get("kind")
	if kind == "" {
		handleErrors(w, makeInvalidRequestError("kind cannot be empty"))
		return
	}
	pk := model.PlanKind(kind)
	if !pk.IsSupported() {
		handleErrors(w, makeInvalidRequestError("invalid kind"))
		return
	}
	planID, err := model.CreatePlan(name, project.ID, pk)
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
	plan, err := getPlan(r, pa.sc.AuthConfig)
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
	plan, err := getPlan(r, pa.sc.AuthConfig)
	if err != nil {
		handleErrors(w, err)
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
	plan, err := getPlan(r, pa.sc.AuthConfig)
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
	if plan.IsTestFile(handler.Filename) && !plan.IsThePlanFileValid(handler.Filename) {
		handleErrors(w, makeInvalidRequestError("Wrong file for the plan"))
		return
	}
	err = plan.StoreFile(pa.objStorage, file, handler.Filename)
	if err != nil {
		// TODO need to handle the upload error here
		if driverErr, ok := err.(*mysql.MySQLError); ok {
			if driverErr.Number == 1062 {
				handleErrors(w, makeInvalidRequestError("File already exists. If you wish to update it then delete existing one and upload again."))
				return
			}
		}
		handleErrors(w, makeInternalServerError(err.Error()))
		return
	}
	w.Write([]byte("success"))
}

func (pa *PlanAPI) planFilesGetHandler(w http.ResponseWriter, _ *http.Request) {
	renderJSON(w, http.StatusNotImplemented, nil)
}

func (pa *PlanAPI) planFilesDeleteHandler(w http.ResponseWriter, r *http.Request) {
	plan, err := getPlan(r, pa.sc.AuthConfig)
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
