-- Brain database initial schema
-- Run: migrate -path migrations/brain -database $DATABASE_URL up

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm"; -- trigram search for full-text

-- ─── Users (managed by interface, referenced by brain) ────────────────────────
CREATE TABLE IF NOT EXISTS users (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Sessions ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS sessions (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      TEXT,
    summary    TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_updated_at ON sessions(updated_at DESC);

-- ─── Messages ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS messages (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('system', 'user', 'assistant', 'tool')),
    content    TEXT NOT NULL,
    metadata   JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_session_id ON messages(session_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);

-- ─── Memories ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS memories (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES sessions(id) ON DELETE SET NULL,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT NOT NULL CHECK (type IN ('episodic', 'semantic', 'procedural', 'working')),
    content    TEXT NOT NULL,
    summary    TEXT,
    importance REAL NOT NULL DEFAULT 0.5 CHECK (importance BETWEEN 0 AND 1),
    metadata   JSONB,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_memories_user_id ON memories(user_id);
CREATE INDEX idx_memories_type ON memories(type);
CREATE INDEX idx_memories_importance ON memories(importance DESC);
CREATE INDEX idx_memories_created_at ON memories(created_at DESC);
CREATE INDEX idx_memories_content_trgm ON memories USING gin(content gin_trgm_ops);

-- ─── Documents ────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS documents (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    content    TEXT,
    source     TEXT,
    metadata   JSONB,
    chunk_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_documents_user_id ON documents(user_id);

-- ─── Agents ───────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS agent_defs (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name          TEXT NOT NULL,
    description   TEXT,
    system_prompt TEXT NOT NULL,
    model         TEXT,
    tool_ids      TEXT[] NOT NULL DEFAULT '{}',
    max_steps     INT NOT NULL DEFAULT 10,
    temperature   REAL NOT NULL DEFAULT 0.7,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_runs (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    def_id       UUID REFERENCES agent_defs(id),
    session_id   UUID REFERENCES sessions(id),
    user_id      UUID REFERENCES users(id),
    status       TEXT NOT NULL DEFAULT 'idle',
    input        TEXT,
    output       TEXT,
    step_count   INT NOT NULL DEFAULT 0,
    latency_ms   BIGINT,
    error        TEXT,
    spawned_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at  TIMESTAMPTZ
);

CREATE INDEX idx_agent_runs_user_id ON agent_runs(user_id);
CREATE INDEX idx_agent_runs_status ON agent_runs(status);

-- ─── Workflow definitions ─────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS workflow_defs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT NOT NULL,
    description TEXT,
    version     INT NOT NULL DEFAULT 1,
    definition  JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS workflow_runs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    def_id      UUID REFERENCES workflow_defs(id),
    status      TEXT NOT NULL DEFAULT 'pending',
    input       JSONB,
    output      JSONB,
    node_states JSONB,
    error       TEXT,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX idx_workflow_runs_status ON workflow_runs(status);
CREATE INDEX idx_workflow_runs_def_id ON workflow_runs(def_id);

-- ─── Knowledge graph ─────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS kg_nodes (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    label      TEXT NOT NULL,
    type       TEXT NOT NULL,
    properties JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS kg_edges (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id   UUID NOT NULL REFERENCES kg_nodes(id) ON DELETE CASCADE,
    target_id   UUID NOT NULL REFERENCES kg_nodes(id) ON DELETE CASCADE,
    relation    TEXT NOT NULL,
    weight      REAL NOT NULL DEFAULT 1.0,
    properties  JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_kg_edges_source ON kg_edges(source_id);
CREATE INDEX idx_kg_edges_target ON kg_edges(target_id);
CREATE INDEX idx_kg_edges_relation ON kg_edges(relation);
