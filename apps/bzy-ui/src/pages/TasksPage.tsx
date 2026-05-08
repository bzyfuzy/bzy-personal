import { TopBar } from '../components/layout/TopBar'
import { TasksPanel } from '../components/tasks/TasksPanel'

export function TasksPage() {
  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <TopBar title="Tasks" />
      <TasksPanel />
    </div>
  )
}
