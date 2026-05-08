import { request, streamSSE } from './client'
import type { Task, TaskStatus, TaskPriority, LogEntry } from './types'

export interface EnqueueRequest {
  type: string
  payload: unknown
  priority?: TaskPriority
  max_attempts?: number
  timeout?: number
  run_at?: string
}

export async function enqueue(req: EnqueueRequest): Promise<{ task_id: string }> {
  return request('/api/v1/tasks/', { method: 'POST', body: req })
}

export async function get(id: string): Promise<Task> {
  return request<Task>(`/api/v1/tasks/${id}`)
}

export async function list(
  status?: TaskStatus,
  limit = 50,
): Promise<Task[]> {
  const params = new URLSearchParams()
  if (status) params.set('status', status)
  params.set('limit', String(limit))
  const resp = await request<{ tasks: Task[] }>(`/api/v1/tasks/?${params}`)
  return resp.tasks ?? []
}

export async function cancel(id: string): Promise<void> {
  return request(`/api/v1/tasks/${id}`, { method: 'DELETE' })
}

export async function* streamLogs(
  taskId: string,
  follow = true,
  signal?: AbortSignal,
): AsyncGenerator<LogEntry> {
  const params = follow ? '?follow=true' : ''
  for await (const { data } of streamSSE(
    `/api/v1/tasks/${taskId}/logs${params}`,
    undefined,
    signal,
  )) {
    yield JSON.parse(data) as LogEntry
  }
}
