import { request, tokenStore } from './client'
import type { TokenPair } from './types'

export interface LoginRequest {
  email: string
  password: string
}

export async function login(email: string, password: string): Promise<TokenPair> {
  const pair = await request<TokenPair>('/api/v1/auth/login', {
    method: 'POST',
    body: { email, password },
  })
  tokenStore.set(pair.access_token, pair.refresh_token, pair.expires_at)
  return pair
}

export async function refresh(refreshToken: string): Promise<TokenPair> {
  const pair = await request<TokenPair>('/api/v1/auth/refresh', {
    method: 'POST',
    body: { refresh_token: refreshToken },
  })
  tokenStore.set(pair.access_token, pair.refresh_token, pair.expires_at)
  return pair
}

export function logout(): void {
  tokenStore.clear()
}
