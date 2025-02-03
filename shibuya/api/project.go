package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/rakutentech/shibuya/shibuya/config"
	httproute "github.com/rakutentech/shibuya/shibuya/http/route"
	"github.com/rakutentech/shibuya/shibuya/model"
)

type ProjectAPI struct {
	sc config.ShibuyaConfig
}

func NewProjectAPI(sc config.ShibuyaConfig) *ProjectAPI {
	pa := &ProjectAPI{
		sc: sc,
	}
	return pa
}

func (pa *ProjectAPI) Router() *httproute.Router {
	router := httproute.NewRouter("project api", "projects")
	router.AddRoutes(httproute.Routes{
		{
			Name:        "Get all projects",
			Method:      "GET",
			HandlerFunc: pa.projectsGetHandler,
		},
		{
			Name:        "Create a project",
			Method:      "POST",
			HandlerFunc: pa.projectCreateHandler,
		},
		{
			Name:        "Get a project",
			Method:      "GET",
			Path:        "{project_id}",
			HandlerFunc: pa.projectGetHandler,
		},
		{
			Name:        "Update a project",
			Method:      "PUT",
			HandlerFunc: pa.projectUpdateHandler,
		},
		{
			Name:        "Delete a project",
			Method:      "DELETE",
			Path:        "{project_id}",
			HandlerFunc: pa.projectDeleteHandler,
		},
	})
	return router
}

func getProject(projectID string) (*model.Project, error) {
	if projectID == "" {
		return nil, makeInvalidRequestError("project_id cannot be empty")
	}
	pid, err := strconv.Atoi(projectID)
	if err != nil {
		return nil, makeInvalidResourceError("project_id")
	}
	project, err := model.GetProject(int64(pid))
	if err != nil {
		return nil, err
	}
	return project, nil
}

func (pa *ProjectAPI) projectCreateHandler(w http.ResponseWriter, r *http.Request) {
	account := r.Context().Value(accountKey).(*model.Account)
	r.ParseForm()
	name := r.Form.Get("name")
	if name == "" {
		handleErrors(w, makeInvalidRequestError("Project name cannot be empty"))
		return
	}
	owner := r.Form.Get("owner")
	if owner == "" {
		handleErrors(w, makeInvalidRequestError("Owner name cannot be empty"))
		return
	}
	if _, ok := account.MLMap[owner]; !ok {
		handleErrors(w, makeNoPermissionErr(fmt.Sprintf("You are not part of %s", owner)))
		return
	}
	var sid string
	sc := pa.sc
	if sc.EnableSid {
		sid = r.Form.Get("sid")
		if sid == "" {
			handleErrors(w, makeInvalidRequestError("SID cannot be empty"))
			return
		}
		if _, err := strconv.Atoi(sid); err != nil {
			handleErrors(w, makeInvalidRequestError("SID is invalid"))
			return
		}
	}
	projectID, err := model.CreateProject(name, owner, sid)
	if err != nil {
		handleErrors(w, err)
		return
	}
	project, err := model.GetProject(projectID)
	if err != nil {
		handleErrors(w, err)
		return
	}
	renderJSON(w, http.StatusOK, project)
}

func (pa *ProjectAPI) projectGetHandler(w http.ResponseWriter, r *http.Request) {
	project, err := getProject(r.PathValue("project_id"))
	if err != nil {
		handleErrors(w, err)
		return
	}
	renderJSON(w, http.StatusOK, project)
}

func (pa *ProjectAPI) projectUpdateHandler(w http.ResponseWriter, _ *http.Request) {
	renderJSON(w, http.StatusNotImplemented, nil)
}

func (pa *ProjectAPI) projectDeleteHandler(w http.ResponseWriter, r *http.Request) {
	account := r.Context().Value(accountKey).(*model.Account)
	project, err := getProject(r.PathValue("project_id"))
	if err != nil {
		handleErrors(w, err)
		return
	}
	if r := hasProjectOwnership(project, account, pa.sc.AuthConfig); !r {
		handleErrors(w, makeProjectOwnershipError())
		return
	}
	collectionIDs, err := project.GetCollections()
	if err != nil {
		handleErrors(w, err)
		return
	}
	if len(collectionIDs) > 0 {
		handleErrors(w, makeInvalidRequestError("You cannot delete a project that has collections"))
		return
	}
	planIDs, err := project.GetPlans()
	if err != nil {
		handleErrors(w, err)
		return
	}
	if len(planIDs) > 0 {
		handleErrors(w, makeInvalidRequestError("You cannot delete a project that has plans"))
		return
	}
	project.Delete()
}

func (pa *ProjectAPI) projectsGetHandler(w http.ResponseWriter, r *http.Request) {
	account := r.Context().Value(accountKey).(*model.Account)
	qs := r.URL.Query()
	var includeCollections, includePlans bool
	var err error

	includeCollectionsList := qs["include_collections"]
	includePlansList := qs["include_plans"]
	if len(includeCollectionsList) > 0 {
		if includeCollections, err = strconv.ParseBool(includeCollectionsList[0]); err != nil {
			includeCollections = false
		}
	} else {
		includeCollections = false
	}

	if len(includePlansList) > 0 {
		if includePlans, err = strconv.ParseBool(includePlansList[0]); err != nil {
			includePlans = false
		}
	} else {
		includePlans = false
	}
	projects, _ := model.GetProjectsByOwners(account.ML)
	if !includeCollections && !includePlans {
		renderJSON(w, http.StatusOK, projects)
		return
	}
	for _, p := range projects {
		if includeCollections {
			p.Collections, _ = p.GetCollections()
		}
		if includePlans {
			p.Plans, _ = p.GetPlans()
		}
	}
	renderJSON(w, http.StatusOK, projects)
}
