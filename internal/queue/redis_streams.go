package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TaskMessage is the payload stored in Redis Streams.
type TaskMessage struct {
	TaskExecutionID string         `json:"task_execution_id"`
	ExecutionID     string         `json:"execution_id"`
	WorkflowID      string         `json:"workflow_id"`
	TaskDefID       string         `json:"task_def_id"`
	Attempt         int            `json:"attempt"`
	Input           map[string]any `json:"input"`
	TimeoutSec      int            `json:"timeout_sec"`
	IdempotencyKey  string         `json:"idempotency_key"`
}

// Queue wraps Redis Streams for task queueing.
type Queue struct {
	rdb       *redis.Client
	queueName string
	dlqName   string
	groupName string
}

func NewQueue(rdb *redis.Client, queueName, dlqName string) *Queue {
	return &Queue{
		rdb:       rdb,
		queueName: queueName,
		dlqName:   dlqName,
		groupName: "workers",
	}
}

// Init creates consumer groups if they don't exist.
func (q *Queue) Init(ctx context.Context) error {
	for _, stream := range []string{q.queueName, q.dlqName} {
		err := q.rdb.XGroupCreateMkStream(ctx, stream, q.groupName, "0").Err()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return fmt.Errorf("create consumer group for %s: %w", stream, err)
		}
	}
	return nil
}

// Enqueue adds a task message to the stream.
func (q *Queue) Enqueue(ctx context.Context, msg TaskMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal task message: %w", err)
	}
	err = q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: q.queueName,
		Values: map[string]interface{}{
			"payload": string(payload),
		},
	}).Err()
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}
	return nil
}

// Consume reads messages from the stream for a specific consumer.
func (q *Queue) Consume(ctx context.Context, consumerID string, batchSize int, timeout time.Duration) ([]redis.XMessage, error) {
	streams, err := q.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.groupName,
		Consumer: consumerID,
		Streams:  []string{q.queueName, ">"},
		Count:    int64(batchSize),
		Block:    timeout,
	}).Result()
	if err == redis.Nil || err == context.DeadlineExceeded {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("consume from queue: %w", err)
	}
	if len(streams) == 0 {
		return nil, nil
	}
	return streams[0].Messages, nil
}

// Ack acknowledges a processed message.
func (q *Queue) Ack(ctx context.Context, msgID string) error {
	return q.rdb.XAck(ctx, q.queueName, q.groupName, msgID).Err()
}

// MoveToDLQ sends a message to the dead-letter queue.
func (q *Queue) MoveToDLQ(ctx context.Context, msg TaskMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal dlq message: %w", err)
	}
	return q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: q.dlqName,
		Values: map[string]interface{}{
			"payload": string(payload),
		},
	}).Err()
}

// ParseMessage deserialises a Redis stream message into TaskMessage.
func ParseMessage(msg redis.XMessage) (*TaskMessage, error) {
	payload, ok := msg.Values["payload"].(string)
	if !ok {
		return nil, fmt.Errorf("missing payload field")
	}
	var tm TaskMessage
	if err := json.Unmarshal([]byte(payload), &tm); err != nil {
		return nil, fmt.Errorf("parse task message: %w", err)
	}
	return &tm, nil
}

// Len returns the approximate number of pending messages.
func (q *Queue) Len(ctx context.Context) (int64, error) {
	return q.rdb.XLen(ctx, q.queueName).Result()
}
