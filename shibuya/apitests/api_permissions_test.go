package apitests

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/gavv/httpexpect/v2"
	"github.com/rakutentech/shibuya/shibuya/api"
	"github.com/rakutentech/shibuya/shibuya/client"
	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/model"
	"github.com/stretchr/testify/assert"
)

func testAllEndpoints(e *httpexpect.Expect, expectedStatusCode int) {
	sc := config.ShibuyaConfig{}
	router := api.MakeRouter(sc, nil, nil)
	for _, r := range router.GetRoutes() {
		switch r.Method {
		case http.MethodGet:
			e.GET(r.Path).Expect().Status(expectedStatusCode)
		case http.MethodPut:
			e.PUT(r.Path).Expect().Status(expectedStatusCode)
		case http.MethodDelete:
			e.DELETE(r.Path).Expect().Status(expectedStatusCode)
		case http.MethodPost:
			e.POST(r.Path).Expect().Status(expectedStatusCode)
		}
	}
}

func testOwnershipEndpoints(t *testing.T, e *httpexpect.Expect, projectID, collectionID, planID string, expectedStatusCode int) {
	sc := config.ShibuyaConfig{}
	router := api.MakeRouter(sc, nil, nil)
	n := 0
	for _, r := range router.GetRoutes() {
		if !(strings.HasPrefix(r.Path, "/api/projects") || strings.HasPrefix(r.Path, "/api/collections") || strings.HasPrefix(r.Path, "/api/plans")) {
			log.Printf("Skip path %s since we now only testing resources related apis", r.Path)
			continue
		}
		if strings.Contains(r.Path, "runs") {
			continue
		}
		req := &httpexpect.Request{}
		switch r.Method {
		case http.MethodGet:
			if r.Path == "/api/projects" {
				continue
			}
			req = e.GET(r.Path)
		case http.MethodPut:
			req = e.PUT(r.Path)
			if strings.HasSuffix(r.Path, "{collection_id}") || strings.HasSuffix(r.Path, "{plan_id}") || strings.HasSuffix(r.Path, "{project_id}") {
				log.Printf("Skip path %s since it's not being implemented", r.Path)
				continue
			}
		case http.MethodDelete:
			req = e.DELETE(r.Path)
		case http.MethodPost:
			req = e.POST(r.Path)
			if strings.HasSuffix(r.Path, "collections") || strings.HasSuffix(r.Path, "plans") || strings.HasSuffix(r.Path, "projects") {
				log.Printf("Skip path %s because creation requests do not need ownership check", r.Path)
				continue
			}
		}
		if strings.Contains(r.Path, "{project_id}") {
			req.WithPath("project_id", projectID)
		}
		if strings.Contains(r.Path, "{collection_id}") {
			req.WithPath("collection_id", collectionID)
		}
		if strings.Contains(r.Path, "{plan_id}") {
			req.WithPath("plan_id", planID)
		}
		n += 1
		req.Expect().Status(expectedStatusCode)
	}
	t.Logf("Tested paths: %d; total paths: %d", n, len(router.GetRoutes()))
}

func TestAPIWithoutSession(t *testing.T) {
	endpoint := "http://localhost:8080"
	e := httpexpect.Default(t, endpoint)
	testAllEndpoints(e, http.StatusUnauthorized)
}

func TestAPIWithoutOwnership(t *testing.T) {
	endpoint := "http://localhost:8080"
	token, err := fetchToken(endpoint, "shibuya")
	assert.Nil(t, err)
	clientOpts := client.NewClientOpts(endpoint, token, nil)
	projectClient := client.NewProjectClient(clientOpts)
	collectionClient := client.NewCollectionClient(*clientOpts)
	planClient := client.NewPlanClient(clientOpts)
	project, err := projectClient.Create("test-project", "shibuya")
	assert.Nil(t, err)
	projectID := strconv.Itoa(int(project.ID))
	collection, err := collectionClient.Create(projectID, "test-c")
	assert.Nil(t, err)
	plan, err := planClient.Create(projectID, "test-p", model.JmeterPlan)
	assert.Nil(t, err)

	anotherToken, err := fetchToken(endpoint, "test-user")
	assert.Nil(t, err)

	e := httpexpect.Default(t, endpoint)
	auth := e.Builder(func(req *httpexpect.Request) {
		req.WithHeader("Authorization", "Bearer "+anotherToken)
	})
	collectionID := strconv.Itoa(int(collection.ID))
	planID := strconv.Itoa(int(plan.ID))
	testOwnershipEndpoints(t, auth, projectID, collectionID, planID, http.StatusForbidden)
}
