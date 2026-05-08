-- Enable pgvector and create the unified vector store table
-- All embeddings live in one table, partitioned by collection.

CREATE EXTENSION IF NOT EXISTS vector;

-- ─── Vectors table (used by all RAG/memory/knowledge collections) ─────────────
CREATE TABLE IF NOT EXISTS vectors (
    id         TEXT PRIMARY KEY,
    collection TEXT NOT NULL,
    content    TEXT NOT NULL,
    embedding  vector(1536) NOT NULL,  -- OpenAI text-embedding-3-small dimensions
    metadata   JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vectors_collection ON vectors(collection);

-- HNSW index for cosine similarity (best recall/speed tradeoff for 1536-dim)
-- Tune m=16 and ef_construction=64 based on your dataset size
CREATE INDEX idx_vectors_embedding_hnsw ON vectors
USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);

-- ─── Vector collections reference ────────────────────────────────────────────
-- collection='memory'    → episodic/semantic memories
-- collection='documents' → document chunks for RAG
-- collection='knowledge' → knowledge graph node embeddings
-- collection='agents'    → agent context embeddings

COMMENT ON TABLE vectors IS
  'Unified vector store for all embedding-based retrieval. '
  'Collection partitions logical stores; HNSW index provides ANN search.';

-- ─── Materialized view for collection statistics ──────────────────────────────
CREATE MATERIALIZED VIEW IF NOT EXISTS vector_collection_stats AS
SELECT
    collection,
    COUNT(*) AS count,
    MIN(created_at) AS oldest,
    MAX(created_at) AS newest
FROM vectors
GROUP BY collection;

CREATE UNIQUE INDEX ON vector_collection_stats(collection);
