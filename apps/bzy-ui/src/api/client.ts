import { APIError, type APIErrorBody } from './types'

const BASE = import.meta.env.VITE_API_URL ?? ''

// ── Token store (in-memory, persisted to localStorage) ────────────────────────

let _accessToken: string | null = localStorage.getItem('bzy:access_token')
let _refreshToken: string | null = localStorage.getItem('bzy:refresh_token')
let _tokenExpiry: number = Number(localStorage.getItem('bzy:token_expiry') ?? 0)
let _refreshPromise: Promise<void> | null = null

export const tokenStore = {
  set(access: string, refresh: string, expiresAt: string) {
    _accessToken = access
    _refreshToken = refresh
    _tokenExpiry = new Date(expiresAt).getTime()
    localStorage.setItem('bzy:access_token', access)
    localStorage.setItem('bzy:refresh_token', refresh)
    localStorage.setItem('bzy:token_expiry', String(_tokenExpiry))
  },
  clear() {
    _accessToken = _refreshToken = null
    _tokenExpiry = 0
    localStorage.removeItem('bzy:access_token')
    localStorage.removeItem('bzy:refresh_token')
    localStorage.removeItem('bzy:token_expiry')
  },
  get access()   { return _accessToken },
  get refresh()  { return _refreshToken },
  get isExpired(){ return Date.now() > _tokenExpiry - 60_000 },
  get hasToken() { return !!_accessToken },
}

// ── Core fetch wrapper ────────────────────────────────────────────────────────

interface RequestOptions {
  method?: string
  body?: unknown
  headers?: Record<string, string>
  signal?: AbortSignal
}

async function authHeaders(): Promise<Record<string, string>> {
  const apiKey = import.meta.env.VITE_API_KEY
  if (apiKey) return { 'X-API-Key': apiKey }
  if (tokenStore.hasToken && tokenStore.isExpired) {
    await ensureFresh()
  }
  if (tokenStore.access) return { Authorization: `Bearer ${tokenStore.access}` }
  return {}
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, headers = {}, signal } = opts
  const auth = await authHeaders()

  const resp = await fetch(`${BASE}${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
      ...auth,
      ...headers,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal,
  })

  if (!resp.ok) {
    let errBody: APIErrorBody = { code: 'UNKNOWN', message: resp.statusText }
    try { errBody = await resp.json() } catch { /* ignore */ }
    throw new APIError(resp.status, errBody.code, errBody.message, errBody.detail)
  }

  if (resp.status === 204) return undefined as T
  return resp.json() as Promise<T>
}

// ── SSE streaming via fetch (supports POST unlike EventSource) ────────────────

export async function* streamSSE(
  path: string,
  body?: unknown,
  signal?: AbortSignal,
): AsyncGenerator<{ event: string; data: string; id?: string }> {
  const auth = await authHeaders()
  const resp = await fetch(`${BASE}${path}`, {
    method: body ? 'POST' : 'GET',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
      'Cache-Control': 'no-cache',
      ...auth,
    },
    body: body ? JSON.stringify(body) : undefined,
    signal,
  })

  if (!resp.ok) {
    let errBody: APIErrorBody = { code: 'UNKNOWN', message: resp.statusText }
    try { errBody = await resp.json() } catch { /* ignore */ }
    throw new APIError(resp.status, errBody.code, errBody.message)
  }

  const reader = resp.body!.getReader()
  const decoder = new TextDecoder()
  let buf = ''

  let eventName = ''
  let eventData = ''
  let eventId = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buf += decoder.decode(value, { stream: true })

    const lines = buf.split('\n')
    buf = lines.pop() ?? ''

    for (const line of lines) {
      if (line.startsWith('event:')) {
        eventName = line.slice(6).trim()
      } else if (line.startsWith('data:')) {
        eventData = line.slice(5).trim()
      } else if (line.startsWith('id:')) {
        eventId = line.slice(3).trim()
      } else if (line === '') {
        if (eventData === '[DONE]' || eventName === 'done') {
          return
        }
        if (eventData) {
          yield { event: eventName || 'message', data: eventData, id: eventId || undefined }
        }
        eventName = ''
        eventData = ''
        eventId = ''
      }
    }
  }
}

// ── Token refresh ─────────────────────────────────────────────────────────────

async function ensureFresh(): Promise<void> {
  if (!_refreshPromise) {
    _refreshPromise = doRefresh().finally(() => { _refreshPromise = null })
  }
  return _refreshPromise
}

async function doRefresh(): Promise<void> {
  if (!tokenStore.refresh) return
  try {
    const { refresh } = await import('./auth')
    await refresh(tokenStore.refresh)
  } catch {
    tokenStore.clear()
    window.location.href = '/login'
  }
}
