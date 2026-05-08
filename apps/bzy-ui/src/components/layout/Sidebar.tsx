import { NavLink } from 'react-router-dom'
import {
  ChatBubbleLeftRightIcon,
  CpuChipIcon,
  DocumentTextIcon,
  BoltIcon,
  CircleStackIcon,
  ArrowRightOnRectangleIcon,
} from '@heroicons/react/24/outline'
import { useAuthStore } from '../../store/auth'
import { useWSStore } from '../../store/ws'
import { useChatStore } from '../../store/chat'
import { Button } from '../ui/Button'

const nav = [
  { to: '/chat', icon: ChatBubbleLeftRightIcon, label: 'Chat' },
  { to: '/agents', icon: CpuChipIcon, label: 'Agents' },
  { to: '/documents', icon: DocumentTextIcon, label: 'Documents' },
  { to: '/tasks', icon: BoltIcon, label: 'Tasks' },
  { to: '/memory', icon: CircleStackIcon, label: 'Memory' },
]

export function Sidebar() {
  const { logout } = useAuthStore()
  const connected = useWSStore((s) => s.connected)
  const { sessions, activeId, newSession, setActive, removeSession } = useChatStore()

  return (
    <aside className="flex h-full w-56 flex-col border-r border-zinc-800 bg-zinc-900">
      <div className="flex h-14 items-center gap-2 px-4 border-b border-zinc-800">
        <span className="text-lg font-bold text-indigo-400">bzy</span>
        <span
          title={connected ? 'connected' : 'disconnected'}
          className={`ml-auto h-2 w-2 rounded-full ${connected ? 'bg-emerald-400' : 'bg-zinc-600'}`}
        />
      </div>

      <nav className="flex flex-col gap-0.5 p-2">
        {nav.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              `flex items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors ${
                isActive
                  ? 'bg-indigo-600 text-white'
                  : 'text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100'
              }`
            }
          >
            <Icon className="h-4 w-4 flex-shrink-0" />
            {label}
          </NavLink>
        ))}
      </nav>

      <div className="mt-2 border-t border-zinc-800 px-2 py-2">
        <div className="mb-1 flex items-center justify-between px-1">
          <span className="text-xs text-zinc-500 uppercase tracking-wide">Chats</span>
          <button
            onClick={() => newSession()}
            className="text-xs text-indigo-400 hover:text-indigo-300"
          >
            + New
          </button>
        </div>
        <div className="flex flex-col gap-0.5 overflow-y-auto max-h-64">
          {sessions.map((s) => (
            <div
              key={s.id}
              onClick={() => setActive(s.id)}
              className={`group flex items-center justify-between rounded px-2 py-1.5 cursor-pointer text-xs ${
                s.id === activeId
                  ? 'bg-zinc-700 text-zinc-100'
                  : 'text-zinc-400 hover:bg-zinc-800'
              }`}
            >
              <span className="truncate">{s.title}</span>
              <button
                onClick={(e) => { e.stopPropagation(); removeSession(s.id) }}
                className="invisible group-hover:visible text-zinc-600 hover:text-red-400 ml-1"
              >
                ×
              </button>
            </div>
          ))}
        </div>
      </div>

      <div className="mt-auto border-t border-zinc-800 p-2">
        <Button variant="ghost" size="sm" className="w-full justify-start gap-2" onClick={logout}>
          <ArrowRightOnRectangleIcon className="h-4 w-4" />
          Sign out
        </Button>
      </div>
    </aside>
  )
}
