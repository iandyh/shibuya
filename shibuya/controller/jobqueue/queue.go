package jobqueue

import "encoding/json"

type Job struct {
	EngineUrl string `json:"engine_url"`
}

func (j *Job) Map() map[string]interface{} {
	m := make(map[string]interface{})
	t, _ := json.Marshal(j)
	json.Unmarshal(t, &m)
	return m
}

func fromMap(m map[string]interface{}) *Job {
	t, _ := json.Marshal(m)
	j := &Job{}
	json.Unmarshal(t, &j)
	return j
}
