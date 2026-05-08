import { createBrowserRouter, Navigate } from 'react-router-dom'
import { Layout } from './components/layout/Layout'
import { LoginPage } from './pages/LoginPage'
import { ChatPage } from './pages/ChatPage'
import { MemoryPage } from './pages/MemoryPage'
import { DocumentsPage } from './pages/DocumentsPage'
import { AgentsPage } from './pages/AgentsPage'
import { TasksPage } from './pages/TasksPage'
import { useAuthStore } from './store/auth'

function RequireAuth({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />,
  },
  {
    path: '/',
    element: (
      <RequireAuth>
        <Layout />
      </RequireAuth>
    ),
    children: [
      { index: true, element: <Navigate to="/chat" replace /> },
      { path: 'chat', element: <ChatPage /> },
      { path: 'memory', element: <MemoryPage /> },
      { path: 'documents', element: <DocumentsPage /> },
      { path: 'agents', element: <AgentsPage /> },
      { path: 'tasks', element: <TasksPage /> },
    ],
  },
])
