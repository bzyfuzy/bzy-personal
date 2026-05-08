import { useCallback, useRef } from 'react'
import { useChatStore } from '../store/chat'
import * as chatApi from '../api/chat'
import type { Message } from '../api/types'

export function useChat(sessionId: string) {
  const { sessions, addMessage, setStreaming, finalizeStream } = useChatStore()
  const session = sessions.find((s) => s.id === sessionId)
  const abortRef = useRef<AbortController | null>(null)

  const send = useCallback(
    async (text: string) => {
      if (!session) return

      const userMsg: Message = { role: 'user', content: text }
      addMessage(sessionId, userMsg)

      abortRef.current = new AbortController()
      let accumulated = ''

      try {
        for await (const chunk of chatApi.stream(
          {
            messages: [...session.messages, userMsg],
            stream: true,
          },
          abortRef.current.signal,
        )) {
          if (chunk.done) break
          accumulated += chunk.delta ?? ''
          setStreaming(sessionId, accumulated, false)
        }
      } catch (err: unknown) {
        if ((err as { name?: string }).name === 'AbortError') return
        throw err
      } finally {
        finalizeStream(sessionId)
      }
    },
    [session, sessionId, addMessage, setStreaming, finalizeStream],
  )

  const stop = useCallback(() => {
    abortRef.current?.abort()
  }, [])

  return {
    messages: session?.messages ?? [],
    streaming: session?.streaming ?? false,
    streamingContent: session?.streamingContent ?? '',
    send,
    stop,
  }
}
