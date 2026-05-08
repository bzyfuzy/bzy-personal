import { useEffect, useRef } from 'react'
import { useChat } from '../../hooks/useChat'
import { MessageBubble } from './MessageBubble'
import { StreamingMessage } from './StreamingMessage'
import { ChatInput } from './ChatInput'

interface Props {
  sessionId: string
}

export function ChatView({ sessionId }: Props) {
  const { messages, streaming, streamingContent, send, stop } = useChat(sessionId)
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streamingContent])

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <div className="flex-1 overflow-y-auto px-4 py-6 space-y-3">
        {messages.length === 0 && !streaming && (
          <div className="flex h-full items-center justify-center">
            <p className="text-zinc-500 text-sm">Send a message to get started.</p>
          </div>
        )}
        {messages.map((msg, i) => (
          <MessageBubble key={i} msg={msg} />
        ))}
        {streaming && <StreamingMessage content={streamingContent} />}
        <div ref={bottomRef} />
      </div>
      <ChatInput onSend={send} onStop={stop} streaming={streaming} />
    </div>
  )
}
