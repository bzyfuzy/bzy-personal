# bzy-personal Architecture

## Overview

`bzy-personal` is a self-hosted personal AI operating system built as a **Go monorepo** with
three independently deployable services communicating via gRPC and an async event bus.

```
┌─────────────────────────────────────────────────────┐
│                   bzy-interface                     │
│    REST API  |  WebSocket  |  SSE  |  gRPC proxy    │
└───────────┬───────────────────────────┬─────────────┘
            │ gRPC                      │ gRPC
            ▼                           ▼
┌───────────────────────┐   ┌───────────────────────────┐
│      bzy-brain        │   │       bzy-runner          │
│  Memory | RAG         │   │  Tasks | Workflows        │
│  Reasoning | Agents   │   │  Scheduler | Executor     │
│  Embedding | Planning │   │  Worker Pool | Cluster    │
└───────────┬───────────┘   └───────────────────────────┘
            │                           │
            └─────────┬─────────────────┘
                      │  Events (Redis Streams / NATS)
            ┌─────────▼──────────────────────────────┐
            │           Shared Infrastructure        │
            │  PostgreSQL+pgvector  Redis  NATS      │
            │  OpenTelemetry  Prometheus  Jaeger      │
            └────────────────────────────────────────┘
```

---

## 1. Monorepo Structure

```
bzy-personal/
├── apps/
│   ├── bzy-brain/           # AI memory, reasoning, RAG service (gRPC :50051)
│   │   ├── cmd/main.go      # fx-wired entrypoint
│   │   ├── config/          # typed service config
│   │   └── internal/
│   │       ├── agents/      # agent registry & live execution
│   │       ├── embedding/   # cached embedding service
│   │       ├── memory/      # memory store + session management
│   │       ├── planning/    # LLM-based goal decomposition
│   │       ├── providers/   # AI provider abstraction (OpenAI, Ollama...)
│   │       ├── rag/         # ingest→chunk→embed→retrieve→augment→generate
│   │       ├── reasoning/   # ReAct / CoT / ToT reasoning engines
│   │       ├── server/      # gRPC server setup
│   │       ├── tools/       # tool registry + executor
│   │       ├── vectorstore/ # pgvector store
│   │       └── workflows/   # DAG workflow engine
│   │
│   ├── bzy-runner/          # Distributed task execution (gRPC :50052)
│   │   ├── cmd/main.go
│   │   ├── config/
│   │   └── internal/
│   │       ├── cluster/     # node discovery & capacity tracking
│   │       ├── executor/    # local + Docker sandboxed execution
│   │       ├── heartbeat/   # node liveness via Redis TTL
│   │       ├── locks/       # Redis distributed locks
│   │       ├── logstream/   # Redis Streams task log pipeline
│   │       ├── queue/       # Redis Streams task queue
│   │       ├── scheduler/   # cron-based workflow scheduling
│   │       └── worker/      # concurrent worker pool
│   │
│   └── bzy-interface/       # API gateway (HTTP :8080)
│       ├── cmd/main.go
│       ├── config/
│       └── internal/
│           ├── auth/        # JWT + API key authentication
│           ├── gateway/     # typed gRPC clients for brain/runner
│           ├── httpserver/  # Gin router + all route handlers
│           ├── middleware/  # auth, rate limiting, CORS, tracing
│           ├── session/     # Redis-backed session management
│           ├── streaming/   # SSE hub for AI response streaming
│           └── ws/          # WebSocket hub (realtime bidirectional)
│
├── pkg/                     # Shared SDK (imported by all services)
│   ├── config/              # typed env-based config loader
│   ├── errors/              # domain error types with HTTP codes
│   ├── events/              # event bus interface + NoopBus
│   ├── health/              # composable health check system
│   ├── logging/             # zap logger factory
│   ├── plugin/              # plugin SDK (manifest + executor)
│   └── telemetry/           # OTel tracer provider bootstrap
│
├── proto/                   # Protobuf contracts
│   ├── brain/v1/brain.proto
│   └── runner/v1/runner.proto
│
├── migrations/              # SQL migrations (golang-migrate)
│   ├── brain/               # 001_init, 002_pgvector, 003_kg
│   └── runner/              # 001_init
│
├── deploy/
│   ├── docker/              # docker-compose.yml + docker-compose.dev.yml
│   └── k8s/base/            # Kustomize base manifests
│
├── scripts/                 # setup.sh, proto-gen.sh
├── .github/workflows/       # CI/CD
├── go.work                  # Go workspace linking all modules
└── Makefile                 # build/test/lint/docker/k8s targets
```

