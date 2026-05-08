import { TopBar } from '../components/layout/TopBar'
import { MemoryPanel } from '../components/memory/MemoryPanel'

export function MemoryPage() {
  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <TopBar title="Memory" />
      <MemoryPanel />
    </div>
  )
}
