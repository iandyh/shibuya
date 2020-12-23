package model

type WorkLoad struct {
	Context      string `json:"context"`
	EngineUrl    string `json:"engine_url"`
	RunID        int64  `json:"run_id"`
	CollectionID int64  `json:"collection_id"`
	PlanID       int64  `json:"plan_id"`
	ProjectID    int64  `json:"project_id"`
	//EngineID
}

const (
	SubscriptionQueue = "distributed:sub-queue"
	TermQueue         = "distributed:term-queue"
)
