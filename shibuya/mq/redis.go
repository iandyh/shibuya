package mq

import (
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(addr string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	return rdb
}

type RedisMessageQueue struct {
	c *redis.Client
}

func (rmq *RedisMessageQueue) Enqueue(streamName, groupName string, message map[string]interface{}) error {
	// create the consumer group
	// publish the message
	c := rmq.c

	// we need to put the group creation into process start. Not here.
	if err := c.XGroupCreateMkStream(ctx, streamName, groupName, "$").Err(); err != nil {
		log.Println(err)
	}
	if _, err := c.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: message,
	}).Result(); err != nil {
		return err
	}
	return nil
}

// we should return a channel of messages
func (rmq *RedisMessageQueue) Dequeue(streamName, groupName, consumerID string) (chan Message, error) {
	c := rmq.c
	messageChan := make(chan Message)
	go func() {
		for {
			streams, err := c.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    groupName,
				Consumer: consumerID,
				Streams:  []string{streamName, ">"},
				Block:    0,
				Count:    1,
			}).Result()
			if err != nil {
				log.Println(err)
				close(messageChan)
				break
			}
			for _, item := range streams {
				for _, xm := range item.Messages {
					m := Message{
						MessageID: xm.ID,
						Body:      xm.Values,
					}
					messageChan <- m
				}
			}
		}
	}()
	return messageChan, nil
}

func (rmq *RedisMessageQueue) AckMessage(streamName, groupName, messageID string) error {
	return rmq.c.XAck(ctx, streamName, groupName, messageID).Err()
}

func (rmq *RedisMessageQueue) GroupPendingMessages(streamName, groupName string) (int64, error) {
	xp, err := rmq.c.XPending(ctx, streamName, groupName).Result()
	if err != nil {
		return 0, err
	}
	return xp.Count, nil
}

func (rmq *RedisMessageQueue) PendingMessagesPerConsumer(streamName, groupName, consumerID string) (int64, error) {
	xp, err := rmq.c.XPending(ctx, streamName, groupName).Result()
	if err != nil {
		return 0, err
	}
	count, ok := xp.Consumers[consumerID]
	if !ok {
		return 0, fmt.Errorf("Cannot find consumerID %s", consumerID)
	}
	return count, nil
}
