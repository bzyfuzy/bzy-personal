package client

import (
	"context"
	"fmt"
	"net/http"
)

// ─── Types ────────────────────────────────────────────────────────────────────

type IngestRequest struct {
	Title         string         `json:"title"`
	Content       string         `json:"content"`
	Source        string         `json:"source,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	ChunkStrategy string         `json:"chunk_strategy,omitempty"` // fixed | sentence | paragraph | semantic
	ChunkSize     int            `json:"chunk_size,omitempty"`
	ChunkOverlap  int            `json:"chunk_overlap,omitempty"`
}

type IngestResponse struct {
	DocumentID string `json:"document_id"`
	ChunkCount int    `json:"chunk_count"`
}

type RAGQueryRequest struct {
	Query      string    `json:"query"`
	Collection string    `json:"collection,omitempty"` // default: "documents"
	TopK       int       `json:"top_k,omitempty"`
	History    []Message `json:"history,omitempty"`
}

type RAGChunk struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	Score    float32        `json:"score"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type RAGQueryResponse struct {
	Response  string     `json:"response"`
	Chunks    []RAGChunk `json:"chunks"`
	LatencyMs int64      `json:"latency_ms"`
}

// ─── DocumentsClient ─────────────────────────────────────────────────────────

type DocumentsClient struct{ c *Client }

// Ingest uploads a document for chunking, embedding, and RAG indexing.
func (d *DocumentsClient) Ingest(ctx context.Context, req IngestRequest) (*IngestResponse, error) {
	var resp IngestResponse
	if err := d.c.request(ctx, http.MethodPost, "/api/v1/documents/", req, &resp); err != nil {
		return nil, fmt.Errorf("ingest: %w", err)
	}
	return &resp, nil
}

// Query runs a full RAG query: retrieve relevant chunks, then generate a response.
func (d *DocumentsClient) Query(ctx context.Context, req RAGQueryRequest) (*RAGQueryResponse, error) {
	var resp RAGQueryResponse
	if err := d.c.request(ctx, http.MethodPost, "/api/v1/documents/query", req, &resp); err != nil {
		return nil, fmt.Errorf("rag query: %w", err)
	}
	return &resp, nil
}

// Delete removes a document and all its indexed chunks.
func (d *DocumentsClient) Delete(ctx context.Context, documentID string) error {
	return d.c.request(ctx, http.MethodDelete, "/api/v1/documents/"+documentID, nil, nil)
}

// Ask is a shortcut for Query with a plain text question.
func (d *DocumentsClient) Ask(ctx context.Context, question string) (string, error) {
	resp, err := d.Query(ctx, RAGQueryRequest{Query: question})
	if err != nil {
		return "", err
	}
	return resp.Response, nil
}
