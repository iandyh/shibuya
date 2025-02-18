package model

import (
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestSupportedKinds(t *testing.T) {
	k := PlanKind("asdf")
	assert.False(t, k.IsSupported())

	k = JmeterPlan
	assert.True(t, k.IsSupported())

	k = LocustPlan
	assert.True(t, k.IsSupported())
}

func TestExtensions(t *testing.T) {
	assert.True(t, len(ValidExtensions) == len(TestFileExtensions))
	for _, item := range TestFileExtensions {
		t.Log(item)
	}
}

func TestCreateAndGetPlan(t *testing.T) {
	name := "testplan"
	projectID := int64(1)
	planID, err := CreatePlan(name, projectID, LocustPlan)
	if err != nil {
		t.Fatal(err)
	}
	p, err := GetPlan(planID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, name, p.Name)
	assert.Equal(t, LocustPlan, p.Kind)
	assert.Equal(t, projectID, p.ProjectID)
	assert.True(t, p.IsThePlanFileValid("asdf.py"))
	assert.False(t, p.IsThePlanFileValid("asdf.jmx"))

	p.Delete(nil)
	p, err = GetPlan(planID)
	assert.NotNil(t, err)
	assert.Nil(t, p)
}

func TestGetRunningPlans(t *testing.T) {
	collectionID := int64(1)
	planID := int64(1)
	ctx := "test"
	if err := AddRunningPlan(ctx, collectionID, planID); err != nil {
		t.Fatal(err)
	}
	rp, err := GetRunningPlan(collectionID, planID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, rp.PlanID, planID)
	assert.Equal(t, rp.CollectionID, collectionID)
	assert.NotNil(t, rp.StartedTime)
	rps, err := GetRunningPlans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(rps))
	rp = rps[0]
	assert.Equal(t, rp.CollectionID, collectionID)
	assert.Equal(t, rp.PlanID, planID)

	DeleteRunningPlan(collectionID, planID)
	rps, err = GetRunningPlans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 0, len(rps))

	// delete should be idempotent
	err = DeleteRunningPlan(collectionID, planID)
	assert.Equal(t, nil, err)
}

func TestMain(m *testing.M) {
	if err := setupAndTeardown(); err != nil {
		log.Fatal(err)
	}
	r := m.Run()
	setupAndTeardown()
	os.Exit(r)
}