---

## 2. Service Communication Design

### Synchronous (gRPC)
- `bzy-interface` → `bzy-brain`: all AI operations (chat, memory, RAG, agents)
- `bzy-interface` → `bzy-runner`: task management (enqueue, status, logs, cancel)
- Services do NOT call each other's HTTP APIs — gRPC only for internal IPC

### Asynchronous (Event Bus)
- **Redis Streams** (MVP): durable, consumer groups, replay, low ops overhead
- **NATS JetStream** (recommended production): lower latency, built-in load balancing, push/pull

### Streaming
- AI responses: SSE from interface to browser (Server-Sent Events)
- Task logs: Redis Pub/Sub → SSE fan-out via Hub
- Realtime updates: WebSocket for bidirectional (task events, agent updates)

### Request tracing
Every request carries an OpenTelemetry TraceID propagated via gRPC metadata
and HTTP headers (`traceparent`). End-to-end traces visible in Jaeger UI.

---

## 3. Event Bus Design

```
Producer                  Redis Streams / NATS                Consumer
───────                   ──────────────────────              ────────
brain → memory.created    bzy:events:memory    → runner/brain
runner → task.completed   bzy:events:tasks     → brain (update memory)
brain → agent.responded   bzy:events:agents    → interface (SSE push)
runner → worker.heartbeat bzy:events:workers   → cluster registry
```

**Event envelope** (pkg/events):
```go
type Event struct {
    ID        string
    Type      EventType    // e.g. "task.completed"
    Source    string       // originating service
    Subject   string       // resource ID (task ID, user ID, etc.)
    Payload   json.RawMessage
    Timestamp time.Time
    TraceID   string
}
```

**Consumer groups** ensure exactly-once delivery per consumer.
Failed events are retried with exponential backoff then moved to DLQ.

---

## 4. AI Memory Architecture

Three memory layers inspired by cognitive science:

```
Working Memory (Redis, TTL-based)
    ↓ consolidation trigger (N messages or time)
Episodic Memory (PostgreSQL + pgvector)
    ↓ semantic distillation
Semantic Memory (PostgreSQL + pgvector, high importance)
```

**Recall flow:**
1. Query arrives with user context
2. Embed query with OpenAI `text-embedding-3-small`
3. ANN search over `vectors` table, collection=`memory`
4. Cosine similarity filter (score ≥ 0.65)
5. Rerank by (score × importance × recency_decay)
6. Inject top-K into system prompt context window

**Session management:**
- Session = ordered message list + rolling summary
- When `len(messages) > threshold (20)`, trigger summary via reasoning engine
- Summary replaces old messages, preserving key facts

---

## 5. RAG Pipeline Design

```
Document In
    ↓
[Chunker] → fixed/sentence/paragraph/semantic chunks
    ↓
[Embedder] → OpenAI text-embedding-3-small (with Redis cache)
    ↓
[VectorStore.UpsertBatch] → pgvector table, collection=documents
    ↓
[Query time]
    ↓
[Embedder] → embed user query
    ↓
[VectorStore.Search] → HNSW cosine ANN, top-K, min score 0.65
    ↓
[Reranker] → cross-encoder reranking (future: Cohere/Jina)
    ↓
[Context Builder] → inject chunks into system prompt
    ↓
[AI Provider] → generate answer with augmented context
```

**Chunking strategies** (ChunkStrategy):
- `fixed`: token-based windows (best for uniform docs)
- `sentence`: NLTK-style sentence boundaries (better recall)
- `paragraph`: double-newline splits (best for markdown/prose)
- `semantic`: embedding similarity breaks (best quality, most expensive)

---

## 6. Worker Lifecycle Design

```
Enqueue → Redis Stream  →  Worker.Dequeue
                               ↓
                         Handler.Handle()
                         ┌─────────────┐
                         │  executor   │  (local or docker)
                         └─────────────┘
                               ↓
                    success → Queue.Ack()
                    failure → Queue.Nack()
                               ↓
                         attempt < maxRetries?
                              ↙           ↘
                       retry (backoff)   → DLQ
```

**Distributed coordination:**
- Each worker node registers via `heartbeat.Service` → Redis TTL key
- Node discovery via `cluster.Registry` (SCAN pattern + JSON decode)
- Distributed locks via `locks.RedisLocker` (SET NX + Lua release script)
- No single point of failure — any node can process any task type

