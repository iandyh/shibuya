package planprogress

import (
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type PlanProgress struct {
	m  map[string]*Progress
	mu sync.RWMutex
}

func NewPlanProgress() *PlanProgress {
	return &PlanProgress{m: make(map[string]*Progress)}
}

// This func depends on the broadcast signal sent to all engines
// and wait for the engines termination and its reporting.
// So never call this func alone.
func (pp *PlanProgress) TermPlan(collectionID, planID string) {
	prgs, ok := pp.Get(collectionID, planID)
	if !ok {
		return
	}
	wait := time.After(1 * time.Minute)
waitLoop:
	for {
		select {
		case <-wait:
			break waitLoop
		default:
			time.Sleep(1 * time.Second)
			if prgs.AnyRunning() {
				continue
			}
			break waitLoop
		}
	}
	pp.Delete(collectionID, planID)
	log.Infof("Delete plan from progress cache %s, %s", collectionID, planID)
}

func (pp *PlanProgress) Add(p *Progress) {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.m[p.MakeKey()] = p
}

func (pp *PlanProgress) Get(collectionID, planID string) (*Progress, bool) {
	pp.mu.RLock()
	defer pp.mu.RUnlock()
	key := makeKey(collectionID, planID)
	v, ok := pp.m[key]
	return v, ok
}

func (pp *PlanProgress) Delete(collectionID, planID string) {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	key := makeKey(collectionID, planID)
	delete(pp.m, key)
}

type Progress struct {
	CollectionID string
	PlanID       string
	Engines      []*EngineProgress
}

func NewProgress(collectionID, planID string, enginesNum int) *Progress {
	engines := make([]*EngineProgress, enginesNum)
	for i := 0; i < enginesNum; i++ {
		engines[i] = NewEngineProgress(i)
	}
	return &Progress{
		CollectionID: collectionID,
		PlanID:       planID,
		Engines:      engines,
	}
}

func (p *Progress) MakeKey() string {
	return makeKey(p.CollectionID, p.PlanID)
}

func (p *Progress) IsRunning() bool {
	if len(p.Engines) == 0 {
		return false
	}
	t := true
	for _, e := range p.Engines {
		t = t && e.GetStatus()
	}
	return t
}

func (p *Progress) AnyRunning() bool {
	if len(p.Engines) == 0 {
		return false
	}
	t := false
	for _, e := range p.Engines {
		t = t || e.GetStatus()
	}
	return t
}

type EngineProgress struct {
	running bool
	mu      sync.RWMutex
}

func NewEngineProgress(engineID int) *EngineProgress {
	ep := &EngineProgress{running: false}
	return ep
}

func (ep *EngineProgress) SetStatus(running bool) {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.running = running
}

func (ep *EngineProgress) GetStatus() bool {
	ep.mu.RLock()
	defer ep.mu.RUnlock()
	return ep.running
}

func makeKey(collectionID, planID string) string {
	return fmt.Sprintf("%s:%s", collectionID, planID)
}
