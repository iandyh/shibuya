package apitests

import (
	"fmt"
	"strings"

	"os"
	"testing"
	"time"

	"github.com/rakutentech/shibuya/shibuya/client"
	"github.com/rakutentech/shibuya/shibuya/model"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

type resourceManager struct {
	endpoint         string
	projectClient    client.ProjectClient
	collectionClient client.CollectionClient
	planClient       client.PlanClient
}

func newResourceManager(endpoint string) *resourceManager {
	clientOpts := client.NewClientOpts(endpoint, nil)
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

func (rm *resourceManager) createPlan(projectID string) (*model.Plan, error) {
	pc := rm.planClient
	plan, err := pc.Create(projectID, "plan1")
	if err != nil {
		return nil, err
	}
	file, err := os.Open("sample.jmx")
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

func (rm *resourceManager) prepareResources() (*model.Project, *model.Collection, *model.Plan, error) {
	project, err := rm.projectClient.Create("test1", "shibuya")
	if err != nil {
		return nil, nil, nil, err
	}
	projectID := fmt.Sprintf("%d", project.ID)
	plan, err := rm.createPlan(projectID)
	if err != nil {
		return project, nil, nil, err
	}
	collection, err := rm.createCollection(projectID)
	if err != nil {
		return project, nil, plan, nil
	}
	return project, collection, plan, nil
}

func TestFullAPI(t *testing.T) {
	endpoint := "http://localhost:8080"
	rm := newResourceManager(endpoint)
	project, collection, plan, err := rm.prepareResources()
	defer func() {
		if err := rm.collectionClient.Delete(collection.ID); err != nil {
			t.Fatal(err)
		}
		log.Infof("Removed collection %d", collection.ID)

		if err := rm.planClient.Delete(plan.ID); err != nil {
			t.Fatal(err)
		}
		log.Infof("Removed plan %d", plan.ID)

		if err := rm.projectClient.Delete(project.ID); err != nil {
			t.Fatal(err)
		}
		log.Infof("Removed project %d", project.ID)
	}()
	cc := rm.collectionClient
	testcases := []string{"testcasea", "testcaseb"}
	// The number of engines should be equal to the number of test cases above
	// Because in the following test, we are going to test csv_split case and we want to check
	// the data has been evently dispatched to all engines. With 2 engines, we expect each engine
	// gets 1 test case
	collectionConfigurationFile, err := rm.prepareCollectionConfiguration(project, collection, plan,
		len(testcases))
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
	// Replace sleep to collection status api call
	time.Sleep(20 * time.Second)

	err = cc.Trigger(collection.ID)
	assert.NoError(t, err)
	stream, cancel, err := cc.Subscribe(collection.ID)
	assert.Nil(t, err)
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
			cancel()
			break
		}
	}
	cc.Purge(collection.ID)
	time.Sleep(5 * time.Second)
}

func TestObjectCRUD(t *testing.T) {
	endpoint := "http://localhost:8080"
	rm := newResourceManager(endpoint)
	project, collection, plan, err := rm.prepareResources()
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
	rm := newResourceManager(endpoint)
	invalidID := int64(100000000)
	_, err := rm.projectClient.Get(invalidID)
	assert.Error(t, err)
	_, err = rm.collectionClient.Get(invalidID)
	assert.Error(t, err)
	_, err = rm.planClient.Get(invalidID)
	assert.Error(t, err)
}