**Graceful shutdown:**
- SIGTERM → Pool.Shutdown(30s context)
- In-flight tasks get 30s to complete
- Worker drains queue, sends final heartbeat, deregisters

---

## 7. Distributed Execution Model

```
bzy-runner Node 1    bzy-runner Node 2    bzy-runner Node 3
  [W1][W2][W3][W4]    [W1][W2][W3][W4]    [W1][W2][W3][W4]
        │                    │                    │
        └────────────────────┴────────────────────┘
                             │
                      Redis Streams
                      (shared queue)
```

**Task routing:** Consumer groups ensure each task is processed by exactly one worker.

**Docker executor:** Each task runs in an isolated container:
- Resource limits: CPU + memory caps per task
- Network isolation: custom bridge network
- No-new-privileges security opt
- Container auto-removed after execution

**Cron scheduler:** Built on `robfig/cron` with:
- SkipIfStillRunning: prevents overlapping job runs
- Recover: panics logged, scheduler continues
- 6-field cron (with seconds) for precision

---

## 8. Redis vs NATS vs Kafka — Tradeoffs

| Feature              | Redis Streams        | NATS JetStream       | Kafka                |
|---------------------|----------------------|----------------------|----------------------|
| **Ops complexity**  | Very low             | Low                  | High                 |
| **Latency**         | ~1ms                 | <1ms                 | ~5-10ms              |
| **Throughput**      | 100K msg/s           | 1M+ msg/s            | 10M+ msg/s           |
| **Durability**      | Configurable (AOF)   | File-backed          | Replication factor   |
| **Consumer groups** | Yes                  | Yes (pull+push)      | Yes (consumer groups)|
| **Replay**          | Yes (XRange)         | Yes                  | Yes                  |
| **Already using Redis?** | ✅ Yes (reuse)  | ➕ Extra infra       | ➕ Extra infra       |
| **Multi-region**    | Redis Cluster        | Leaf nodes           | Kafka MirrorMaker    |
| **Best for**        | MVP, simple patterns | Production scale     | High-volume ingestion|

**Recommendation:**
- **MVP**: Redis Streams — already running for queue/locks/cache/sessions
- **Production**: Migrate to NATS JetStream for lower latency and better multi-node fan-out
- **Enterprise**: Kafka if you need 10M+ msg/s or Kafka ecosystem tools (Flink, ksqlDB)

The `events.Bus` interface in `pkg/events` is provider-agnostic —
swapping implementations requires zero changes to service code.

---

## 9. Authentication Flow

```
Client                 bzy-interface              Redis            bzy-brain
  │                        │                        │                  │
  │── POST /api/v1/auth ──→│                        │                  │
  │   { email, password }  │                        │                  │
  │                        │─── gRPC ValidateUser ─→│                  │
  │                        │                        │                  │
  │                        │←── UserID, Roles ──────│                  │
  │                        │                        │                  │
  │                        │─── session.Create ────→│                  │
  │                        │   (store in Redis)     │                  │
  │                        │                        │                  │
  │←─ { access_token,      │                        │                  │
  │     refresh_token }    │                        │                  │
  │                        │                        │                  │
  │── GET /api/v1/chat ───→│                        │                  │
  │   Authorization: Bearer <JWT>                   │                  │
  │                        │                        │                  │
  │                        │── jwt.Validate() ──────(local)            │
  │                        │── Extract user_id ─────(from claims)      │
  │                        │                        │                  │
  │                        │─────────── gRPC ChatRequest ─────────────→│
  │                        │            (user_id in metadata)          │
```

**Security layers:**
1. JWT (HS256) for session-less stateless auth
2. Redis-backed session tokens for refresh/revocation
3. API keys for service-to-service and automation use
4. Rate limiting per client IP + user ID
5. All internal service communication via gRPC (no public exposure)

---

## 10. Streaming Response Design

### SSE (Server-Sent Events) — AI streaming responses

```
Client                  bzy-interface               bzy-brain
  │                          │                          │
  │── GET /chat/stream ─────→│                          │
  │   (EventSource)          │                          │
  │                          │── gRPC ChatStream() ────→│
  │                          │                          │── OpenAI stream
  │←─ event: delta ──────────│←── StreamChunk{delta} ──│
  │←─ event: delta ──────────│←── StreamChunk{delta} ──│
  │←─ event: done ───────────│←── StreamChunk{done} ───│
  │   (EventSource closes)   │                          │
```

