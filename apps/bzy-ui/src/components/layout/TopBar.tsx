interface TopBarProps {
  title?: string
}

export function TopBar({ title }: TopBarProps) {
  return (
    <header className="flex h-14 items-center border-b border-zinc-800 bg-zinc-950 px-6">
      <h1 className="text-sm font-semibold text-zinc-100">{title ?? ''}</h1>
    </header>
  )
}
