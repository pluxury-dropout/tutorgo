import { api } from './client'
import { Tutor } from '@/types/api'

export interface LoginInput {
  email?: string
  phone?: string
  password: string
}

export interface RegisterInput {
  email: string
  password: string
  first_name: string
  last_name: string
  phone?: string
}

export const authApi = {
  login: (data: LoginInput) =>
    api.post<{ token: string }>('/auth/login', data).then((r) => r.data),

  register: (data: RegisterInput) =>
    api.post<Tutor>('/auth/register', data).then((r) => r.data),
}
