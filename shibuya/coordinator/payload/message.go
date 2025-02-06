package payload

import (
	"encoding/json"
	"fmt"
)

type Payload struct {
	Verb        string      `json:"verb"`
	PlanMessage PlanMessage `json:"plan_message"`
}

func (p Payload) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

func (p Payload) String() string {
	return p.Verb
}

type PlanMessage map[string]*EngineMessage

func (pm PlanMessage) ToJSON() ([]byte, error) {
	return json.Marshal(pm)
}

func (pm PlanMessage) String() string {
	s := ""
	for pid := range pm {
		s = fmt.Sprintf("%s,%s", s, pid)
	}
	return s
}

type EngineMessage struct {
	Verb      string              `json:"verb"`
	RunID     int64               `json:"run_id"`
	TestFile  string              `json:"test_file"`
	DataFiles map[string]struct{} `json:"data_files"`
}
