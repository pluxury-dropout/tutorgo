import axios, { AxiosError } from 'axios'
import type { ApiError } from '@/types/api'

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'

// Изолированный клиент кабинета ученика: свой токен (tg_student_token) и свой
// refresh (/student/auth/refresh). Логика — упрощённая копия tutor-клиента
// (client.ts) без 402-ветки; tutor-стек не трогаем, чтобы оба логина жили в
// одном браузере независимо.
export const studentHttp = axios.create({
  baseURL: BASE_URL,
  headers: { 'Content-Type': 'application/json' },
})

function getTokenExp(token: string): number | null {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    return typeof payload.exp === 'number' ? payload.exp : null
  } catch {
    return null
  }
}

let isRefreshing = false
let refreshPromise: Promise<void> | null = null
// После жёсткого провала refresh не долбим /student/auth/refresh на каждый
// запрос (иначе rate limiter выдаст 429). Сбрасывается перезагрузкой страницы.
let refreshFailed = false

export function forceStudentLogout(): void {
  localStorage.removeItem('tg_student_token')
  localStorage.removeItem('tg_student_user')
  if (typeof window !== 'undefined') window.location.href = '/student/login'
}

async function refreshToken(): Promise<string> {
  const { data } = await axios.post<{ access_token: string }>(
    `${BASE_URL}/student/auth/refresh`,
    {},
    { withCredentials: true },
  )
  localStorage.setItem('tg_student_token', data.access_token)
  return data.access_token
}

async function proactiveRefresh(): Promise<void> {
  if (isRefreshing || refreshFailed) return
  isRefreshing = true
  const p = refreshToken().then(
    () => undefined,
    (err: AxiosError) => {
      refreshFailed = true
      if (err.response?.status === 401) forceStudentLogout()
    },
  )
  refreshPromise = p
  try {
    await p
  } finally {
    isRefreshing = false
    refreshPromise = null
  }
}

studentHttp.interceptors.request.use(async (config) => {
  const token =
    typeof window !== 'undefined' ? localStorage.getItem('tg_student_token') : null
  if (token) {
    const exp = getTokenExp(token)
    // refresh, если осталось меньше 7 дней
    if (exp && exp - Date.now() / 1000 < 7 * 24 * 60 * 60) {
      await proactiveRefresh()
      const fresh = localStorage.getItem('tg_student_token')
      config.headers.Authorization = `Bearer ${fresh ?? token}`
    } else {
      config.headers.Authorization = `Bearer ${token}`
    }
  }
  return config
})

/**
 * 401 на student-эндпоинте: пробовать refresh или отдать ошибку как есть.
 *
 * - Нет токена — запрос анонимный (гость пробного урока по ссылке): 401 значит
 *   «я не ученик», а не «сессия протухла». Рефрешить нечего, а forceStudentLogout
 *   увёл бы гостя прямо с урока на /student/login.
 * - /student/auth/* и /student/password отдают 401 по делу (неверный пароль) —
 *   refresh-retry на них зациклился бы.
 */
export function shouldRefreshOn401(url: string | undefined, hasToken: boolean): boolean {
  if (!hasToken) return false
  return !(url?.startsWith('/student/auth/') || url === '/student/password')
}

studentHttp.interceptors.response.use(
  (r) => r,
  async (error: AxiosError<{ error: string } | Record<string, string>>) => {
    const hasToken =
      typeof window !== 'undefined' && Boolean(localStorage.getItem('tg_student_token'))

    if (error.response?.status === 401 && shouldRefreshOn401(error.config?.url, hasToken) && !isRefreshing) {
      isRefreshing = true
      const p = refreshToken()
      refreshPromise = p.then(() => undefined)
      try {
        const freshToken = await p
        isRefreshing = false
        refreshPromise = null
        if (error.config) {
          error.config.headers = error.config.headers ?? {}
          error.config.headers['Authorization'] = `Bearer ${freshToken}`
          return studentHttp.request(error.config)
        }
      } catch {
        isRefreshing = false
        refreshPromise = null
        forceStudentLogout()
      }
    }

    const status = error.response?.status ?? 0
    const data = error.response?.data

    let normalized: ApiError
    if (data && typeof data === 'object' && 'error' in data) {
      normalized = { message: data.error as string, status }
    } else if (data && typeof data === 'object') {
      normalized = {
        message: 'Validation error',
        fieldErrors: data as Record<string, string>,
        status,
      }
    } else {
      normalized = { message: 'Unknown error', status }
    }

    return Promise.reject(normalized)
  },
)
