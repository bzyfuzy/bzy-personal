import { request, streamSSE } from './client'
import type { IngestRequest, IngestResponse, RAGQueryResponse, RAGChunk } from './types'
import type { Message } from './types'

export async function ingest(req: IngestRequest): Promise<IngestResponse> {
  return request<IngestResponse>('/api/v1/documents/', {
    method: 'POST',
    body: req,
  })
}

export interface RAGQueryRequest {
  query: string
  collection?: string
  top_k?: number
  history?: Message[]
}

export async function query(req: RAGQueryRequest): Promise<RAGQueryResponse> {
  return request<RAGQueryResponse>('/api/v1/documents/query', {
    method: 'POST',
    body: req,
  })
}

export async function* queryStream(
  req: RAGQueryRequest,
  signal?: AbortSignal,
): AsyncGenerator<{ delta: string; chunks?: RAGChunk[]; done: boolean }> {
  for await (const { data } of streamSSE('/api/v1/documents/query/stream', req, signal)) {
    const chunk = JSON.parse(data)
    yield chunk
    if (chunk.done) return
  }
}

export async function remove(documentId: string): Promise<void> {
  return request(`/api/v1/documents/${documentId}`, { method: 'DELETE' })
}
