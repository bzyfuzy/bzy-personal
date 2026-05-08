// Package events provides the platform event bus abstraction.
// Implementations: Redis Streams (default), NATS JetStream (recommended for prod).
package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ─── Core types ────────────────────────────────────────────────────────────────

// Event is the platform-wide envelope for all async messages.
type Event struct {
	ID        string          `json:"id"`
	Type      EventType       `json:"type"`
	Source    string          `json:"source"`    // originating service
	Subject   string          `json:"subject"`   // e.g. user ID / task ID
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	TraceID   string          `json:"trace_id,omitempty"`
}

type EventType string

// Platform-wide event taxonomy.
const (
	// Memory domain
	EventMemoryCreated EventType = "memory.created"
	EventMemoryUpdated EventType = "memory.updated"
	EventMemoryDeleted EventType = "memory.deleted"

	// Task / execution domain
	EventTaskCreated   EventType = "task.created"
	EventTaskStarted   EventType = "task.started"
	EventTaskCompleted EventType = "task.completed"
	EventTaskFailed    EventType = "task.failed"
	EventTaskCancelled EventType = "task.cancelled"

	// Agent domain
	EventAgentSpawned    EventType = "agent.spawned"
	EventAgentResponsed  EventType = "agent.responded"
	EventAgentTerminated EventType = "agent.terminated"

	// Workflow domain
	EventWorkflowStarted   EventType = "workflow.started"
	EventWorkflowCompleted EventType = "workflow.completed"
	EventWorkflowFailed    EventType = "workflow.failed"

	// Document / RAG domain
	EventDocumentIngested EventType = "document.ingested"
	EventDocumentIndexed  EventType = "document.indexed"

	// Worker domain
	EventWorkerRegistered   EventType = "worker.registered"
	EventWorkerHeartbeat    EventType = "worker.heartbeat"
	EventWorkerDeregistered EventType = "worker.deregistered"
)

// ─── Interfaces ────────────────────────────────────────────────────────────────

// Publisher sends events onto the bus.
type Publisher interface {
	Publish(ctx context.Context, event *Event) error
	// PublishBatch publishes multiple events atomically where supported.
	PublishBatch(ctx context.Context, events []*Event) error
}

// Subscriber receives events from the bus.
type Subscriber interface {
	// Subscribe registers a handler for a given event type (supports wildcards).
	Subscribe(ctx context.Context, topic string, handler Handler) (Subscription, error)
}

// Bus combines both sides.
type Bus interface {
	Publisher
	Subscriber
	Close() error
}

// Handler processes a received event. Return non-nil error to trigger retry/DLQ.
type Handler func(ctx context.Context, event *Event) error

// Subscription represents an active subscription.
type Subscription interface {
	Unsubscribe() error
	Topic() string
}

// ─── Factory helpers ───────────────────────────────────────────────────────────

func NewEvent(eventType EventType, source, subject string, payload any) (*Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Source:    source,
		Subject:   subject,
		Payload:   raw,
		Timestamp: time.Now().UTC(),
	}, nil
}

func DecodePayload[T any](e *Event) (T, error) {
	var v T
	return v, json.Unmarshal(e.Payload, &v)
}

// ─── No-op (testing) ───────────────────────────────────────────────────────────

type NoopBus struct{}

func (n *NoopBus) Publish(_ context.Context, _ *Event) error              { return nil }
func (n *NoopBus) PublishBatch(_ context.Context, _ []*Event) error       { return nil }
func (n *NoopBus) Subscribe(_ context.Context, _ string, _ Handler) (Subscription, error) {
	return &noopSub{}, nil
}
func (n *NoopBus) Close() error { return nil }

type noopSub struct{ topic string }

func (s *noopSub) Unsubscribe() error { return nil }
func (s *noopSub) Topic() string      { return s.topic }
