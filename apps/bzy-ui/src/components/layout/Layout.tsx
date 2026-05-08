import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { useWS } from '../../hooks/useWS'

export function Layout() {
  useWS()

  return (
    <div className="flex h-screen overflow-hidden bg-zinc-950 text-zinc-100">
      <Sidebar />
      <main className="flex flex-1 flex-col overflow-hidden">
        <Outlet />
      </main>
    </div>
  )
}
