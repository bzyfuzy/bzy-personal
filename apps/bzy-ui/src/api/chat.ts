import { request, streamSSE } from './client'
import type { ChatRequest, ChatResponse, ChatDelta, Session, Message } from './types'

export async function complete(req: ChatRequest): Promise<ChatResponse> {
  return request<ChatResponse>('/api/v1/chat/completions', {
    method: 'POST',
    body: { ...req, stream: false },
  })
}

export async function* stream(
  req: ChatRequest,
  signal?: AbortSignal,
): AsyncGenerator<ChatDelta> {
  for await (const { data } of streamSSE('/api/v1/chat/stream', req, signal)) {
    const delta: ChatDelta = JSON.parse(data)
    yield delta
    if (delta.done) return
  }
}

export async function createSession(title?: string): Promise<Session> {
  return request<Session>('/api/v1/chat/sessions', {
    method: 'POST',
    body: { title },
  })
}

export async function getSession(id: string): Promise<Session> {
  return request<Session>(`/api/v1/chat/sessions/${id}`)
}

export async function getMessages(sessionId: string): Promise<Message[]> {
  const sess = await getSession(sessionId)
  return sess.messages
}
