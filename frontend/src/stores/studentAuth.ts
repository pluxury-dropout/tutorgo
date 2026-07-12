import { create } from 'zustand'
import type { StudentProfile } from '@/lib/api/student'

interface StudentAuthState {
  token: string | null
  user: StudentProfile | null
  setAuth: (token: string, user: StudentProfile) => void
  clearAuth: () => void
}

const hydrate = () => {
  if (typeof window === 'undefined') return { token: null, user: null }
  const token = localStorage.getItem('tg_student_token')
  const raw = localStorage.getItem('tg_student_user')
  const user = raw ? (JSON.parse(raw) as StudentProfile) : null
  return { token, user }
}

export const useStudentAuthStore = create<StudentAuthState>((set) => ({
  ...hydrate(),
  setAuth: (token, user) => {
    localStorage.setItem('tg_student_token', token)
    localStorage.setItem('tg_student_user', JSON.stringify(user))
    set({ token, user })
  },
  clearAuth: () => {
    localStorage.removeItem('tg_student_token')
    localStorage.removeItem('tg_student_user')
    set({ token: null, user: null })
  },
}))
