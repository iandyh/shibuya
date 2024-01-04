package mq

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	localRedis = "localhost:6379"
	streamName = "messages"
	groupName  = "messages_group"
)

func makeTestMessages(number int) []map[string]interface{} {
	messages := []map[string]interface{}{}
	for i := 0; i < number; i++ {
		j := make(map[string]interface{})
		j["engine_url"] = fmt.Sprintf("%d", i)
		messages = append(messages, j)
	}
	return messages
}

func makeRMQ() *RedisMessageQueue {
	c := NewRedisClient(localRedis)
	return &RedisMessageQueue{c: c}
}

func tearDownTest(t *testing.T) {
	c := NewRedisClient(localRedis)
	if err := c.XGroupDestroy(ctx, streamName, groupName).Err(); err != nil {
		t.Error(err)
	}
}

func TestNewRedisClient(t *testing.T) {
	c := NewRedisClient(localRedis)
	err := c.Ping(ctx).Err()
	assert.Nil(t, err)
}

func TestEnqueueDequeue(t *testing.T) {
	defer tearDownTest(t)
	rmq := makeRMQ()
	messagesCount := 10
	messages := makeTestMessages(messagesCount)
	for _, j := range messages {
		err := rmq.Enqueue(streamName, groupName, j)
		assert.Nil(t, err)
	}
	resultChan := make(chan Message)
	for i := 0; i < 10; i++ {
		go func(index int) {
			chanchan, err := rmq.Dequeue(streamName, groupName, fmt.Sprintf("%d", index))
			assert.Nil(t, err)
			for m := range chanchan {
				messageID := m.MessageID
				if err := rmq.AckMessage(streamName, groupName, messageID); err != nil {
					t.Error(err)
				}
				resultChan <- m
			}
		}(i)
	}
	for i := 0; i < messagesCount; i++ {
		<-resultChan
	}
	pendingMessages, err := rmq.GroupPendingMessages(streamName, groupName)
	assert.Nil(t, err)
	assert.Equal(t, 0, int(pendingMessages))
	assert.Equal(t, 10, messagesCount)
}
