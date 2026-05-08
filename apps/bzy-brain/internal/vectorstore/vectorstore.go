// Package vectorstore defines the vector store abstraction and pgvector implementation.
package vectorstore

import (
	"context"
	"time"
)

// ─── Domain types ──────────────────────────────────────────────────────────────

// Vector is a stored embedding with metadata.
type Vector struct {
	ID         string         `json:"id"`
	Collection string         `json:"collection"`
	Content    string         `json:"content"`
	Embedding  []float32      `json:"embedding"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Score      float32        `json:"score,omitempty"` // populated on search
	CreatedAt  time.Time      `json:"created_at"`
}

// SearchQuery specifies retrieval parameters.
type SearchQuery struct {
	Collection string
	Embedding  []float32
	TopK       int
	MinScore   float32
	Filters    map[string]any // metadata equality filters
}

// ─── Store interface ───────────────────────────────────────────────────────────

// Store is the platform's vector persistence layer.
type Store interface {
	// Upsert inserts or updates a vector. Uses ID for idempotency.
	Upsert(ctx context.Context, v *Vector) error
	// UpsertBatch is the bulk equivalent.
	UpsertBatch(ctx context.Context, vectors []*Vector) error
	// Search performs ANN search over a collection.
	Search(ctx context.Context, q SearchQuery) ([]*Vector, error)
	// Delete removes a vector by ID.
	Delete(ctx context.Context, id string) error
	// DeleteCollection removes all vectors in a collection.
	DeleteCollection(ctx context.Context, collection string) error
	// Get retrieves a single vector by ID.
	Get(ctx context.Context, id string) (*Vector, error)
}

// Standard collection names used across the platform.
const (
	CollectionMemory    = "memory"
	CollectionDocuments = "documents"
	CollectionAgents    = "agents"
	CollectionKnowledge = "knowledge"
)
