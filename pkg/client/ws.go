package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ─── Wire types ───────────────────────────────────────────────────────────────

type WSMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Known server-pushed message types.
const (
	WSTypeTaskUpdate     = "task_update"
	WSTypeAgentResponse  = "agent_response"
	WSTypeMemoryUpdate   = "memory_update"
	WSTypeWorkflowUpdate = "workflow_update"
	WSTypePing           = "ping"
	WSTypePong           = "pong"
)

// ─── WSConn ───────────────────────────────────────────────────────────────────

// WSConn is a live WebSocket connection to bzy-interface.
type WSConn struct {
	conn     *websocket.Conn
	mu       sync.Mutex
	handlers map[string][]WSHandler
	done     chan struct{}
	once     sync.Once
}

// WSHandler is called when a message of the registered type arrives.
type WSHandler func(msg WSMessage) error

// Connect opens a WebSocket connection to the /ws endpoint.
// The connection is authenticated via the client's current token.
func (c *Client) Connect(ctx context.Context) (*WSConn, error) {
	rawURL := c.cfg.BaseURL
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	// Switch scheme http→ws, https→wss
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	u.Path = "/ws"

	headers := http.Header{}
	headers.Set("User-Agent", c.cfg.UserAgent)
	if c.cfg.APIKey != "" {
		headers.Set("X-API-Key", c.cfg.APIKey)
	} else if tok := c.Token(); tok != "" {
		headers.Set("Authorization", "Bearer "+tok)
	}

	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		return nil, fmt.Errorf("ws connect: %w", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("ws: unexpected status %d", resp.StatusCode)
	}

	wsc := &WSConn{
		conn:     conn,
		handlers: make(map[string][]WSHandler),
		done:     make(chan struct{}),
	}
	go wsc.readLoop()
	go wsc.pingLoop()
	return wsc, nil
}

// On registers a handler for a specific message type.
// Multiple handlers for the same type are called in registration order.
func (wsc *WSConn) On(msgType string, h WSHandler) {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()
	wsc.handlers[msgType] = append(wsc.handlers[msgType], h)
}

// Send writes a message to the server.
func (wsc *WSConn) Send(msg WSMessage) error {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()
	return wsc.conn.WriteJSON(msg)
}

// SendJSON marshals payload and sends a typed message.
func (wsc *WSConn) SendJSON(msgType string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return wsc.Send(WSMessage{Type: msgType, Payload: raw})
}

// Close shuts down the WebSocket connection.
func (wsc *WSConn) Close() error {
	wsc.once.Do(func() { close(wsc.done) })
	return wsc.conn.Close()
}

// Done returns a channel that is closed when the connection is terminated.
func (wsc *WSConn) Done() <-chan struct{} { return wsc.done }

func (wsc *WSConn) readLoop() {
	defer wsc.once.Do(func() { close(wsc.done) })
	for {
		var msg WSMessage
		if err := wsc.conn.ReadJSON(&msg); err != nil {
			return // connection closed or read error
		}
		wsc.dispatch(msg)
	}
}

func (wsc *WSConn) dispatch(msg WSMessage) {
	wsc.mu.Lock()
	handlers := append([]WSHandler{}, wsc.handlers[msg.Type]...)
	wsc.mu.Unlock()

	for _, h := range handlers {
		_ = h(msg) // errors are silently dropped; use On() error handling in caller
	}
}

func (wsc *WSConn) pingLoop() {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-wsc.done:
			return
		case <-ticker.C:
			_ = wsc.Send(WSMessage{Type: WSTypePing})
		}
	}
}

// DecodePayload unmarshals a WSMessage's Payload into v.
func (m *WSMessage) DecodePayload(v any) error {
	return json.Unmarshal(m.Payload, v)
}
