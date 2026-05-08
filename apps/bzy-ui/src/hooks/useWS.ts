import { useEffect } from 'react'
import { useWSStore } from '../store/ws'
import { wsClient } from '../api/ws'
import type { WSHandler } from '../api/ws'

export function useWS() {
  const { connected, connect, disconnect } = useWSStore()

  useEffect(() => {
    connect()
    return () => disconnect()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return { connected }
}

export function useWSEvent(type: string, handler: WSHandler) {
  useEffect(() => {
    return wsClient.on(type, handler)
  }, [type, handler])
}
