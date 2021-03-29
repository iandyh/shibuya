package model

import (
	"log"
	"math"
	"time"

	"github.com/rakutentech/shibuya/shibuya/config"
)

type UsageSummary struct {
	// TotalEngineHours   map[string]float64            `json:"total_engine_hours"`
	// TotalNodesHours    map[string]float64            `json:"total_nodes_hours"`
	// EngineHoursByOwner map[string]map[string]float64 `json:"engine_hours_by_owner"`
	TotalVUH   map[string]float64            `json:"total_vuh"`
	VUHByOnwer map[string]map[string]float64 `json:"vuh_by_owner"`
	Contacts   map[string][]string           `json:"contacts"`
}

type LaunchHistory struct {
	Context      string
	CollectionID int64
	Owner        string
	vu           int
	StartedTime  time.Time
	EndTime      time.Time
}

func GetHistory(startedTime, endTime string) ([]*LaunchHistory, error) {
	db := config.SC.DBC
	q, err := db.Prepare("select collection_id, context, owner, vu, started_time, end_time from collection_launch_history2 where started_time > ? and end_time < ?")
	if err != nil {
		return nil, err
	}
	rs, err := q.Query(startedTime, endTime)
	defer rs.Close()

	history := []*LaunchHistory{}
	for rs.Next() {
		lh := new(LaunchHistory)
		rs.Scan(&lh.CollectionID, &lh.Context, &lh.Owner, &lh.vu, &lh.StartedTime, &lh.EndTime)
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
	s.TotalVUH = make(map[string]float64)
	s.VUHByOnwer = make(map[string]map[string]float64)
	s.Contacts = make(map[string][]string)
	uniqueOwners := make(map[string]struct{})
	uniqueCollections := make(map[int64]struct{})
	for _, h := range history {
		uniqueOwners[h.Owner] = struct{}{}
		uniqueCollections[h.CollectionID] = struct{}{}
	}
	collectionsToProjects := make(map[int64]int64)
	for cid, _ := range uniqueCollections {
		c, err := GetCollection(cid)
		if err != nil {
			continue
		}
		collectionsToProjects[cid] = c.ProjectID
	}
	owners := []string{}
	for o, _ := range uniqueOwners {
		owners = append(owners, o)
	}
	projects, _ := GetProjectsByOwners(owners)
	ownerToSid := make(map[string]string)
	for _, p := range projects {
		sid := p.Sid
		if sid == "" {
			sid = "unknown"
		}
		ownerToSid[p.Owner] = sid
	}
	for owner, sid := range ownerToSid {
		contacts := s.Contacts[sid]
		contacts = append(contacts, owner)
		s.Contacts[sid] = contacts
	}
	for _, h := range history {
		totalVUH := s.TotalVUH
		vhByOwner := s.VUHByOnwer
		sid, _ := ownerToSid[h.Owner]
		if sid == "unknown" {
			if pid, ok := collectionsToProjects[h.CollectionID]; ok {
				p, err := GetProject(pid)
				if err == nil && p.Sid != "" {
					sid = p.Sid
				}
			}
		}
		duration := h.EndTime.Sub(h.StartedTime)

		// if users run 0.1 hours, we should bill them based on 1 hour.
		billingHours := math.Ceil(duration.Hours())
		vuh := billingHours * float64(h.vu)
		totalVUH[h.Context] += vuh
		if m, ok := vhByOwner[sid]; !ok {
			vhByOwner[sid] = make(map[string]float64)
			vhByOwner[sid][h.Context] += vuh
		} else {
			m[h.Context] += vuh
		}
	}
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
