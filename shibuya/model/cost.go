package model

import (
	"log"
	"time"

	"github.com/rakutentech/shibuya/shibuya/config"
)

type UsageSummary struct {
	TotalEngineHours   map[string]float64            `json:"total_engine_hours"`
	TotalNodesHours    map[string]float64            `json:"total_nodes_hours"`
	EngineHoursByOwner map[string]map[string]float64 `json:"engine_hours_by_owner"`
}

type LaunchHistory struct {
	Context     string
	Owner       string
	Engines     int
	Nodes       int
	StartedTime time.Time
	EndTime     time.Time
}

func GetHistory(startedTime, endTime string) ([]*LaunchHistory, error) {
	db := config.SC.DBC
	q, err := db.Prepare("select context, owner, engines_count, nodes_count, started_time, end_time from collection_launch_history where started_time > ? and end_time < ?")
	if err != nil {
		return nil, err
	}
	rs, err := q.Query(startedTime, endTime)
	defer rs.Close()

	history := []*LaunchHistory{}
	for rs.Next() {
		lh := new(LaunchHistory)
		rs.Scan(&lh.Context, &lh.Owner, &lh.Engines, &lh.Nodes, &lh.StartedTime, &lh.EndTime)
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
	s.TotalNodesHours = make(map[string]float64)
	for _, h := range history {
		teh := s.TotalEngineHours
		tnh := s.TotalNodesHours
		ehe := s.EngineHoursByOwner
		duration := h.EndTime.Sub(h.StartedTime)
		engineHours := duration.Hours() * float64(h.Engines)
		nodeHours := duration.Hours() * float64(h.Nodes)
		teh[h.Context] += engineHours
		tnh[h.Context] += nodeHours
		if m, ok := ehe[h.Owner]; !ok {
			ehe[h.Owner] = make(map[string]float64)
			ehe[h.Owner][h.Context] += engineHours
		} else {
			m[h.Context] += engineHours
		}
	}
	var all float64
	for _, usage := range s.TotalEngineHours {
		all += usage
	}
	s.TotalEngineHours["engine_hours"] = all

	return s, nil
}

func GetPastMonthHistory(start_run_id, end_run_id int64) ([]*RunHistory, error) {
	db := config.SC.DBC
	q, err := db.Prepare("select collection_id, started_time, end_time from collection_run_history where run_id >=? and run_id <=?")
	if err != nil {
		return nil, err
	}
	defer q.Close()

	r := []*RunHistory{}
	rs, err := q.Query(start_run_id, end_run_id)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		run := new(RunHistory)
		rs.Scan(&run.CollectionID, &run.StartedTime, &run.EndTime)
		r = append(r, run)
	}
	return r, nil
}

type result struct {
	engineHours  float64
	engines      int
	threads      int
	hasEndTime   bool
	collectionID int64
	owner        string
}

type ResultPage struct {
	TotalRuns        int                `json:"total_runs"`
	TotalEngineHours float64            `json:"total_engine_hours"`
	NoEndtime        int                `json:"no_end_time"`
	TotalThreads     int                `json:"total_threads"`
	TotalEngines     int                `json:"total_engines"`
	CollectionUsage  map[int64]int      `json:"collection_engine_usage"`
	OwnerUsage       map[string]float64 `json:"owner_engine_hour_usage"`
}

func CalEngineHours(start_run_id, end_run_id int64) *ResultPage {
	history, err := GetPastMonthHistory(start_run_id, end_run_id)
	if err != nil {
		log.Print(err)
	}

	work := make(chan *RunHistory, len(history))
	resultChan := make(chan *result, len(history))
	for w := 0; w < 5; w++ {
		go func() {
			for h := range work {
				hasEndtime := !h.EndTime.IsZero()
				var duration time.Duration
				if hasEndtime {
					duration = h.EndTime.Sub(h.StartedTime)
				} else {
					duration, _ = time.ParseDuration("0s")
				}
				collection, err := GetCollection(h.CollectionID)
				if err != nil {
					log.Print(err)
				}
				eps, _ := collection.GetExecutionPlans()
				total_engines := 0
				total_threads := 0
				for _, e := range eps {
					total_engines += e.Engines
					total_threads += e.Concurrency * e.Engines
				}
				p, err := GetProject(collection.ProjectID)
				if err != nil {
					log.Print(err)
				}
				resultChan <- &result{
					engineHours:  duration.Hours() * float64(total_engines),
					engines:      total_engines,
					hasEndTime:   hasEndtime,
					threads:      total_threads,
					collectionID: h.CollectionID,
					owner:        p.Owner,
				}
			}
		}()
	}
	for _, h := range history {
		work <- h
	}
	var totalEngineHours float64
	totalEngines := 0
	noEndtime := 0
	totalThreads := 0
	collectionEngines := make(map[int64]int)
	ownerEngines := make(map[string]float64)
	for range history {
		item := <-resultChan
		totalEngineHours += item.engineHours
		totalEngines += item.engines
		totalThreads += item.threads
		collectionEngines[item.collectionID] = item.engines
		ownerEngines[item.owner] += item.engineHours
		if !item.hasEndTime {
			noEndtime += 1
		}
	}
	//sortedEngines := make([]int, len(collectionEngines))
	close(work)
	close(resultChan)
	rp := &ResultPage{
		TotalRuns:        len(history),
		TotalEngineHours: totalEngineHours,
		NoEndtime:        noEndtime,
		TotalEngines:     totalEngines,
		TotalThreads:     totalThreads,
		CollectionUsage:  collectionEngines,
		OwnerUsage:       ownerEngines,
	}
	return rp
}
