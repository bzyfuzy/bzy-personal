// Package streaming implements Server-Sent Events (SSE) for streaming AI responses.
package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"go.uber.org/zap"
)

// Event is a single SSE payload.
type Event struct {
	ID    string `json:"id,omitempty"`
	Event string `json:"event,omitempty"` // e.g. "delta", "done", "error"
	Data  any    `json:"data"`
}

// Client represents a connected SSE subscriber.
type client struct {
	id      string
	topic   string
	channel chan Event
}

// Hub manages all active SSE connections.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[string]*client // topic → clientID → client
	logger  *zap.Logger
}

func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients: make(map[string]map[string]*client),
		logger:  logger,
	}
}

// Subscribe adds a client to a topic. Returns a channel to receive events.
func (h *Hub) Subscribe(clientID, topic string) chan Event {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan Event, 32)
	if h.clients[topic] == nil {
		h.clients[topic] = make(map[string]*client)
	}
	h.clients[topic][clientID] = &client{id: clientID, topic: topic, channel: ch}
	return ch
}

// Unsubscribe removes a client from a topic.
func (h *Hub) Unsubscribe(clientID, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[topic]; ok {
		if c, ok := clients[clientID]; ok {
			close(c.channel)
			delete(clients, clientID)
		}
	}
}

// Publish sends an event to all subscribers of a topic.
func (h *Hub) Publish(topic string, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, c := range h.clients[topic] {
		select {
		case c.channel <- event:
		default:
			h.logger.Warn("SSE client slow — dropping event", zap.String("client", c.id))
		}
	}
}

// ServeSSE writes events to an HTTP response as SSE stream.
// The handler blocks until ctx is cancelled or the stream completes.
func ServeSSE(ctx context.Context, w http.ResponseWriter, events <-chan Event) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-events:
			if !ok {
				fmt.Fprintf(w, "event: done\ndata: {}\n\n")
				flusher.Flush()
				return nil
			}
			if err := writeSSEEvent(w, event); err != nil {
				return err
			}
			flusher.Flush()
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, event Event) error {
	if event.Event != "" {
		fmt.Fprintf(w, "event: %s\n", event.Event)
	}
	if event.ID != "" {
		fmt.Fprintf(w, "id: %s\n", event.ID)
	}
	data, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	return nil
}
