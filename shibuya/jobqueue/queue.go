package jobqueue

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
We need the underlying job queue to support following features:
1. Add the job to the to the queue
2. Get the job from the queue(listen for new job)
3. To able to ack the message to the queue
4. Resume processing the messages when the consumer is experiencing some restarts(for example, during a release)
*/
type JobQueue interface {
	Enqueue(map[string]interface{}) error
	Dequeue(string) (chan map[string]interface{}, error)
	AckJob(string) error
}
