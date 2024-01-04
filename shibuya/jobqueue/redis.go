package jobqueue

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

type RedisJobQueue struct {
	c *redis.Client
}

func (rj *RedisJobQueue) Enqueue(streamName, groupName string, message map[string]interface{}) error {
	// create the consumer group
	// publish the message
	c := rj.c

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
func (rj *RedisJobQueue) Dequeue(streamName, groupName, consumerID string) (chan Message, error) {
	c := rj.c
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

func (rj *RedisJobQueue) AckMessage(streamName, groupName, messageID string) error {
	return rj.c.XAck(ctx, streamName, groupName, messageID).Err()
}

func (rj *RedisJobQueue) GroupPendingMessages(streamName, groupName string) (int64, error) {
	xp, err := rj.c.XPending(ctx, streamName, groupName).Result()
	if err != nil {
		return 0, err
	}
	return xp.Count, nil
}

func (rj *RedisJobQueue) PendingMessagesPerConsumer(streamName, groupName, consumerID string) (int64, error) {
	xp, err := rj.c.XPending(ctx, streamName, groupName).Result()
	if err != nil {
		return 0, err
	}
	count, ok := xp.Consumers[consumerID]
	if !ok {
		return 0, fmt.Errorf("Cannot find consumerID %s", consumerID)
	}
	return count, nil
}
