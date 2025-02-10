package model

import "github.com/rakutentech/shibuya/shibuya/model"

type PlanEnginesConfig struct {
	Kind          model.PlanKind      `json:"kind"`
	Name          string              `json:"Name"`
	Duration      string              `json:"duration"`
	Concurrency   string              `json:"concurrency"`
	Rampup        string              `json:"rampup"`
	EnginesConfig []*EngineDataConfig `json:"engine_data_config"`
}
type EngineDataConfig struct {
	EngineData map[string]*model.ShibuyaFile `json:"engine_data"`
	RunID      int64                         `json:"run_id"`
	EngineID   int                           `json:"engine_id"`
}
