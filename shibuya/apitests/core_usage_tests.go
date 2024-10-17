package apitests

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/rakutentech/shibuya/shibuya/client"
	"github.com/stretchr/testify/assert"
)

type resourceManager struct {
	endpoint string
}

func (rm *resourceManager) createProject(clientOpts *client.ClientOpts) error {
	pc := client.NewProjectClient(clientOpts)
	if err := pc.Create("test1", "shibuya"); err != nil {
		return err
	}
	return nil
}

func (rm *resourceManager) createPlan(projectID string, clientOpts *client.ClientOpts) error {
	pc := client.NewPlanClient(clientOpts)
	if err := pc.Create(projectID, "plan1"); err != nil {
		return err
	}
	planID := "1"
	file, err := os.Open("sample.jmx")
	if err != nil {
		return err
	}
	defer file.Close()
	if err := pc.UploadFile(planID, file); err != nil {
		return err
	}
	return nil
}

func (rm *resourceManager) createCollection(projectID string, clientOpts *client.ClientOpts) error {
	cc := client.NewCollectionClient(*clientOpts)
	if err := cc.Create(projectID, "collection1"); err != nil {
		return err
	}
	return nil

}

func (rm *resourceManager) prepareResources() error {
	// clientOpts := client.NewClientOpts(rm.endpoint, nil)
	// // if err := rm.createProject(clientOpts); err != nil {
	// // 	return err
	// // }
	// // if err := rm.createPlan("1", clientOpts); err != nil {
	// // 	return err
	// // }
	// if err := rm.createCollection("1", clientOpts); err != nil {
	// 	return err
	// }
	return nil
}

func TestCore(t *testing.T) {
	file, err := os.Open("1.yaml")
	assert.Nil(t, err)
	defer file.Close()
	collectionID := "1"
	endpoint := "http://localhost:8080"
	clientOpts := client.NewClientOpts(endpoint, nil)
	cc := client.NewCollectionClient(*clientOpts)
	err = cc.Configure(collectionID, file)
	assert.Nil(t, err)
	err = cc.Launch(collectionID)
	time.Sleep(20 * time.Second)
	err = cc.Trigger(collectionID)
	assert.Nil(t, err)
	stream, cancel, err := cc.Subscribe(collectionID)
	assert.Nil(t, err)
	for e := range stream.Events {
		assert.NotEmpty(t, e.Data())
		cancel()
		break
	}
	defer cc.Purge(collectionID)
}

func setup(endpoint string) error {
	rm := &resourceManager{endpoint: endpoint}
	return rm.prepareResources()
}

func TestMain(m *testing.M) {
	endpoint := "http://localhost:8080"
	if err := setup(endpoint); err != nil {
		log.Fatal(err)
	}
	r := m.Run()
	os.Exit(r)
}
