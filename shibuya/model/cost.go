package model

import (
	"time"

	"github.com/rakutentech/shibuya/shibuya/config"
)

type UsageSummary struct {
	TotalEngineHours   map[string]float64            `json:"total_engine_hours"`
	EngineHoursByOwner map[string]map[string]float64 `json:"engine_hours_by_owner"`
}

type LaunchHistory struct {
	Context     string
	Owner       string
	Engines     int
	StartedTime time.Time
	EndTime     time.Time
}

func GetHistory(startedTime, endTime string) ([]*LaunchHistory, error) {
	db := config.SC.DBC
	q, err := db.Prepare("select context, owner, engines_count, started_time, end_time from collection_launch_history where started_time > ? and end_time < ?")
	if err != nil {
		return nil, err
	}
	rs, err := q.Query(startedTime, endTime)
	defer rs.Close()

	history := []*LaunchHistory{}
	for rs.Next() {
		lh := new(LaunchHistory)
		rs.Scan(&lh.Context, &lh.Owner, &lh.Engines, &lh.StartedTime, &lh.EndTime)
		history = append(history, lh)
	}
	return history, nil
}

func GetUsageSummary(startedTime, endTime string) (*UsageSummary, error) {
	history, err := GetHistory(startedTime, endTime)
	if err != nil {
		return nil, err
	}
	s := new(UsageSummary)
	s.TotalEngineHours = make(map[string]float64)
	s.EngineHoursByOwner = make(map[string]map[string]float64)
	for _, h := range history {
		teh := s.TotalEngineHours
		ehe := s.EngineHoursByOwner
		duration := h.EndTime.Sub(h.StartedTime)
		engineHours := duration.Hours() * float64(h.Engines)
		teh[h.Context] += engineHours
		if m, ok := ehe[h.Owner]; !ok {
			ehe[h.Owner] = make(map[string]float64)
			ehe[h.Owner][h.Context] += engineHours
		} else {
			m[h.Context] += engineHours
		}
	}
	return s, nil
}
