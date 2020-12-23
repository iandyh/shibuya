package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-redis/redis/v8"
	model "github.com/rakutentech/shibuya/shibuya/controller/model"
	log "github.com/sirupsen/logrus"
)

var ctx = context.Background()

func createRedisClient(addr string) *redis.Client {
	rds := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})
	return rds
}

type ShibuyaWorker struct {
	rds           *redis.Client
	id            string
	jobKey        string
	jobHistoryKey string
}

func newShibuyaWorker(rds *redis.Client, id string) *ShibuyaWorker {
	jobKey := fmt.Sprintf("distributed:job:%s:", id)
	jobHistoryKey := fmt.Sprintf("distributed:history:%s", id)
	return &ShibuyaWorker{
		rds:           rds,
		id:            id,
		jobKey:        jobKey,
		jobHistoryKey: jobHistoryKey,
	}
}

func (sw *ShibuyaWorker) unmarshalWorkLoad(r string) (*model.WorkLoad, error) {
	wl := new(model.WorkLoad)
	if err := json.Unmarshal([]byte(r), wl); err != nil {
		return nil, err
	}
	return wl, nil
}

func (sw *ShibuyaWorker) marshalWorkLoad(wl *model.WorkLoad) (string, error) {
	bts, err := json.Marshal(wl)
	if err != nil {
		return "", err
	}
	return string(bts), nil
}

func (sw *ShibuyaWorker) addToWorkList(wl *model.WorkLoad) error {
	rds := sw.rds
	t, err := sw.marshalWorkLoad(wl)
	if err != nil {
		return err
	}
	key := sw.jobKey + wl.EngineUrl
	log.Print(key)
	err = rds.Set(ctx, key, t, 0).Err()
	if err != nil {
		return err
	}
	return rds.LPush(ctx, sw.jobHistoryKey, key).Err()
}

func (sw *ShibuyaWorker) removeFromWorkList(wl *model.WorkLoad) error {
	rds := sw.rds
	key := sw.jobKey + wl.EngineUrl
	rds.LRem(ctx, sw.jobHistoryKey, 0, key)
	rds.Del(ctx, key)
	return nil
}

func (sw *ShibuyaWorker) subscribeAndRead(wl *model.WorkLoad) {
	e := model.NewEngine(wl.ProjectID, wl.CollectionID, wl.PlanID, wl.RunID, wl.EngineUrl)
	log.Print(wl.EngineUrl)
	e.Subscribe()
	e.ReadMetrics()
}

func (sw *ShibuyaWorker) resume() {
	// When worker restarts, probably due to release, it needs to resume the previous subscription
	rds := sw.rds
	result, err := rds.LRange(ctx, sw.jobHistoryKey, 0, -1).Result()
	if err != nil {

	}
	for _, item := range result {
		log.Print(item)
		r, err := rds.Get(ctx, item).Result()
		if err != nil {
		}
		wl, err := sw.unmarshalWorkLoad(r)
		go sw.subscribeAndRead(wl)
	}

}

func (sw *ShibuyaWorker) listenForTrigger() {
	for {
		result, err := sw.rds.BLPop(ctx, 0, model.SubscriptionQueue).Result()
		if err != nil {
			log.Print(err)
		}
		wl, err := sw.unmarshalWorkLoad(result[1])
		if err != nil {
			continue
		}
		sw.addToWorkList(wl)
		go sw.subscribeAndRead(wl)
	}
}

func (sw *ShibuyaWorker) listenForTerm() {
	for {
		result, err := sw.rds.BLPop(ctx, 0, model.TermQueue).Result()
		if err != nil {
			log.Print(err)
		}
		log.Print(result[1])
		wl, err := sw.unmarshalWorkLoad(result[1])
		if err != nil {
			continue
		}
		sw.removeFromWorkList(wl)
		engine := model.NewEngine(wl.ProjectID, wl.CollectionID, wl.PlanID, 0, wl.EngineUrl)
		engine.Terminate()
		log.Printf("terminated for %s", wl.EngineUrl)

		// also need to deal when the engine is stopped
	}
}

func (sw *ShibuyaWorker) start() {
	go sw.resume()
	go sw.listenForTrigger()
	sw.listenForTerm()
}

func main() {
	rds := createRedisClient("redis:6379")
	id := os.Getenv("ID")
	sw := newShibuyaWorker(rds, id)
	sw.start()
}
