import { useState, useRef } from 'react'
import { useMutation } from '@tanstack/react-query'
import { ArrowUpTrayIcon } from '@heroicons/react/24/outline'
import * as docsApi from '../../api/documents'
import type { IngestRequest } from '../../api/types'
import { Button } from '../ui/Button'
import { Input } from '../ui/Input'

export function DocumentsPanel() {
  const [query, setQuery] = useState('')
  const [answer, setAnswer] = useState('')
  const [streaming, setStreaming] = useState(false)
  const abortRef = useRef<AbortController | null>(null)
  const [fileUploading, setFileUploading] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const ingest = useMutation({
    mutationFn: (req: IngestRequest) => docsApi.ingest(req),
  })

  const handleFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setFileUploading(true)
    try {
      const content = await file.text()
      await docsApi.ingest({ title: file.name, content })
    } finally {
      setFileUploading(false)
      e.target.value = ''
    }
  }

  const handleQuery = async () => {
    if (!query.trim()) return
    setAnswer('')
    setStreaming(true)
    abortRef.current = new AbortController()
    try {
      for await (const chunk of docsApi.queryStream(
        { query },
        abortRef.current.signal,
      )) {
        setAnswer((a) => a + (chunk.delta ?? ''))
        if (chunk.done) break
      }
    } catch { /* aborted or done */ } finally {
      setStreaming(false)
    }
  }

  return (
    <div className="flex flex-col gap-4 p-6 overflow-y-auto h-full">
      <h2 className="text-base font-semibold">Documents (RAG)</h2>

      <div className="flex gap-2">
        <input
          ref={fileInputRef}
          type="file"
          accept=".txt,.md,.pdf"
          className="hidden"
          onChange={handleFile}
        />
        <Button
          variant="ghost"
          size="md"
          loading={fileUploading}
          onClick={() => fileInputRef.current?.click()}
          className="border border-zinc-700"
        >
          <ArrowUpTrayIcon className="h-4 w-4" />
          Upload file
        </Button>
        {ingest.isSuccess && (
          <span className="self-center text-xs text-emerald-400">Ingested successfully.</span>
        )}
      </div>

      <div className="flex gap-2">
        <Input
          placeholder="Ask a question about your documents…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="flex-1"
          onKeyDown={(e) => e.key === 'Enter' && !streaming && handleQuery()}
        />
        {streaming ? (
          <Button variant="danger" size="md" onClick={() => abortRef.current?.abort()}>
            Stop
          </Button>
        ) : (
          <Button size="md" onClick={handleQuery} disabled={!query.trim()}>
            Ask
          </Button>
        )}
      </div>

      {(answer || streaming) && (
        <div className="rounded-lg border border-zinc-800 bg-zinc-900 p-4 text-sm text-zinc-200 whitespace-pre-wrap">
          {answer}
          {streaming && (
            <span className="ml-0.5 inline-block h-4 w-0.5 animate-pulse bg-indigo-400 align-text-bottom" />
          )}
        </div>
      )}
    </div>
  )
}
