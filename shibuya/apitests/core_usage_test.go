package apitests

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"os"
	"testing"
	"time"

	es "github.com/iandyh/eventsource"
	"github.com/rakutentech/shibuya/shibuya/client"
	authtoken "github.com/rakutentech/shibuya/shibuya/http/auth/token"
	"github.com/rakutentech/shibuya/shibuya/model"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

type resourceManager struct {
	endpoint         string
	projectClient    *client.ProjectClient
	collectionClient *client.CollectionClient
	planClient       *client.PlanClient
}

func fetchToken(endpoint string) (string, error) {
	form := url.Values{}
	form.Add("username", "shibuya")
	form.Add("password", "test")
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/login", endpoint), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Prevent auto-following redirects
		},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	for _, cookie := range resp.Cookies() {
		if cookie.Name == authtoken.CookieName {
			return cookie.Value, nil
		}
	}
	return "", errors.New("Cannot find the cookie")
}

func newResourceManager(endpoint, url string) *resourceManager {
	token, err := fetchToken(url)
	if err != nil {
		log.Fatal(err)
	}
	clientOpts := client.NewClientOpts(endpoint, token, nil)
	return &resourceManager{
		endpoint:         endpoint,
		projectClient:    client.NewProjectClient(clientOpts),
		collectionClient: client.NewCollectionClient(*clientOpts),
		planClient:       client.NewPlanClient(clientOpts),
	}
}

func (rm *resourceManager) prepareCollectionConfiguration(project *model.Project,
	collection *model.Collection, plan *model.Plan, engineNo int) (*os.File, error) {
	ew := &model.ExecutionWrapper{
		Content: &model.ExecutionCollection{
			Name:         collection.Name,
			ProjectID:    project.ID,
			CollectionID: collection.ID,
			CSVSplit:     true,
			Tests: []*model.ExecutionPlan{
				{
					Name:        plan.Name,
					Duration:    5,
					Concurrency: 1,
					Rampup:      0,
					PlanID:      plan.ID,
					Engines:     engineNo,
					CSVSplit:    true,
				},
			},
		},
	}
	file, err := os.CreateTemp("", "collection-*.yaml")
	if err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(ew)
	if err != nil {
		file.Close()
		return nil, err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return nil, err
	}
	log.Infof("Created collection configuration file %s", file.Name())
	return file, nil
}
func (rm *resourceManager) createProject() (*model.Project, error) {
	project, err := rm.projectClient.Create("test1", "shibuya")
	if err != nil {
		return nil, err
	}
	log.Infof("Created project %d", project.ID)
	return project, nil
}

