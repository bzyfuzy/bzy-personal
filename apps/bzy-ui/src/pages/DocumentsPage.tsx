import { TopBar } from '../components/layout/TopBar'
import { DocumentsPanel } from '../components/documents/DocumentsPanel'

export function DocumentsPage() {
  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <TopBar title="Documents" />
      <DocumentsPanel />
    </div>
  )
}