SSE is preferred over WebSocket for streaming because:
- Works through HTTP/2 multiplexing and proxies
- Auto-reconnects on disconnect
- No handshake overhead
- Half-duplex is sufficient for AI responses

### WebSocket — Realtime bidirectional

Used for: task progress events, agent status, multi-device sync.

```
ws.Hub manages connections per user:
  user_123 → [conn_1 (laptop), conn_2 (phone), conn_3 (tablet)]

Hub.SendToUser("user_123", msg) broadcasts to all devices simultaneously.
```

---

## 11. Plugin SDK Architecture

Plugins are self-contained units loaded by the agent runtime. Two types:

**In-process plugins** (pkg/plugin.Plugin interface):
```go
type Plugin interface {
    Manifest() Manifest        // capability declaration
    Execute(ctx, Input) (*Output, error)
}
```

**Tool plugins** (apps/bzy-brain/internal/tools.Tool):
```go
type Tool interface {
    Spec() Spec                // JSON Schema for agent tool calling
    Execute(ctx, argsJSON string) (string, error)
}
```

Tool plugins are exposed to AI agents via the provider's tool-calling API.
Plugin registry → Tool registry → Agent sees tools as JSON Schema specs.

**Plugin lifecycle:**
1. Register plugin in registry at startup
2. Agent receives tool specs via `ToProviderSpecs()`
3. AI model decides to call a tool → `tool_calls` in response
4. Executor dispatches to matching Tool.Execute()
5. Result returned as tool observation message

---

## 12. MVP Roadmap

### Phase 1 — Foundation (Weeks 1-4)
- [ ] Dev infra up (`make dev`)
- [ ] Brain: PostgreSQL + pgvector + migrations
- [ ] Brain: OpenAI provider, embedding service, vector store
- [ ] Brain: Basic memory store + retrieval
- [ ] Interface: HTTP server, JWT auth, chat endpoint
- [ ] Interface: SSE streaming for AI responses

### Phase 2 — Intelligence (Weeks 5-8)
- [ ] Brain: Full RAG pipeline (ingest + query)
- [ ] Brain: ReAct reasoning engine
- [ ] Brain: Agent registry + general agent
- [ ] Interface: Document ingestion API
- [ ] Interface: Memory recall API
- [ ] Runner: Task queue + worker pool

### Phase 3 — Automation (Weeks 9-12)
- [ ] Runner: Cron scheduler
- [ ] Runner: Docker executor
- [ ] Brain: Workflow engine (DAG)
- [ ] Brain: Planning/decomposition
- [ ] Interface: WebSocket hub
- [ ] Interface: Task management API

### Phase 4 — Scale (Weeks 13-16)
- [ ] NATS JetStream migration for event bus
- [ ] Kubernetes deployment
- [ ] Observability (Grafana dashboards)
- [ ] Multi-device session sync
- [ ] Knowledge graph (kg_nodes + kg_edges)
- [ ] Local model support (Ollama provider)

---

## 13. Production Scaling Strategy

### Horizontal scaling
- `bzy-interface`: stateless, scale to N replicas behind load balancer
- `bzy-runner`: each pod = independent worker pool, HPA on CPU/memory
- `bzy-brain`: stateless except DB connections; scale replicas freely
  - Use pgBouncer for connection pooling (PG has ~100 max connections by default)

### Database scaling
- PgVector HNSW index handles millions of vectors efficiently
- Partition `vectors` table by `collection` when >10M rows
- Read replicas for search-heavy workloads
- Redis Cluster for high-availability queue/lock/cache

### AI cost optimization
- Embedding cache in Redis (24h TTL) — eliminates ~80% of redundant API calls
- Token counting before requests to avoid overages
- Batch embedding for document ingestion
- Model tiering: use smaller models (gpt-4o-mini) for lightweight tasks

### Caching strategy
- L1: In-process Go `sync.Map` for hot config/manifests
- L2: Redis for embeddings, sessions, rate limits, distributed state
- L3: PostgreSQL for durable storage

---

## 14. Security Considerations

### Authentication & Authorization
- JWT signed with HS256, short expiry (24h), refresh tokens (7d)
- All refresh tokens stored in Redis → revocable on logout
- API keys hashed (bcrypt) before storage, compared in constant time

