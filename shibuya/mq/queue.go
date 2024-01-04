package mq

import (
	"context"
)

var (
	ctx = context.Background()
)

type Message struct {
	MessageID string
	Body      map[string]interface{}
}

/*
We need the underlying message queue to support following features:
1. Add the message to the to the queue
2. Get the message from the queue(listen for new message)
3. To able to ack the message to the queue
4. Resume processing the messages when the consumer is experiencing some restarts(for example, during a release)
*/
type MessageQueue interface {
	Enqueue(map[string]interface{}) error
	Dequeue(string) (chan map[string]interface{}, error)
	AckMessage(string) error
}
