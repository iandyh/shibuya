package planprogress_test

import (
	"testing"
	"time"

	"github.com/rakutentech/shibuya/shibuya/coordinator/planprogress"
	"github.com/stretchr/testify/assert"
)

func TestPlanProgress(t *testing.T) {
	pp := planprogress.NewPlanProgress()
	collectionID := "1"
	planID := "1"
	p := planprogress.NewProgress(collectionID, planID, 2)
	pp.Add(p)

	getPP, ok := pp.Get(collectionID, planID)
	assert.True(t, ok)
	assert.Equal(t, collectionID, getPP.CollectionID)
	assert.Equal(t, planID, getPP.PlanID)
	assert.Equal(t, 2, len(p.Engines))

	pp.Delete(collectionID, planID)
	getPP, ok = pp.Get(collectionID, planID)
	assert.False(t, ok)
}

func TestProgress(t *testing.T) {
	collectionID := "1"
	planID := "1"
	enginesNum := 2
	p := planprogress.NewProgress(collectionID, planID, enginesNum)
	assert.False(t, p.IsRunning())
	assert.False(t, p.IsRunning())
	assert.False(t, p.AnyRunning())

	for _, ep := range p.Engines {
		assert.False(t, ep.GetStatus())
		ep.SetStatus(true)
		assert.True(t, p.AnyRunning())
		assert.True(t, ep.GetStatus())
	}
	assert.True(t, p.IsRunning())

	pp := planprogress.NewPlanProgress()
	pp.Add(p)
	_, ok := pp.Get(collectionID, planID)
	assert.True(t, ok)
	go func() {
		time.Sleep(2 * time.Second)
		for _, ep := range p.Engines {
			ep.SetStatus(false)
		}
	}()
	pp.TermPlan(collectionID, planID)
	_, ok = pp.Get(collectionID, planID)
	assert.False(t, ok)
}
