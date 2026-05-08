import { tokenStore } from './client'
import type { WSMessage } from './types'

export type WSHandler = (msg: WSMessage) => void

const WS_URL =
  (import.meta.env.VITE_WS_URL ?? window.location.origin)
    .replace(/^http/, 'ws') + '/ws'

export class BzyWebSocket {
  private ws: WebSocket | null = null
  private handlers = new Map<string, WSHandler[]>()
  private pingTimer: ReturnType<typeof setInterval> | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectDelay = 1000
  private maxReconnectDelay = 30_000
  private shouldReconnect = true
  private listeners: Array<(connected: boolean) => void> = []

  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN) return
    const token = tokenStore.access
    const apiKey = import.meta.env.VITE_API_KEY
    const url = apiKey
      ? `${WS_URL}?api_key=${apiKey}`
      : token
      ? `${WS_URL}?token=${token}`
      : WS_URL

    this.ws = new WebSocket(url)

    this.ws.onopen = () => {
      this.reconnectDelay = 1000
      this.startPing()
      this.emit(true)
    }

    this.ws.onmessage = (e) => {
      try {
        const msg: WSMessage = JSON.parse(e.data)
        if (msg.type === 'pong') return
        const fns = this.handlers.get(msg.type) ?? []
        fns.forEach((fn) => fn(msg))
        const wildcard = this.handlers.get('*') ?? []
        wildcard.forEach((fn) => fn(msg))
      } catch { /* malformed message */ }
    }

    this.ws.onclose = () => {
      this.stopPing()
      this.emit(false)
      if (this.shouldReconnect) {
        this.reconnectTimer = setTimeout(() => {
          this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay)
          this.connect()
        }, this.reconnectDelay)
      }
    }

    this.ws.onerror = () => this.ws?.close()
  }

  on(type: string, handler: WSHandler): () => void {
    const existing = this.handlers.get(type) ?? []
    this.handlers.set(type, [...existing, handler])
    return () => {
      this.handlers.set(type, (this.handlers.get(type) ?? []).filter((h) => h !== handler))
    }
  }

  send(type: string, payload?: unknown): void {
    if (this.ws?.readyState !== WebSocket.OPEN) return
    this.ws.send(JSON.stringify({ type, payload }))
  }

  onConnectionChange(fn: (connected: boolean) => void): () => void {
    this.listeners.push(fn)
    return () => { this.listeners = this.listeners.filter((l) => l !== fn) }
  }

  get isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }

  disconnect(): void {
    this.shouldReconnect = false
    this.stopPing()
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    this.ws?.close()
  }

  private startPing(): void {
    this.pingTimer = setInterval(() => this.send('ping'), 25_000)
  }

  private stopPing(): void {
    if (this.pingTimer) clearInterval(this.pingTimer)
  }

  private emit(connected: boolean): void {
    this.listeners.forEach((fn) => fn(connected))
  }
}

export const wsClient = new BzyWebSocket()
