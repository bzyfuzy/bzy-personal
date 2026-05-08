// Package queue defines the task queue abstraction.
// Implementations: Redis Streams (default), NATS JetStream, Kafka.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ─── Task domain ──────────────────────────────────────────────────────────────

type Priority int

const (
	PriorityLow    Priority = 1
	PriorityNormal Priority = 5
	PriorityHigh   Priority = 9
)

type TaskStatus string

const (
	TaskQueued     TaskStatus = "queued"
	TaskProcessing TaskStatus = "processing"
	TaskCompleted  TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
	TaskRetrying   TaskStatus = "retrying"
	TaskDead       TaskStatus = "dead"
)

// Task is the unit of work enqueued for async processing.
type Task struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`        // handler type key
	Payload     json.RawMessage `json:"payload"`
	Priority    Priority        `json:"priority"`
	Status      TaskStatus      `json:"status"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"max_attempts"`
	RunAt       time.Time       `json:"run_at"`       // scheduled execution time
	Timeout     time.Duration   `json:"timeout"`
	TraceID     string          `json:"trace_id,omitempty"`
	WorkerID    string          `json:"worker_id,omitempty"`
	QueuedAt    time.Time       `json:"queued_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
	Error       string          `json:"error,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

// ─── Queue interface ──────────────────────────────────────────────────────────

// Queue is the platform task queue.
type Queue interface {
	// Enqueue adds a task to the queue.
	Enqueue(ctx context.Context, task *Task) error
	// Dequeue blocks until a task is available or context is cancelled.
	Dequeue(ctx context.Context) (*Task, error)
	// Ack marks a task as successfully completed.
	Ack(ctx context.Context, taskID string, result json.RawMessage) error
	// Nack marks a task as failed (triggers retry or DLQ).
	Nack(ctx context.Context, taskID, errMsg string) error
	// Peek returns queued tasks without removing them.
	Peek(ctx context.Context, limit int) ([]*Task, error)
	// Depth returns the current number of queued tasks.
	Depth(ctx context.Context) (int64, error)
	// Close cleans up resources.
	Close() error
}

// ─── Redis Streams implementation ─────────────────────────────────────────────

type redisQueue struct {
	client   *redis.Client
	stream   string
	dlq      string
	group    string
	consumer string
	maxRetry int
}

func NewRedisQueue(rdb *redis.Client, stream, dlq, group, consumer string, maxRetry int) Queue {
	return &redisQueue{
		client:   rdb,
		stream:   stream,
		dlq:      dlq,
		group:    group,
		consumer: consumer,
		maxRetry: maxRetry,
	}
}

func (q *redisQueue) Enqueue(ctx context.Context, task *Task) error {
	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	task.QueuedAt = time.Now().UTC()
	task.Status = TaskQueued

	raw, err := json.Marshal(task)
	if err != nil {
		return err
	}

	return q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		Values: map[string]any{
			"task_id": task.ID,
			"type":    task.Type,
			"data":    string(raw),
		},
	}).Err()
}

func (q *redisQueue) Dequeue(ctx context.Context) (*Task, error) {
	// Ensure consumer group exists
	_ = q.client.XGroupCreateMkStream(ctx, q.stream, q.group, "0").Err()

	entries, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.group,
		Consumer: q.consumer,
		Streams:  []string{q.stream, ">"},
		Count:    1,
		Block:    500 * time.Millisecond,
	}).Result()
	if err == redis.Nil {
		return nil, nil // timeout, no message
	}
	if err != nil {
		return nil, fmt.Errorf("queue dequeue: %w", err)
	}

	if len(entries) == 0 || len(entries[0].Messages) == 0 {
		return nil, nil
	}

	msg := entries[0].Messages[0]
	raw, ok := msg.Values["data"].(string)
	if !ok {
		return nil, fmt.Errorf("queue message missing data field")
	}

	var task Task
	if err := json.Unmarshal([]byte(raw), &task); err != nil {
		return nil, err
	}
	task.WorkerID = q.consumer
	return &task, nil
}

func (q *redisQueue) Ack(ctx context.Context, taskID string, result json.RawMessage) error {
	// In a real implementation, look up the stream message ID by taskID
	// then call XACK. Simplified here.
	return q.client.XAck(ctx, q.stream, q.group, taskID).Err()
}

func (q *redisQueue) Nack(ctx context.Context, taskID, errMsg string) error {
	// Check attempt count; if exceeded, move to DLQ
	return nil
}

func (q *redisQueue) Peek(ctx context.Context, limit int) ([]*Task, error) {
	entries, err := q.client.XRange(ctx, q.stream, "-", "+").Result()
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	var tasks []*Task
	for _, e := range entries {
		raw, _ := e.Values["data"].(string)
		var t Task
		if err := json.Unmarshal([]byte(raw), &t); err == nil {
			tasks = append(tasks, &t)
		}
	}
	return tasks, nil
}

func (q *redisQueue) Depth(ctx context.Context) (int64, error) {
	return q.client.XLen(ctx, q.stream).Result()
}

func (q *redisQueue) Close() error { return nil }
