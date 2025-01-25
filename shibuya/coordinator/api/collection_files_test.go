package api_test

import (
	"testing"

	"github.com/rakutentech/shibuya/shibuya/coordinator/api"
	"github.com/stretchr/testify/assert"
)

func TestFormFileKey(t *testing.T) {
	key := "asdf"
	ffk := api.FormFileKey(key)
	collectionDataKey := "data:collection:asdf"
	planDataKey := "data:plan:asdf"
	testFileKey := "test:asdf"
	assert.Equal(t, collectionDataKey, ffk.MakeCollectionDataKey())
	assert.Equal(t, planDataKey, ffk.MakePlanDataKey())
	assert.Equal(t, testFileKey, ffk.MakeTestFileKey())

	cdk := api.FormFileKey(collectionDataKey)
	pdk := api.FormFileKey(planDataKey)
	tfk := api.FormFileKey(testFileKey)
	assert.True(t, cdk.IsCollectionData())
	assert.True(t, pdk.IsPlanData())
	assert.True(t, tfk.IsTestFile())

	assert.Equal(t, "asdf", pdk.PlanID())
	assert.Equal(t, "asdf", tfk.PlanID())
	assert.Equal(t, "", cdk.PlanID())
}
