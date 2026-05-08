import { request } from './client'
import type { Memory, MemoryType, RecallResult } from './types'

export interface StoreMemoryRequest {
  session_id?: string
  type: MemoryType
  content: string
  importance?: number
  metadata?: Record<string, unknown>
}

export async function store(req: StoreMemoryRequest): Promise<{ id: string }> {
  return request('/api/v1/memory/', { method: 'POST', body: req })
}

export async function recall(
  query: string,
  topK = 10,
  minScore = 0.65,
): Promise<RecallResult[]> {
  const params = new URLSearchParams({
    q: query,
    top_k: String(topK),
    min_score: String(minScore),
  })
  const resp = await request<{ results: RecallResult[] }>(
    `/api/v1/memory/recall?${params}`,
  )
  return resp.results ?? []
}

export async function forget(id: string): Promise<void> {
  return request(`/api/v1/memory/${id}`, { method: 'DELETE' })
}

export async function list(limit = 50): Promise<Memory[]> {
  const resp = await request<{ memories: Memory[] }>(
    `/api/v1/memory/?limit=${limit}`,
  )
  return resp.memories ?? []
}
