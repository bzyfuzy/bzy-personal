import { create } from 'zustand'
import type { Message } from '../api/types'

export interface ChatSession {
  id: string
  title: string
  messages: Message[]
  streaming: boolean
  streamingContent: string
  createdAt: string
}

interface ChatState {
  sessions: ChatSession[]
  activeId: string | null
  newSession: () => string
  setActive: (id: string) => void
  addMessage: (sessionId: string, msg: Message) => void
  setStreaming: (sessionId: string, content: string, done: boolean) => void
  finalizeStream: (sessionId: string) => void
  removeSession: (id: string) => void
  updateTitle: (sessionId: string, title: string) => void
}

let counter = 0
function genId() {
  return `${Date.now()}-${++counter}`
}

export const useChatStore = create<ChatState>((set, get) => ({
  sessions: [],
  activeId: null,

  newSession: () => {
    const id = genId()
    set((s) => ({
      sessions: [
        ...s.sessions,
        {
          id,
          title: 'New chat',
          messages: [],
          streaming: false,
          streamingContent: '',
          createdAt: new Date().toISOString(),
        },
      ],
      activeId: id,
    }))
    return id
  },

  setActive: (id) => set({ activeId: id }),

  addMessage: (sessionId, msg) =>
    set((s) => ({
      sessions: s.sessions.map((sess) =>
        sess.id === sessionId
          ? { ...sess, messages: [...sess.messages, msg] }
          : sess,
      ),
    })),

  setStreaming: (sessionId, content, done) =>
    set((s) => ({
      sessions: s.sessions.map((sess) =>
        sess.id === sessionId
          ? { ...sess, streaming: !done, streamingContent: done ? '' : content }
          : sess,
      ),
    })),

  finalizeStream: (sessionId) => {
    const sess = get().sessions.find((s) => s.id === sessionId)
    if (!sess || !sess.streamingContent) return
    const content = sess.streamingContent
    set((s) => ({
      sessions: s.sessions.map((se) =>
        se.id === sessionId
          ? {
              ...se,
              streaming: false,
              streamingContent: '',
              messages: [
                ...se.messages,
                { role: 'assistant', content } as Message,
              ],
            }
          : se,
      ),
    }))
  },

  removeSession: (id) =>
    set((s) => {
      const sessions = s.sessions.filter((sess) => sess.id !== id)
      return {
        sessions,
        activeId: s.activeId === id ? (sessions[0]?.id ?? null) : s.activeId,
      }
    }),

  updateTitle: (sessionId, title) =>
    set((s) => ({
      sessions: s.sessions.map((sess) =>
        sess.id === sessionId ? { ...sess, title } : sess,
      ),
    })),
}))
