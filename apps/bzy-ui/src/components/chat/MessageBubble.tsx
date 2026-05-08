import type { Message } from '../../api/types'

interface Props {
  msg: Message
}

export function MessageBubble({ msg }: Props) {
  const isUser = msg.role === 'user'
  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}>
      <div
        className={`max-w-[75%] rounded-2xl px-4 py-2.5 text-sm whitespace-pre-wrap break-words ${
          isUser
            ? 'bg-indigo-600 text-white rounded-br-sm'
            : 'bg-zinc-800 text-zinc-100 rounded-bl-sm'
        }`}
      >
        {msg.content}
      </div>
    </div>
  )
}
