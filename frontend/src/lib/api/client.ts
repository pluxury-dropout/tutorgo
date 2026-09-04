import axios, { AxiosError } from 'axios'
import { ApiError } from '@/types/api'

export const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'

export const api = axios.create({
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
// Once a refresh hard-fails (no valid session server-side), stop firing it on
// every request — otherwise the request interceptor re-triggers a doomed
// /auth/refresh per request, draining the auth rate limiter into 429s. Reset on
// page reload (which forceLogout triggers anyway).
let refreshFailed = false

function forceLogout(): void {
  localStorage.removeItem('tg_token')
  localStorage.removeItem('tg_user')
  if (typeof window !== 'undefined') window.location.href = '/login'
}

async function refreshToken(): Promise<string> {
  const { data } = await axios.post<{ access_token: string }>(
    `${BASE_URL}/auth/refresh`,
    {},
    { withCredentials: true },
  )
  localStorage.setItem('tg_token', data.access_token)
  return data.access_token
}

async function proactiveRefresh(): Promise<void> {
  if (isRefreshing || refreshFailed) return
  isRefreshing = true
  const p = refreshToken().then(
    () => undefined,
    (err: AxiosError) => {
      // Сервер ответил (401/429/5xx) → перестаём долбить до перезагрузки.
      // 401 = сессия мертва → разлогин. А вот обрыв связи (response нет) —
      // транзиентен: залипнуть на нём навсегда значит требовать перезахода
      // после каждого моргания мобильной сети.
      if (err.response) refreshFailed = true
      if (err.response?.status === 401) forceLogout()
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

// Сессионный токен репетитора, дождавшись возможного проактивного рефреша.
// Параметра-фолбэка тут больше нет: invite-токен доски — не «запасной вариант»,
// а адресный ключ, и подставлять его после localStorage значило отдавать
// приоритет чужой сессии (см. useExcalidrawSync.connect).
export async function getTokenAsync(): Promise<string | undefined> {
  if (refreshPromise) await refreshPromise
  if (typeof window === 'undefined') return undefined
  return localStorage.getItem('tg_token') ?? undefined
}

api.interceptors.request.use(async (config) => {
  const token =
    typeof window !== 'undefined' ? localStorage.getItem('tg_token') : null
  if (token) {
    const exp = getTokenExp(token)
    // refresh if less than 7 days remain
    if (exp && exp - Date.now() / 1000 < 7 * 24 * 60 * 60) {
      await proactiveRefresh()
      const fresh = localStorage.getItem('tg_token')
      config.headers.Authorization = `Bearer ${fresh ?? token}`
    } else {
      config.headers.Authorization = `Bearer ${token}`
    }
  }
  return config
})

api.interceptors.response.use(
  (r) => r,
  async (error: AxiosError<{ error: string } | Record<string, string>>) => {
    const isAuthRoute = error.config?.url?.startsWith('/auth/')

    if (error.response?.status === 401 && !isAuthRoute && !isRefreshing) {
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
          return api.request(error.config)
        }
      } catch {
        isRefreshing = false
        refreshPromise = null
        forceLogout()
      }
    }

    // 402 = подписка протухла между guard-проверкой и запросом. Страховка:
    // уводим на paywall. Не мешает 401/refresh (другой статус, ранний выход выше).
    if (error.response?.status === 402 && typeof window !== 'undefined') {
      if (window.location.pathname !== '/subscription') {
        window.location.href = '/subscription'
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
