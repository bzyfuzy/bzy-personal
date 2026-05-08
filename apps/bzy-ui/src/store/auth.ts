import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { tokenStore } from '../api/client'
import * as authApi from '../api/auth'

interface AuthState {
  isAuthenticated: boolean
  login: (email: string, password: string) => Promise<void>
  loginWithKey: (apiKey: string) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      isAuthenticated: tokenStore.hasToken,

      login: async (email, password) => {
        await authApi.login(email, password)
        set({ isAuthenticated: true })
      },

      loginWithKey: (_apiKey) => {
        // API key auth is configured via VITE_API_KEY env; flag as authenticated
        set({ isAuthenticated: true })
      },

      logout: () => {
        authApi.logout()
        set({ isAuthenticated: false })
      },
    }),
    {
      name: 'bzy-auth',
      partialize: (s) => ({ isAuthenticated: s.isAuthenticated }),
    },
  ),
)
