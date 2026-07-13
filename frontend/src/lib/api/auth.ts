import { api } from './client'

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
    api.post<{ access_token: string }>('/auth/login', data).then((r) => r.data),

  // Шаг 1: шлёт OTP-код на email, аккаунт ещё не создан (202).
  register: (data: RegisterInput) =>
    api.post('/auth/register', data).then(() => {}),

  // Шаг 2: подтверждает код, создаёт аккаунт, сразу логинит (201 + access_token).
  registerVerify: (email: string, code: string) =>
    api
      .post<{ access_token: string }>('/auth/register/verify', { email, code })
      .then((r) => r.data),

  registerResend: (email: string) =>
    api.post('/auth/register/resend', { email }).then(() => {}),

  logout: () =>
    api.post('/auth/logout', {}, { withCredentials: true }).catch(() => {}),
}
