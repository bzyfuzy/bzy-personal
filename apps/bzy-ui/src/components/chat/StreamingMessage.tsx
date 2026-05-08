interface Props {
  content: string
}

export function StreamingMessage({ content }: Props) {
  return (
    <div className="flex justify-start">
      <div className="max-w-[75%] rounded-2xl rounded-bl-sm bg-zinc-800 px-4 py-2.5 text-sm text-zinc-100 whitespace-pre-wrap break-words">
        {content}
        <span className="ml-0.5 inline-block h-4 w-0.5 animate-pulse bg-indigo-400 align-text-bottom" />
      </div>
    </div>
  )
}
