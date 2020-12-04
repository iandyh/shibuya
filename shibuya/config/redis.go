package config

import (
	"github.com/go-redis/redis/v8"
)

func createRedisClient(addr string) *redis.Client {
	rds := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})
	return rds
}
