// Canonical type definitions mirroring the Go server models.

export type Role = 'system' | 'user' | 'assistant' | 'tool'

export interface Message {
  id?: string
  role: Role
  content: string
  created_at?: string
}

export interface Session {
  id: string
  user_id: string
  title: string
  summary: string
  messages: Message[]
  created_at: string
  updated_at: string
}

export interface ChatRequest {
  session_id?: string
  messages: Message[]
  model?: string
  temperature?: number
  max_tokens?: number
  stream?: boolean
}

export interface ChatResponse {
  id: string
  model: string
  message: Message
  usage: { prompt_tokens: number; completion_tokens: number; total_tokens: number }
  finish_reason: string
}

export interface ChatDelta {
  delta: string
  done: boolean
  finish_reason?: string
}

// ── Memory ────────────────────────────────────────────────────────────────────

export type MemoryType = 'episodic' | 'semantic' | 'procedural' | 'working'

export interface Memory {
  id: string
  session_id?: string
  user_id: string
  type: MemoryType
  content: string
  summary?: string
  importance: number
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface RecallResult {
  memory: Memory
  score: number
}

// ── Documents / RAG ───────────────────────────────────────────────────────────

export interface IngestRequest {
  title: string
  content: string
  source?: string
  metadata?: Record<string, unknown>
  chunk_strategy?: 'fixed' | 'sentence' | 'paragraph' | 'semantic'
  chunk_size?: number
  chunk_overlap?: number
}

export interface IngestResponse {
  document_id: string
  chunk_count: number
}

export interface RAGChunk {
  id: string
  content: string
  score: number
  metadata?: Record<string, unknown>
}

export interface RAGQueryResponse {
  response: string
  chunks: RAGChunk[]
  latency_ms: number
}

// ── Agents ────────────────────────────────────────────────────────────────────

export interface AgentDef {
  id: string
  name: string
  description: string
  tool_ids: string[]
  max_steps: number
}

export interface AgentChunk {
  delta: string
  done: boolean
  step_type: 'thought' | 'action' | 'observation' | 'answer' | ''
}

export interface RunAgentResponse {
  agent_run_id: string
  answer: string
  steps: number
  latency_ms: number
}

// ── Tasks ─────────────────────────────────────────────────────────────────────

export type TaskStatus =
  | 'queued' | 'processing' | 'completed'
  | 'failed' | 'retrying' | 'dead' | 'cancelled'

export type TaskPriority = 1 | 5 | 9

export interface Task {
  id: string
  type: string
  payload: unknown
  priority: TaskPriority
  status: TaskStatus
  attempt: number
  max_attempts: number
  worker_id?: string
  error?: string
  result?: unknown
  queued_at: string
  started_at?: string
  finished_at?: string
}

export interface LogEntry {
  task_id: string
  worker_id: string
  level: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR'
  message: string
  timestamp: string
}

// ── Workflows ─────────────────────────────────────────────────────────────────

export type WorkflowStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'

export interface WorkflowRun {
  id: string
  def_id: string
  status: WorkflowStatus
  input?: unknown
  output?: unknown
  error?: string
  started_at: string
  finished_at?: string
}

// ── Auth ──────────────────────────────────────────────────────────────────────

export interface TokenPair {
  access_token: string
  refresh_token: string
  expires_at: string
}

// ── WebSocket ─────────────────────────────────────────────────────────────────

export interface WSMessage {
  type: string
  id?: string
  payload?: unknown
}

// ── API error ─────────────────────────────────────────────────────────────────

export interface APIErrorBody {
  code: string
  message: string
  detail?: string
}

export class APIError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public detail?: string,
  ) {
    super(message)
    this.name = 'APIError'
  }

  get isNotFound()    { return this.status === 404 }
  get isUnauthorized(){ return this.status === 401 }
  get isRateLimited() { return this.status === 429 }
  get isServerError() { return this.status >= 500 }
}