func (rm *resourceManager) createPlan(projectID string, kind model.PlanKind, planFile string) (*model.Plan, error) {
	pc := rm.planClient
	plan, err := pc.Create(projectID, "plan1", kind)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(planFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if err := pc.UploadFile(plan.ID, file); err != nil {
		return nil, err
	}
	log.Infof("Created plan %d", plan.ID)
	return plan, nil
}

func (rm *resourceManager) createCollection(projectID string) (*model.Collection, error) {
	collection, err := rm.collectionClient.Create(projectID, "collection1")
	if err != nil {
		return nil, err
	}
	log.Infof("Created collection %d", collection.ID)
	return collection, nil
}

func (rm *resourceManager) prepareResources(project *model.Project, kind model.PlanKind, planFile string) (*model.Collection, *model.Plan, error) {
	projectID := fmt.Sprintf("%d", project.ID)
	plan, err := rm.createPlan(projectID, kind, planFile)
	if err != nil {
		return nil, nil, err
	}
	collection, err := rm.createCollection(projectID)
	if err != nil {
		return nil, plan, nil
	}
	return collection, plan, nil
}

// In this test, we create a project, collection and a plan first
// Then we configure two engines for a plan and share some common data among them
// Trigger the test and we check whether the data is being equally shared in the 2 engines(each should have 1)
func TestFullAPI(t *testing.T) {
	endpoint := "http://localhost:8080"
	rm := newResourceManager(endpoint, endpoint)
	project, err := rm.createProject()
	assert.Nil(t, err)

	checkMetricsFunc := func(stream *es.Stream, testcases []string, t *testing.T) {
		notSeen := make(map[string]struct{})
		for _, testcase := range testcases {
			notSeen[testcase] = struct{}{}
		}
		for e := range stream.Events {
			assert.NotEmpty(t, e.Data())
			for _, testcase := range testcases {
				if strings.Contains(e.Data(), testcase) {
					delete(notSeen, testcase)
					break
				}
			}
			if len(notSeen) == 0 {
				return
			}
		}
	}
	testsByPlanKind := []struct {
		kind      model.PlanKind
		planFile  string
		testcases []string
	}{
		{
			kind:      model.JmeterPlan,
			planFile:  "sample.jmx",
			testcases: []string{"testcasea", "testcaseb"},
		},
		{
			kind:      model.LocustPlan,
			planFile:  "locustfile.py",
			testcases: []string{"plan1"},
		},
	}
	for _, tc := range testsByPlanKind {
		t.Run(string(tc.kind), func(t *testing.T) {
			collection, plan, err := rm.prepareResources(project, tc.kind, tc.planFile)
			assert.Nil(t, err)
			defer func() {
				if err := rm.collectionClient.Delete(collection.ID); err != nil {
					t.Fatal(err)
				}
				log.Infof("Removed collection %d", collection.ID)

				if err := rm.planClient.Delete(plan.ID); err != nil {
					t.Fatal(err)
				}
				log.Infof("Removed plan %d", plan.ID)
			}()
			cc := rm.collectionClient
			// The number of engines should be equal to the number of test cases above
			// Because in the following test, we are going to test csv_split case and we want to check
			// the data has been evently dispatched to all engines. With 2 engines, we expect each engine
			// gets 1 test case
			collectionConfigurationFile, err := rm.prepareCollectionConfiguration(project, collection, plan,
				2)
			assert.Nil(t, err)
			defer os.Remove(collectionConfigurationFile.Name())

			content, err := os.Open(collectionConfigurationFile.Name())
			err = cc.Configure(collection.ID, content)
			assert.NoError(t, err)

			dataFile, err := os.Open("testcases.csv")
			assert.Nil(t, err)
			err = cc.UploadFile(collection.ID, dataFile)
			assert.Nil(t, err)
			err = cc.Launch(collection.ID)
			triggerable := false
			timeout := time.Duration(20 * time.Second)
		waitLoop:
			for {
				select {
				case <-time.After(timeout):
					break waitLoop
				default:
					time.Sleep(1 * time.Second)
					cs, err := cc.Status(collection.ID)
					if err != nil {
						continue waitLoop
					}
					if cs.CanBeTriggered() {
						triggerable = true
						break waitLoop
					}
				}
			}
			if !triggerable {
				t.Fatalf("Engines could not be ready after %v", timeout)
			}
			err = cc.Trigger(collection.ID)
			assert.NoError(t, err)
			stream, cancel, err := cc.Subscribe(collection.ID)
			assert.Nil(t, err)
			checkMetricsFunc(stream, tc.testcases, t)
			cancel()
			cc.Purge(collection.ID)
			time.Sleep(5 * time.Second)
		})
		defer func() {
			if err := rm.projectClient.Delete(project.ID); err != nil {
				t.Fatal(err)
			}
			log.Infof("Removed project %d", project.ID)
		}()
	}
}

func TestObjectCRUD(t *testing.T) {
	endpoint := "http://localhost:8080"
	rm := newResourceManager(endpoint, endpoint)
	project, err := rm.createProject()
	assert.Nil(t, err)
	collection, plan, err := rm.prepareResources(project, model.JmeterPlan, "sample.jmx")
	defer func() {
		rm.collectionClient.Delete(collection.ID)
		rm.planClient.Delete(plan.ID)
		rm.projectClient.Delete(project.ID)
	}()
	assert.NoError(t, err)
	pproject, err := rm.projectClient.Get(project.ID)
	assert.Nil(t, err)
	assert.Equal(t, project.ID, pproject.ID)
	ccollection, err := rm.collectionClient.Get(collection.ID)
	assert.Nil(t, err)
	assert.Equal(t, collection.ID, ccollection.ID)

	pplan, err := rm.planClient.Get(plan.ID)
	assert.Nil(t, err)
	assert.Equal(t, plan.ID, pplan.ID)
}

func TestObjectCRUDWithErrors(t *testing.T) {
	endpoint := "http://localhost:8080"
	rm := newResourceManager(endpoint, endpoint)
	invalidID := int64(100000000)
	_, err := rm.projectClient.Get(invalidID)
	assert.Error(t, err)
	_, err = rm.collectionClient.Get(invalidID)
	assert.Error(t, err)
	_, err = rm.planClient.Get(invalidID)
	assert.Error(t, err)
}
