import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { APIError } from '../api/types'

export function LoginPage() {
  const [mode, setMode] = useState<'credentials' | 'apikey'>('credentials')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const login = useAuthStore((s) => s.login)
  const loginWithKey = useAuthStore((s) => s.loginWithKey)
  const navigate = useNavigate()

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      if (mode === 'apikey') {
        loginWithKey(apiKey)
      } else {
        await login(email, password)
      }
      navigate('/chat')
    } catch (err) {
      if (err instanceof APIError) {
        setError(err.message)
      } else {
        setError('Login failed. Check your credentials.')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-zinc-950">
      <div className="w-full max-w-sm space-y-6 rounded-2xl border border-zinc-800 bg-zinc-900 p-8">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-indigo-400">bzy</h1>
          <p className="mt-1 text-sm text-zinc-500">Sign in to continue</p>
        </div>

        <div className="flex rounded-lg border border-zinc-700 p-0.5">
          {(['credentials', 'apikey'] as const).map((m) => (
            <button
              key={m}
              onClick={() => setMode(m)}
              className={`flex-1 rounded-md py-1.5 text-xs font-medium transition-colors ${
                mode === m
                  ? 'bg-indigo-600 text-white'
                  : 'text-zinc-400 hover:text-zinc-200'
              }`}
            >
              {m === 'credentials' ? 'Email / Password' : 'API Key'}
            </button>
          ))}
        </div>

        <form onSubmit={submit} className="space-y-4">
          {mode === 'credentials' ? (
            <>
              <Input
                type="email"
                placeholder="Email"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
              <Input
                type="password"
                placeholder="Password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </>
          ) : (
            <Input
              placeholder="sk-..."
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              required
            />
          )}

          {error && <p className="text-xs text-red-400">{error}</p>}

          <Button type="submit" loading={loading} className="w-full">
            Sign in
          </Button>
        </form>
      </div>
    </div>
  )
}
