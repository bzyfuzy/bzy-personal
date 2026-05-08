// Package rag implements the Retrieval-Augmented Generation pipeline.
//
// Pipeline stages:
//  1. Ingest  → chunk document → embed chunks → store in vectorstore
//  2. Retrieve → embed query → ANN search → rerank → return top-K chunks
//  3. Augment  → inject chunks into system prompt
//  4. Generate → call AI provider with augmented prompt
package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/embedding"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/providers"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/vectorstore"
)

// ─── Domain types ─────────────────────────────────────────────────────────────

type Document struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Content  string         `json:"content"`
	Source   string         `json:"source"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type Chunk struct {
	ID         string         `json:"id"`
	DocumentID string         `json:"document_id"`
	Content    string         `json:"content"`
	Index      int            `json:"index"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type RetrievedChunk struct {
	Chunk
	Score float32 `json:"score"`
}

type QueryResult struct {
	Chunks   []RetrievedChunk `json:"chunks"`
	Response string           `json:"response"`
	Latency  time.Duration    `json:"latency"`
}

// ─── Chunking strategies ──────────────────────────────────────────────────────

type ChunkStrategy string

const (
	ChunkFixed     ChunkStrategy = "fixed"     // fixed token window
	ChunkSentence  ChunkStrategy = "sentence"  // sentence-boundary aware
	ChunkParagraph ChunkStrategy = "paragraph" // paragraph breaks
	ChunkSemantic  ChunkStrategy = "semantic"  // embedding-similarity breaks
)

type ChunkerConfig struct {
	Strategy  ChunkStrategy
	ChunkSize int // tokens
	Overlap   int // tokens
}

// ─── Pipeline interface ───────────────────────────────────────────────────────

type Pipeline interface {
	// Ingest processes a document: chunk → embed → store.
	Ingest(ctx context.Context, doc *Document, cfg ChunkerConfig) error
	// Retrieve performs semantic search for a query.
	Retrieve(ctx context.Context, collection, query string, topK int) ([]RetrievedChunk, error)
	// Query is the full RAG flow: retrieve + augment + generate.
	Query(ctx context.Context, collection, query string, history []providers.Message) (*QueryResult, error)
	// DeleteDocument removes all chunks for a document.
	DeleteDocument(ctx context.Context, documentID string) error
}

// ─── Concrete pipeline ────────────────────────────────────────────────────────

type pipeline struct {
	embedder    *embedding.Service
	vecStore    vectorstore.Store
	chatProvider providers.ChatProvider
}

func NewPipeline(embedder *embedding.Service, store vectorstore.Store, reg providers.Registry) Pipeline {
	return &pipeline{
		embedder:     embedder,
		vecStore:     store,
		chatProvider: reg.Default(),
	}
}

func (p *pipeline) Ingest(ctx context.Context, doc *Document, cfg ChunkerConfig) error {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}

	chunks := chunkText(doc.Content, cfg)
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c
	}

	embeddings, err := p.embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("rag ingest embed: %w", err)
	}

	vectors := make([]*vectorstore.Vector, len(chunks))
	for i, chunk := range chunks {
		vectors[i] = &vectorstore.Vector{
			ID:         fmt.Sprintf("%s-chunk-%d", doc.ID, i),
			Collection: vectorstore.CollectionDocuments,
			Content:    chunk,
			Embedding:  embeddings[i],
			Metadata: map[string]any{
				"document_id": doc.ID,
				"title":       doc.Title,
				"source":      doc.Source,
				"chunk_index": i,
			},
		}
	}
	return p.vecStore.UpsertBatch(ctx, vectors)
}

func (p *pipeline) Retrieve(ctx context.Context, collection, query string, topK int) ([]RetrievedChunk, error) {
	queryVec, err := p.embedder.EmbedOne(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("rag retrieve embed: %w", err)
	}

	results, err := p.vecStore.Search(ctx, vectorstore.SearchQuery{
		Collection: collection,
		Embedding:  queryVec,
		TopK:       topK,
		MinScore:   0.65,
	})
	if err != nil {
		return nil, fmt.Errorf("rag retrieve search: %w", err)
	}

	chunks := make([]RetrievedChunk, len(results))
	for i, r := range results {
		chunks[i] = RetrievedChunk{
			Chunk: Chunk{
				ID:       r.ID,
				Content:  r.Content,
				Metadata: r.Metadata,
			},
			Score: r.Score,
		}
	}
	return chunks, nil
}

func (p *pipeline) Query(ctx context.Context, collection, query string, history []providers.Message) (*QueryResult, error) {
	start := time.Now()

	chunks, err := p.Retrieve(ctx, collection, query, 5)
	if err != nil {
		return nil, err
	}

	context := buildContextPrompt(chunks)

	messages := []providers.Message{
		{Role: providers.RoleSystem, Content: ragSystemPrompt(context)},
	}
	messages = append(messages, history...)
	messages = append(messages, providers.Message{Role: providers.RoleUser, Content: query})

	resp, err := p.chatProvider.Chat(ctx, providers.ChatRequest{
		Messages:    messages,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("rag generate: %w", err)
	}

	return &QueryResult{
		Chunks:   chunks,
		Response: resp.Message.Content,
		Latency:  time.Since(start),
	}, nil
}

func (p *pipeline) DeleteDocument(ctx context.Context, documentID string) error {
	// Real impl: query all chunk IDs for documentID then delete
	return nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func chunkText(text string, cfg ChunkerConfig) []string {
	// Simple paragraph-split fallback — replace with proper chunker for production
	parts := strings.Split(text, "\n\n")
	var chunks []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			chunks = append(chunks, p)
		}
	}
	return chunks
}

func buildContextPrompt(chunks []RetrievedChunk) string {
	var sb strings.Builder
	for i, c := range chunks {
		fmt.Fprintf(&sb, "[%d] (score: %.2f)\n%s\n\n", i+1, c.Score, c.Content)
	}
	return sb.String()
}

func ragSystemPrompt(context string) string {
	return fmt.Sprintf(`You are a helpful assistant. Use the following retrieved context to answer questions accurately.
If the context doesn't contain enough information, say so.

CONTEXT:
%s`, context)
}
