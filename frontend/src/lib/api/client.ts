import axios, { AxiosError } from 'axios'
import { ApiError } from '@/types/api'

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'

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

async function proactiveRefresh(): Promise<void> {
  if (isRefreshing) return
  isRefreshing = true
  const p = (async () => {
    try {
      const { data } = await axios.post<{ access_token: string }>(
        `${BASE_URL}/auth/refresh`,
        {},
        { withCredentials: true },
      )
      localStorage.setItem('tg_token', data.access_token)
    } catch {
      // silently ignore — reactive 401 handler will log out if needed
    }
  })()
  refreshPromise = p
  try {
    await p
  } finally {
    isRefreshing = false
    refreshPromise = null
  }
}

export async function getTokenAsync(fallback?: string): Promise<string | undefined> {
  if (refreshPromise) await refreshPromise
  return localStorage.getItem('tg_token') ?? fallback ?? undefined
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
      try {
        const { data } = await axios.post<{ access_token: string }>(
          `${BASE_URL}/auth/refresh`,
          {},
          { withCredentials: true },
        )
        localStorage.setItem('tg_token', data.access_token)
        isRefreshing = false

        if (error.config) {
          error.config.headers = error.config.headers ?? {}
          error.config.headers['Authorization'] = `Bearer ${data.access_token}`
          return api.request(error.config)
        }
      } catch {
        isRefreshing = false
        localStorage.removeItem('tg_token')
        localStorage.removeItem('tg_user')
        if (typeof window !== 'undefined') window.location.href = '/login'
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
