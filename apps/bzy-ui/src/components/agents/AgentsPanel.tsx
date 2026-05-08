import { useState, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import * as agentsApi from '../../api/agents'
import { Button } from '../ui/Button'
import { Input } from '../ui/Input'
import { Spinner } from '../ui/Spinner'
import { Badge } from '../ui/Badge'

export function AgentsPanel() {
  const { data: agents, isLoading } = useQuery({
    queryKey: ['agents'],
    queryFn: agentsApi.list,
  })

  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const [running, setRunning] = useState(false)
  const abortRef = useRef<AbortController | null>(null)

  const run = async () => {
    if (!selectedId || !input.trim()) return
    setOutput('')
    setRunning(true)
    abortRef.current = new AbortController()
    try {
      for await (const chunk of agentsApi.runStream(
        selectedId,
        { input },
        abortRef.current.signal,
      )) {
        if (chunk.delta) setOutput((o) => o + chunk.delta)
        if (chunk.done) break
      }
    } catch { /* aborted */ } finally {
      setRunning(false)
    }
  }

  return (
    <div className="flex gap-4 h-full overflow-hidden p-6">
      <div className="w-56 flex-shrink-0 flex flex-col gap-2 overflow-y-auto">
        <h2 className="text-base font-semibold mb-2">Agents</h2>
        {isLoading && <Spinner />}
        {agents?.map((a) => (
          <button
            key={a.id}
            onClick={() => { setSelectedId(a.id); setOutput('') }}
            className={`rounded-lg border p-3 text-left transition-colors ${
              selectedId === a.id
                ? 'border-indigo-600 bg-indigo-950/40'
                : 'border-zinc-800 bg-zinc-900 hover:border-zinc-700'
            }`}
          >
            <p className="text-sm font-medium text-zinc-100">{a.name}</p>
            {a.description && (
              <p className="text-xs text-zinc-500 mt-0.5 line-clamp-2">{a.description}</p>
            )}
          </button>
        ))}
      </div>

      <div className="flex flex-1 flex-col gap-3 overflow-hidden">
        {selectedId ? (
          <>
            <div className="flex-1 overflow-y-auto rounded-lg border border-zinc-800 bg-zinc-900 p-4 text-sm text-zinc-200 whitespace-pre-wrap">
              {output || <span className="text-zinc-600">Run the agent to see output…</span>}
              {running && (
                <span className="ml-0.5 inline-block h-4 w-0.5 animate-pulse bg-indigo-400 align-text-bottom" />
              )}
            </div>
            <div className="flex gap-2">
              <Input
                placeholder="Enter input for the agent…"
                value={input}
                onChange={(e) => setInput(e.target.value)}
                className="flex-1"
                onKeyDown={(e) => e.key === 'Enter' && !running && run()}
              />
              {running ? (
                <Button variant="danger" onClick={() => abortRef.current?.abort()}>Stop</Button>
              ) : (
                <Button onClick={run} disabled={!input.trim()}>Run</Button>
              )}
            </div>
          </>
        ) : (
          <div className="flex flex-1 items-center justify-center text-zinc-500 text-sm">
            Select an agent to run it.
          </div>
        )}
      </div>
    </div>
  )
}
