import { useEffect } from 'react'
import { useChatStore } from '../store/chat'
import { ChatView } from '../components/chat/ChatView'
import { TopBar } from '../components/layout/TopBar'

export function ChatPage() {
  const { sessions, activeId, newSession, setActive } = useChatStore()

  useEffect(() => {
    if (sessions.length === 0) {
      newSession()
    } else if (!activeId) {
      setActive(sessions[0].id)
    }
  }, [sessions, activeId, newSession, setActive])

  const activeSession = sessions.find((s) => s.id === activeId)

  if (!activeSession) {
    return <div className="flex-1 bg-zinc-950" />
  }

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <TopBar title={activeSession.title} />
      <ChatView sessionId={activeSession.id} />
    </div>
  )
}
