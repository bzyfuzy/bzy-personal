import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { TrashIcon } from '@heroicons/react/24/outline'
import * as memoryApi from '../../api/memory'
import { Badge } from '../ui/Badge'
import { Button } from '../ui/Button'
import { Input } from '../ui/Input'
import { Spinner } from '../ui/Spinner'

const typeColor: Record<string, 'blue' | 'green' | 'purple' | 'yellow'> = {
  episodic: 'blue',
  semantic: 'green',
  procedural: 'purple',
  working: 'yellow',
}

export function MemoryPanel() {
  const qc = useQueryClient()
  const [q, setQ] = useState('')
  const [recallQ, setRecallQ] = useState('')

  const { data: memories, isLoading } = useQuery({
    queryKey: ['memories'],
    queryFn: () => memoryApi.list(100),
  })

  const { data: recalled, isFetching: recalling, refetch: doRecall } = useQuery({
    queryKey: ['recall', recallQ],
    queryFn: () => memoryApi.recall(recallQ),
    enabled: false,
  })

  const forget = useMutation({
    mutationFn: (id: string) => memoryApi.forget(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['memories'] }),
  })

  const filtered = memories?.filter((m) =>
    !q || m.content.toLowerCase().includes(q.toLowerCase()),
  ) ?? []

  return (
    <div className="flex flex-col gap-4 p-6 overflow-y-auto h-full">
      <h2 className="text-base font-semibold">Memory</h2>

      <div className="flex gap-2">
        <Input
          placeholder="Recall query…"
          value={recallQ}
          onChange={(e) => setRecallQ(e.target.value)}
          className="flex-1"
          onKeyDown={(e) => e.key === 'Enter' && doRecall()}
        />
        <Button onClick={() => doRecall()} loading={recalling} size="md">
          Recall
        </Button>
      </div>

      {recalled && recalled.length > 0 && (
        <div className="rounded-lg border border-indigo-700 bg-indigo-950/30 p-3 space-y-2">
          <p className="text-xs text-indigo-400 font-medium">Recall results</p>
          {recalled.map((r) => (
            <div key={r.memory.id} className="flex items-start justify-between gap-2 text-sm">
              <span className="text-zinc-200 flex-1">{r.memory.content}</span>
              <span className="text-xs text-zinc-500 flex-shrink-0">{(r.score * 100).toFixed(0)}%</span>
            </div>
          ))}
        </div>
      )}

      <Input
        placeholder="Filter memories…"
        value={q}
        onChange={(e) => setQ(e.target.value)}
      />

      {isLoading ? (
        <div className="flex justify-center py-8"><Spinner /></div>
      ) : (
        <div className="space-y-2">
          {filtered.map((m) => (
            <div
              key={m.id}
              className="flex items-start gap-3 rounded-lg border border-zinc-800 bg-zinc-900 p-3"
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 mb-1">
                  <Badge color={typeColor[m.type] ?? 'gray'}>{m.type}</Badge>
                  <span className="text-xs text-zinc-500">
                    {new Date(m.created_at).toLocaleDateString()}
                  </span>
                </div>
                <p className="text-sm text-zinc-200 whitespace-pre-wrap">{m.content}</p>
              </div>
              <button
                onClick={() => forget.mutate(m.id)}
                className="flex-shrink-0 text-zinc-600 hover:text-red-400 transition-colors"
              >
                <TrashIcon className="h-4 w-4" />
              </button>
            </div>
          ))}
          {filtered.length === 0 && (
            <p className="text-sm text-zinc-500 text-center py-8">No memories found.</p>
          )}
        </div>
      )}
    </div>
  )
}
