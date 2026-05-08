import { useState, useEffect, useRef } from 'react'
import { XMarkIcon } from '@heroicons/react/24/outline'
import { useTasks, useCancelTask } from '../../hooks/useTasks'
import * as tasksApi from '../../api/tasks'
import type { Task, LogEntry } from '../../api/types'
import { Badge } from '../ui/Badge'
import { Button } from '../ui/Button'
import { Spinner } from '../ui/Spinner'

type StatusColor = 'gray' | 'yellow' | 'green' | 'red' | 'blue'
const statusColor: Record<string, StatusColor> = {
  queued:     'yellow',
  processing: 'blue',
  completed:  'green',
  failed:     'red',
  retrying:   'yellow',
  dead:       'red',
  cancelled:  'gray',
}

function TaskLogs({ taskId }: { taskId: string }) {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const ac = new AbortController()
    ;(async () => {
      try {
        for await (const entry of tasksApi.streamLogs(taskId, true, ac.signal)) {
          setLogs((l) => [...l, entry])
        }
      } catch { /* done or aborted */ }
    })()
    return () => ac.abort()
  }, [taskId])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  return (
    <pre className="h-48 overflow-y-auto rounded bg-zinc-950 p-3 text-xs text-zinc-400 font-mono">
      {logs.map((l, i) => (
        <div key={i} className={l.level === 'ERROR' ? 'text-red-400' : ''}>
          <span className="text-zinc-600 mr-2">{new Date(l.timestamp).toLocaleTimeString()}</span>
          {l.message}
        </div>
      ))}
      {logs.length === 0 && <span className="text-zinc-600">Waiting for logs…</span>}
      <div ref={bottomRef} />
    </pre>
  )
}

export function TasksPanel() {
  const { data: tasks, isLoading } = useTasks()
  const cancel = useCancelTask()
  const [selected, setSelected] = useState<Task | null>(null)

  return (
    <div className="flex gap-4 h-full overflow-hidden p-6">
      <div className="flex flex-col gap-2 w-72 flex-shrink-0 overflow-y-auto">
        <h2 className="text-base font-semibold mb-2">Tasks</h2>
        {isLoading && <Spinner />}
        {tasks?.map((t) => (
          <div
            key={t.id}
            onClick={() => setSelected(t)}
            className={`cursor-pointer rounded-lg border p-3 transition-colors ${
              selected?.id === t.id
                ? 'border-indigo-600 bg-indigo-950/40'
                : 'border-zinc-800 bg-zinc-900 hover:border-zinc-700'
            }`}
          >
            <div className="flex items-center justify-between gap-2">
              <span className="text-sm font-medium text-zinc-100 truncate">{t.type}</span>
              <Badge color={statusColor[t.status] ?? 'gray'}>{t.status}</Badge>
            </div>
            <p className="text-xs text-zinc-500 mt-1">{t.id.slice(0, 8)}</p>
          </div>
        ))}
        {!isLoading && tasks?.length === 0 && (
          <p className="text-sm text-zinc-500">No tasks.</p>
        )}
      </div>

      <div className="flex-1 overflow-hidden">
        {selected ? (
          <div className="flex flex-col gap-3 h-full">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <h3 className="text-sm font-semibold text-zinc-100">{selected.type}</h3>
                <Badge color={statusColor[selected.status] ?? 'gray'}>{selected.status}</Badge>
              </div>
              <div className="flex gap-2">
                {(selected.status === 'queued' || selected.status === 'processing') && (
                  <Button
                    variant="danger"
                    size="sm"
                    loading={cancel.isPending}
                    onClick={() => cancel.mutate(selected.id)}
                  >
                    <XMarkIcon className="h-3.5 w-3.5" />
                    Cancel
                  </Button>
                )}
                <button onClick={() => setSelected(null)} className="text-zinc-500 hover:text-zinc-300">
                  <XMarkIcon className="h-4 w-4" />
                </button>
              </div>
            </div>
            <div className="text-xs text-zinc-500 font-mono">{selected.id}</div>
            <TaskLogs taskId={selected.id} />
          </div>
        ) : (
          <div className="flex h-full items-center justify-center text-zinc-500 text-sm">
            Select a task to view logs.
          </div>
        )}
      </div>
    </div>
  )
}
