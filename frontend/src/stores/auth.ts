import { create } from 'zustand'
import { Tutor } from '@/types/api'

interface AuthState {
  token: string | null
  user: Tutor | null
  setAuth: (token: string, user: Tutor) => void
  clearAuth: () => void
}

const hydrate = () => {
  if (typeof window === 'undefined') return { token: null, user: null }
  const token = localStorage.getItem('tg_token')
  const raw = localStorage.getItem('tg_user')
  const user = raw ? (JSON.parse(raw) as Tutor) : null
  return { token, user }
}

export const useAuthStore = create<AuthState>((set) => ({
  ...hydrate(),
  setAuth: (token, user) => {
    localStorage.setItem('tg_token', token)
    localStorage.setItem('tg_user', JSON.stringify(user))
    set({ token, user })
  },

  clearAuth: () => {
    localStorage.removeItem('tg_token')
    localStorage.removeItem('tg_user')
    set({ token: null, user: null })
  },
}))
