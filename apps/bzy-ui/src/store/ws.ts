import { create } from 'zustand'
import { wsClient } from '../api/ws'

interface WSState {
  connected: boolean
  connect: () => void
  disconnect: () => void
}

export const useWSStore = create<WSState>((set) => {
  wsClient.onConnectionChange((connected) => set({ connected }))

  return {
    connected: wsClient.isConnected,

    connect: () => wsClient.connect(),

    disconnect: () => wsClient.disconnect(),
  }
})
