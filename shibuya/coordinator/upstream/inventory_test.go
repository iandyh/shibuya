package upstream

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeDummyEngineEndpoint(collectionID, planID string) map[string][]EngineEndPoint {
	return map[string][]EngineEndPoint{
		collectionID: {
			{
				collectionID: collectionID,
				addr:         fmt.Sprintf("addr:%s", collectionID),
				path:         fmt.Sprintf("/%s", collectionID),
				planID:       planID,
			},
			{
				collectionID: collectionID,
				addr:         fmt.Sprintf("addr2:%s", collectionID),
				path:         fmt.Sprintf("/%s/2", collectionID),
				planID:       fmt.Sprintf("/%s/2", "2"),
			},
		},
	}
}

func TestInventory(t *testing.T) {
	inventory, err := NewInventory("", false)
	assert.Nil(t, err)
	collectionID := "1"
	planID := "1"
	ibc := makeDummyEngineEndpoint(collectionID, planID)
	inventory.updateInventory(ibc)
	assert.Equal(t, 2, inventory.GetEndpointsCountByCollection(collectionID))
	ibc[collectionID][0].addr = "new addr"
	inventory.updateInventory(ibc)
	assert.Equal(t, "new addr", inventory.FindPodIP(ibc[collectionID][0].path))
	assert.Equal(t, 1, len(inventory.GetPlanEndpoints(collectionID, planID)))

	anotherCollection := "2"
	anotherPlan := "2"
	ibc = makeDummyEngineEndpoint(anotherCollection, anotherPlan)
	inventory.updateInventory(ibc)
	assert.Equal(t, 2, inventory.GetEndpointsCountByCollection(anotherCollection))
	assert.Equal(t, 0, inventory.GetEndpointsCountByCollection(collectionID))
}
