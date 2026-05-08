-- Runner database schema for task persistence and audit trail

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ─── Tasks ────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tasks (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type         TEXT NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}',
    priority     SMALLINT NOT NULL DEFAULT 5,
    status       TEXT NOT NULL DEFAULT 'queued'
                 CHECK (status IN ('queued','processing','completed','failed','retrying','dead','cancelled')),
    attempt      INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    timeout_sec  INT NOT NULL DEFAULT 300,
    trace_id     TEXT,
    worker_id    TEXT,
    error        TEXT,
    result       JSONB,
    run_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- earliest execution time
    queued_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at   TIMESTAMPTZ,
    finished_at  TIMESTAMPTZ
);

CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_type ON tasks(type);
CREATE INDEX idx_tasks_run_at ON tasks(run_at) WHERE status = 'queued';
CREATE INDEX idx_tasks_worker_id ON tasks(worker_id) WHERE worker_id IS NOT NULL;
CREATE INDEX idx_tasks_trace_id ON tasks(trace_id) WHERE trace_id IS NOT NULL;

-- ─── Scheduled jobs ───────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS scheduled_jobs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT NOT NULL UNIQUE,
    cron_expr   TEXT NOT NULL,
    task_type   TEXT NOT NULL,
    payload     JSONB NOT NULL DEFAULT '{}',
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Workers ──────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS workers (
    id           TEXT PRIMARY KEY,
    hostname     TEXT NOT NULL,
    addr         TEXT,
    concurrency  INT NOT NULL DEFAULT 1,
    active_tasks INT NOT NULL DEFAULT 0,
    version      TEXT,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Workflow definitions ─────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS workflow_defs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT NOT NULL,
    definition  JSONB NOT NULL,
    version     INT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Workflow runs ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS workflow_runs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    def_id      UUID NOT NULL REFERENCES workflow_defs(id),
    status      TEXT NOT NULL DEFAULT 'pending',
    input       JSONB,
    output      JSONB,
    node_states JSONB NOT NULL DEFAULT '{}',
    error       TEXT,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX idx_workflow_runs_status ON workflow_runs(status);
CREATE INDEX idx_workflow_runs_def_id ON workflow_runs(def_id);
CREATE INDEX idx_workflow_runs_started ON workflow_runs(started_at DESC);

-- ─── Task logs (persistent audit trail, complements Redis stream) ─────────────
CREATE TABLE IF NOT EXISTS task_logs (
    id         BIGSERIAL PRIMARY KEY,
    task_id    UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    worker_id  TEXT,
    level      TEXT NOT NULL DEFAULT 'INFO',
    message    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_logs_task_id ON task_logs(task_id);
CREATE INDEX idx_task_logs_created_at ON task_logs(created_at DESC);

-- ─── Dead letter queue view ───────────────────────────────────────────────────
CREATE VIEW dead_letter_queue AS
SELECT *
FROM tasks
WHERE status = 'dead'
ORDER BY finished_at DESC;
