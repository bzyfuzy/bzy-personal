// Package logstream provides real-time log streaming for task execution.
// Uses Redis Pub/Sub for push delivery and Redis Streams for durable storage.
package logstream

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Level represents log severity.
type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// LogEntry is a single log line from a task.
type LogEntry struct {
	TaskID    string    `json:"task_id"`
	WorkerID  string    `json:"worker_id"`
	Level     Level     `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Writer writes log entries for a task.
type Writer interface {
	Write(ctx context.Context, entry LogEntry) error
	WriteString(ctx context.Context, taskID, level, msg string) error
}

// Reader reads log entries for a task.
type Reader interface {
	// Tail streams log entries in real-time until context is cancelled.
	Tail(ctx context.Context, taskID string, ch chan<- LogEntry) error
	// History returns stored log entries for a task.
	History(ctx context.Context, taskID string, limit int) ([]LogEntry, error)
}

// Stream combines Writer and Reader.
type Stream interface {
	Writer
	Reader
}

// ─── Redis implementation ─────────────────────────────────────────────────────

type redisStream struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisStream(rdb *redis.Client) Stream {
	return &redisStream{client: rdb, ttl: 24 * time.Hour}
}

func streamKey(taskID string) string { return fmt.Sprintf("bzy:logs:%s", taskID) }
func pubChannel(taskID string) string { return fmt.Sprintf("bzy:logs:pub:%s", taskID) }

func (s *redisStream) Write(ctx context.Context, entry LogEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	// Persist to stream
	_, err := s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey(entry.TaskID),
		MaxLen: 10000,
		Values: map[string]any{
			"level":     string(entry.Level),
			"message":   entry.Message,
			"worker_id": entry.WorkerID,
			"ts":        entry.Timestamp.UnixMilli(),
		},
	}).Result()
	if err != nil {
		return err
	}

	// Publish for live subscribers
	msg := fmt.Sprintf(`{"level":"%s","message":%q,"ts":%d}`,
		entry.Level, entry.Message, entry.Timestamp.UnixMilli())
	return s.client.Publish(ctx, pubChannel(entry.TaskID), msg).Err()
}

func (s *redisStream) WriteString(ctx context.Context, taskID, level, msg string) error {
	return s.Write(ctx, LogEntry{
		TaskID:    taskID,
		Level:     Level(level),
		Message:   msg,
		Timestamp: time.Now().UTC(),
	})
}

func (s *redisStream) Tail(ctx context.Context, taskID string, ch chan<- LogEntry) error {
	sub := s.client.Subscribe(ctx, pubChannel(taskID))
	defer sub.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-sub.Channel():
			ch <- LogEntry{
				TaskID:  taskID,
				Message: msg.Payload,
			}
		}
	}
}

func (s *redisStream) History(ctx context.Context, taskID string, limit int) ([]LogEntry, error) {
	entries, err := s.client.XRevRangeN(ctx, streamKey(taskID), "+", "-", int64(limit)).Result()
	if err != nil {
		return nil, err
	}

	out := make([]LogEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, LogEntry{
			TaskID:  taskID,
			Level:   Level(e.Values["level"].(string)),
			Message: e.Values["message"].(string),
		})
	}
	return out, nil
}
