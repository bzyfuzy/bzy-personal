import { request, streamSSE } from './client'
import type { AgentDef, AgentChunk, RunAgentResponse } from './types'

export interface RunAgentRequest {
  session_id?: string
  input: string
}

export async function list(): Promise<AgentDef[]> {
  const resp = await request<{ agents: AgentDef[] }>('/api/v1/agents/')
  return resp.agents ?? []
}

export async function run(
  agentId: string,
  req: RunAgentRequest,
): Promise<RunAgentResponse> {
  return request<RunAgentResponse>(`/api/v1/agents/${agentId}/run`, {
    method: 'POST',
    body: req,
  })
}

export async function* runStream(
  agentId: string,
  req: RunAgentRequest,
  signal?: AbortSignal,
): AsyncGenerator<AgentChunk> {
  for await (const { data } of streamSSE(
    `/api/v1/agents/${agentId}/stream`,
    req,
    signal,
  )) {
    const chunk: AgentChunk = JSON.parse(data)
    yield chunk
    if (chunk.done) return
  }
}
