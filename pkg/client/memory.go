package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ─── Types ────────────────────────────────────────────────────────────────────

type MemoryType string

const (
	MemoryEpisodic   MemoryType = "episodic"
	MemorySemantic   MemoryType = "semantic"
	MemoryProcedural MemoryType = "procedural"
	MemoryWorking    MemoryType = "working"
)

type Memory struct {
	ID         string         `json:"id"`
	SessionID  string         `json:"session_id,omitempty"`
	UserID     string         `json:"user_id"`
	Type       MemoryType     `json:"type"`
	Content    string         `json:"content"`
	Summary    string         `json:"summary,omitempty"`
	Importance float32        `json:"importance"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type StoreMemoryRequest struct {
	SessionID  string         `json:"session_id,omitempty"`
	Type       MemoryType     `json:"type"`
	Content    string         `json:"content"`
	Importance float32        `json:"importance,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type StoreMemoryResponse struct {
	ID string `json:"id"`
}

type RecallResult struct {
	Memory *Memory `json:"memory"`
	Score  float32 `json:"score"`
}

type RecallResponse struct {
	Results []RecallResult `json:"results"`
}

// ─── MemoryClient ─────────────────────────────────────────────────────────────

type MemoryClient struct{ c *Client }

// Store saves a new memory. Returns the generated ID.
func (m *MemoryClient) Store(ctx context.Context, req StoreMemoryRequest) (string, error) {
	var resp StoreMemoryResponse
	if err := m.c.request(ctx, http.MethodPost, "/api/v1/memory/", req, &resp); err != nil {
		return "", fmt.Errorf("memory store: %w", err)
	}
	return resp.ID, nil
}

// Recall performs semantic search over memories for a query.
// topK controls how many results are returned (default 10 server-side).
// minScore filters results below the threshold (0–1).
func (m *MemoryClient) Recall(ctx context.Context, query string, topK int, minScore float32) ([]RecallResult, error) {
	q := url.Values{}
	q.Set("q", query)
	if topK > 0 {
		q.Set("top_k", strconv.Itoa(topK))
	}
	if minScore > 0 {
		q.Set("min_score", strconv.FormatFloat(float64(minScore), 'f', 2, 32))
	}

	var resp RecallResponse
	if err := m.c.request(ctx, http.MethodGet,
		"/api/v1/memory/recall?"+q.Encode(), nil, &resp); err != nil {
		return nil, fmt.Errorf("memory recall: %w", err)
	}
	return resp.Results, nil
}

// Forget deletes a memory by ID.
func (m *MemoryClient) Forget(ctx context.Context, id string) error {
	return m.c.request(ctx, http.MethodDelete, "/api/v1/memory/"+id, nil, nil)
}