### Secrets management
- **Never** commit API keys or credentials
- Use environment variables or Kubernetes Secrets
- Rotate JWT secret → all tokens immediately invalidated (by design)

### Network security
- All internal gRPC communication should use mTLS in production
- `bzy-runner` Docker executor: no-new-privileges, read-only rootfs, dropped capabilities
- Rate limiting: per-user + per-IP at interface layer

### Input validation
- All user inputs sanitized before embedding/RAG injection
- Prompt injection mitigation: system prompt clearly delineates user content
- SQL: use parameterized queries exclusively (pgx prepared statements)

### Container security
- Distroless base image (no shell, no package manager)
- Non-root user (uid 65532)
- Read-only root filesystem
- All capabilities dropped

---

## 15. Example: AI Reasoning Pipeline

A user asks: *"Analyze my last 30 emails and summarize key action items"*

```
bzy-interface
  ↓ POST /api/v1/agents/researcher/run
  ↓ gRPC RunAgent(agent_id=researcher, input=<query>)

bzy-brain (agent.Run)
  1. System prompt: "You are a research specialist..."
  2. User message: "Analyze last 30 emails..."
  3. → gRPC to brain provider (OpenAI)
  4. AI responds: tool_call { name: "list_emails", args: { limit: 30 } }
  5. tools.Execute("list_emails", args) → returns email JSON
  6. AI message: tool observation appended
  7. → gRPC to brain provider again
  8. AI responds: tool_call { name: "analyze_text", args: { text: <emails> } }
  9. tools.Execute("analyze_text") → structured analysis
  10. → final AI response: "Here are your 10 action items..."

  Events emitted:
    agent.spawned → bzy:events:agents
    agent.responded → bzy:events:agents → interface → SSE push → browser

bzy-brain (memory.Store)
  → Store summary as Semantic memory (importance=0.8)
  → Vector embed + store in pgvector collection=memory
```

---

## 16. Example: Distributed Task Workflow

*User triggers: "Every weekday at 9am, scrape news, summarize, send digest"*

```
bzy-interface
  ↓ POST /api/v1/workflows/  (creates WorkflowDef in brain)
  ↓ POST /api/v1/schedules/  (creates ScheduledJob in runner)
    cron_expr: "0 9 * * 1-5"
    task_type: "workflow.news_digest"

bzy-runner (CronScheduler triggers at 9:00 Monday)
  → Enqueue task { type: "workflow.news_digest", payload: {...} }

bzy-runner (Worker.processTask)
  → WorkflowHandler.Handle(task)
  → gRPC bzy-brain ExecuteWorkflow(def_id=news_digest)

bzy-brain (workflow.Engine.executeDAG)
  Node 1: agent(researcher) → scrape_web(news sites) → chunks
  Node 2: tool(summarize) → LLM summarization → summary text
  Node 3: tool(send_email) → email digest to user
  Node 4: memory.Store(summary) → persisted for future recall

  Events:
    workflow.started  → bzy:events:workflows
    workflow.completed → brain saves memory, runner logs result
    task.completed → interface sends WS push to connected devices
```

---

## 17. Future Expansion Architecture

### Multi-tenant
- Add `tenant_id` to all DB schemas
- Row-level security in PostgreSQL
- Per-tenant AI provider config (BYOK — bring your own key)

### Multi-model
- Ollama provider → run Llama 3, Mistral, Phi locally
- Model routing: smart router selects model by task complexity/cost
- Fallback chain: primary → secondary → local model

### Knowledge graph
- `kg_nodes` + `kg_edges` tables already in schema
- Neo4j integration for complex graph traversal
- Graph-RAG: combine vector search with graph expansion

### Plugin ecosystem
- Remote plugins via gRPC (language-agnostic)
- Plugin marketplace: discover/install plugins at runtime
- Sandboxed WASM plugins (zero-trust execution)

### Multi-device orchestration
- WebSocket sync protocol for state across devices
- Offline-first: queue ops locally, sync on reconnect
- End-to-end encryption for sensitive memories

### Voice interface
- Whisper for speech-to-text (local or API)
- TTS for AI responses
- Real-time audio streaming via WebSocket

### Mobile
- React Native app consuming bzy-interface REST/WS API
- Push notifications via FCM/APNs via runner automation tasks

### Federated nodes
- Multiple self-hosted instances sharing memories via encrypted sync
- Personal data stays local; only embeddings/summaries shared
```
