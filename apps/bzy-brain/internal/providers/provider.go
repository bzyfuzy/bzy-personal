// Package providers defines the AI provider abstraction.
// Concrete implementations: openai, ollama, anthropic, vllm.
// All services use this interface — swapping providers requires zero business-logic changes.
package providers

import (
	"context"
	"io"
)

// ─── Core types ────────────────────────────────────────────────────────────────

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role        `json:"role"`
	Content    string      `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type ChatRequest struct {
	Model       string      `json:"model"`
	Messages    []Message   `json:"messages"`
	Tools       []ToolSpec  `json:"tools,omitempty"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Temperature float64     `json:"temperature,omitempty"`
	Stream      bool        `json:"stream,omitempty"`
}

type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  map[string]any  `json:"parameters"` // JSON Schema
}

type ChatResponse struct {
	ID      string    `json:"id"`
	Model   string    `json:"model"`
	Message Message   `json:"message"`
	Usage   Usage     `json:"usage"`
	Finish  string    `json:"finish_reason"` // stop | tool_calls | length
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type StreamChunk struct {
	Delta   string `json:"delta"`
	Done    bool   `json:"done"`
	Finish  string `json:"finish_reason,omitempty"`
}

// ─── Provider interfaces ───────────────────────────────────────────────────────

// ChatProvider handles conversational completions.
type ChatProvider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (StreamReader, error)
	ModelID() string
}

// EmbeddingProvider generates vector embeddings.
type EmbeddingProvider interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
}

// Provider combines all capabilities a backend may offer.
type Provider interface {
	ID() string
	ChatProvider
	EmbeddingProvider
	HealthCheck(ctx context.Context) error
}

// StreamReader iterates streaming chunks.
type StreamReader interface {
	io.Closer
	Next() (*StreamChunk, error)
}

// ─── Registry ─────────────────────────────────────────────────────────────────

// Registry manages available AI providers and routes requests.
type Registry interface {
	Register(p Provider)
	Get(id string) (Provider, bool)
	Default() Provider
	SetDefault(id string) error
}

type providerRegistry struct {
	providers  map[string]Provider
	defaultID  string
}

func NewRegistry() Registry {
	return &providerRegistry{providers: make(map[string]Provider)}
}

func (r *providerRegistry) Register(p Provider) {
	r.providers[p.ID()] = p
	if r.defaultID == "" {
		r.defaultID = p.ID()
	}
}

func (r *providerRegistry) Get(id string) (Provider, bool) {
	p, ok := r.providers[id]
	return p, ok
}

func (r *providerRegistry) Default() Provider {
	return r.providers[r.defaultID]
}

func (r *providerRegistry) SetDefault(id string) error {
	if _, ok := r.providers[id]; !ok {
		return &ProviderNotFoundError{ID: id}
	}
	r.defaultID = id
	return nil
}

type ProviderNotFoundError struct{ ID string }

func (e *ProviderNotFoundError) Error() string { return "provider not found: " + e.ID }
