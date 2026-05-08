package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ─── Types ────────────────────────────────────────────────────────────────────

type TaskPriority int

const (
	PriorityLow    TaskPriority = 1
	PriorityNormal TaskPriority = 5
	PriorityHigh   TaskPriority = 9
)

type TaskStatus string

const (
	TaskQueued     TaskStatus = "queued"
	TaskProcessing TaskStatus = "processing"
	TaskCompleted  TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
	TaskRetrying   TaskStatus = "retrying"
	TaskDead       TaskStatus = "dead"
	TaskCancelled  TaskStatus = "cancelled"
)

type EnqueueRequest struct {
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Priority    TaskPriority    `json:"priority,omitempty"`
	MaxAttempts int             `json:"max_attempts,omitempty"`
	Timeout     time.Duration   `json:"timeout,omitempty"`
	RunAt       *time.Time      `json:"run_at,omitempty"` // nil = run immediately
}

type EnqueueResponse struct {
	TaskID string `json:"task_id"`
}

type Task struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Priority    TaskPriority    `json:"priority"`
	Status      TaskStatus      `json:"status"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"max_attempts"`
	WorkerID    string          `json:"worker_id,omitempty"`
	Error       string          `json:"error,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
	QueuedAt    time.Time       `json:"queued_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
}

type ListTasksResponse struct {
	Tasks         []Task `json:"tasks"`
	NextPageToken string `json:"next_page_token,omitempty"`
}

// LogEntry is a single task log line from the streaming log endpoint.
type LogEntry struct {
	TaskID    string    `json:"task_id"`
	WorkerID  string    `json:"worker_id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// LogStream is a live SSE stream of task log entries.
type LogStream struct {
	sse *SSEReader
}

// Next returns the next log entry. Returns io.EOF when the stream ends.
func (ls *LogStream) Next(ctx context.Context) (*LogEntry, error) {
	event, err := ls.sse.Next()
	if err != nil {
		return nil, err
	}
	var entry LogEntry
	if err := event.DecodeData(&entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// Each calls fn for every log line until the stream ends or ctx is cancelled.
func (ls *LogStream) Each(ctx context.Context, fn func(*LogEntry) error) error {
	return ls.sse.Iter(ctx, func(e *SSEEvent) error {
		var entry LogEntry
		if err := e.DecodeData(&entry); err != nil {
			return err
		}
		return fn(&entry)
	})
}

// Close releases the stream.
func (ls *LogStream) Close() { ls.sse.Close() }

// ─── TasksClient ──────────────────────────────────────────────────────────────

type TasksClient struct{ c *Client }

// Enqueue submits a task for async execution. Returns the task ID.
func (t *TasksClient) Enqueue(ctx context.Context, req EnqueueRequest) (string, error) {
	var resp EnqueueResponse
	if err := t.c.request(ctx, http.MethodPost, "/api/v1/tasks/", req, &resp); err != nil {
		return "", fmt.Errorf("enqueue: %w", err)
	}
	return resp.TaskID, nil
}

// EnqueueJSON is a convenience wrapper that marshals payload from any value.
func (t *TasksClient) EnqueueJSON(ctx context.Context, taskType string, payload any, opts ...func(*EnqueueRequest)) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req := EnqueueRequest{Type: taskType, Payload: raw}
	for _, o := range opts {
		o(&req)
	}
	return t.Enqueue(ctx, req)
}

// Get retrieves a task by ID.
func (t *TasksClient) Get(ctx context.Context, id string) (*Task, error) {
	var task Task
	if err := t.c.request(ctx, http.MethodGet, "/api/v1/tasks/"+id, nil, &task); err != nil {
		return nil, fmt.Errorf("task get: %w", err)
	}
	return &task, nil
}

// Cancel requests cancellation of a running or queued task.
func (t *TasksClient) Cancel(ctx context.Context, id string) error {
	return t.c.request(ctx, http.MethodDelete, "/api/v1/tasks/"+id, nil, nil)
}

// List returns tasks filtered by optional status.
func (t *TasksClient) List(ctx context.Context, status TaskStatus, limit int) (*ListTasksResponse, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", string(status))
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/v1/tasks/"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	var resp ListTasksResponse
	if err := t.c.request(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Logs opens a live SSE stream of log lines for a task.
// Set follow=true to keep streaming until the task finishes.
func (t *TasksClient) Logs(ctx context.Context, taskID string, follow bool) (*LogStream, error) {
	q := url.Values{}
	if follow {
		q.Set("follow", "true")
	}
	path := "/api/v1/tasks/" + taskID + "/logs"
	if follow {
		path += "?" + q.Encode()
	}

	resp, err := t.c.rawRequest(ctx, http.MethodGet, path, nil,
		map[string]string{"Accept": "text/event-stream"})
	if err != nil {
		return nil, fmt.Errorf("task logs: %w", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, t.c.decodeError(resp)
	}
	return &LogStream{sse: newSSEReader(resp)}, nil
}

// Wait polls until the task reaches a terminal status or ctx is cancelled.
func (t *TasksClient) Wait(ctx context.Context, id string, interval time.Duration) (*Task, error) {
	if interval == 0 {
		interval = time.Second
	}
	for {
		task, err := t.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		switch task.Status {
		case TaskCompleted, TaskFailed, TaskDead, TaskCancelled:
			return task, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}
