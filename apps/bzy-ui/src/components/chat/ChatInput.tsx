import { useState, useRef, type KeyboardEvent } from 'react'
import { PaperAirplaneIcon, StopIcon } from '@heroicons/react/24/solid'
import { Textarea } from '../ui/Input'
import { Button } from '../ui/Button'

interface Props {
  onSend: (text: string) => void
  onStop: () => void
  streaming: boolean
  disabled?: boolean
}

export function ChatInput({ onSend, onStop, streaming, disabled }: Props) {
  const [value, setValue] = useState('')
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const submit = () => {
    const text = value.trim()
    if (!text) return
    setValue('')
    onSend(text)
    setTimeout(() => textareaRef.current?.focus(), 0)
  }

  const handleKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (streaming) return
      submit()
    }
  }

  return (
    <div className="border-t border-zinc-800 bg-zinc-950 p-4">
      <div className="flex items-end gap-2">
        <Textarea
          ref={textareaRef}
          rows={1}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={handleKey}
          placeholder="Message bzy… (Enter to send, Shift+Enter for newline)"
          disabled={disabled}
          className="max-h-40 leading-relaxed"
          style={{ height: 'auto' }}
          onInput={(e) => {
            const t = e.currentTarget
            t.style.height = 'auto'
            t.style.height = `${t.scrollHeight}px`
          }}
        />
        {streaming ? (
          <Button variant="danger" size="md" onClick={onStop} className="flex-shrink-0 self-end">
            <StopIcon className="h-4 w-4" />
          </Button>
        ) : (
          <Button
            variant="primary"
            size="md"
            onClick={submit}
            disabled={!value.trim() || disabled}
            className="flex-shrink-0 self-end"
          >
            <PaperAirplaneIcon className="h-4 w-4" />
          </Button>
        )}
      </div>
    </div>
  )
}
