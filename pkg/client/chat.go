package client

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ─── Types ────────────────────────────────────────────────────────────────────

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	SessionID   string    `json:"session_id,omitempty"`
	Messages    []Message `json:"messages"`
	Model       string    `json:"model,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type ChatResponse struct {
	ID      string  `json:"id"`
	Model   string  `json:"model"`
	Message Message `json:"message"`
	Usage   Usage   `json:"usage"`
	Finish  string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatDelta is a single SSE delta chunk during streaming.
type ChatDelta struct {
	Delta  string `json:"delta"`
	Done   bool   `json:"done"`
	Finish string `json:"finish_reason,omitempty"`
}

// Session represents a conversation session.
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateSessionRequest struct {
	Title string `json:"title,omitempty"`
}

// ChatStream is a live streaming response.
type ChatStream struct {
	sse    *SSEReader
	Delta  chan ChatDelta
	Err    chan error
	done   chan struct{}
}

// Text collects the full streamed text, blocking until the stream is complete.
func (s *ChatStream) Text(ctx context.Context) (string, error) {
	var full string
	err := s.Iter(ctx, func(d ChatDelta) error {
		full += d.Delta
		return nil
	})
	return full, err
}

// Iter calls fn for each delta until the stream ends or ctx is cancelled.
func (s *ChatStream) Iter(ctx context.Context, fn func(ChatDelta) error) error {
	return s.sse.Iter(ctx, func(e *SSEEvent) error {
		var d ChatDelta
		if err := e.DecodeData(&d); err != nil {
			return err
		}
		return fn(d)
	})
}

// Close releases the stream.
func (s *ChatStream) Close() { s.sse.Close() }

// ─── ChatClient ───────────────────────────────────────────────────────────────

type ChatClient struct{ c *Client }

// Complete sends a chat completion request and returns the full response.
func (cc *ChatClient) Complete(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req.Stream = false
	var resp ChatResponse
	if err := cc.c.request(ctx, http.MethodPost, "/api/v1/chat/completions", req, &resp); err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	return &resp, nil
}

// Stream opens a Server-Sent Events stream for incremental AI responses.
// The caller must call ChatStream.Close() or read until completion.
func (cc *ChatClient) Stream(ctx context.Context, req ChatRequest) (*ChatStream, error) {
	req.Stream = true
	resp, err := cc.c.rawRequest(ctx, http.MethodGet, "/api/v1/chat/stream", req,
		map[string]string{"Accept": "text/event-stream"})
	if err != nil {
		return nil, fmt.Errorf("chat stream: %w", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, cc.c.decodeError(resp)
	}
	return &ChatStream{sse: newSSEReader(resp)}, nil
}

// CreateSession creates a new conversation session.
func (cc *ChatClient) CreateSession(ctx context.Context, title string) (*Session, error) {
	var s Session
	if err := cc.c.request(ctx, http.MethodPost, "/api/v1/chat/sessions",
		CreateSessionRequest{Title: title}, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// GetSession retrieves a session and its message history.
func (cc *ChatClient) GetSession(ctx context.Context, id string) (*Session, error) {
	var s Session
	if err := cc.c.request(ctx, http.MethodGet, "/api/v1/chat/sessions/"+id, nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Ask is a convenience wrapper: create/use session, send one message, return full text.
func (cc *ChatClient) Ask(ctx context.Context, sessionID, question string) (string, error) {
	resp, err := cc.Complete(ctx, ChatRequest{
		SessionID: sessionID,
		Messages:  []Message{{Role: RoleUser, Content: question}},
	})
	if err != nil {
		return "", err
	}
	return resp.Message.Content, nil
}

// AskStream is Ask but returns a streaming response.
func (cc *ChatClient) AskStream(ctx context.Context, sessionID, question string) (*ChatStream, error) {
	return cc.Stream(ctx, ChatRequest{
		SessionID: sessionID,
		Messages:  []Message{{Role: RoleUser, Content: question}},
	})
}
