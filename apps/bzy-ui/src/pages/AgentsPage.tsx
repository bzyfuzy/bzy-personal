import { TopBar } from '../components/layout/TopBar'
import { AgentsPanel } from '../components/agents/AgentsPanel'

export function AgentsPage() {
  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <TopBar title="Agents" />
      <AgentsPanel />
    </div>
  )
}
