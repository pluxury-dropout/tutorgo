import axios, { AxiosError } from 'axios'
import { ApiError } from '@/types/api'

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'

export const api = axios.create({
  baseURL: BASE_URL,
  headers: { 'Content-Type': 'application/json' },
})

api.interceptors.request.use((config) => {
  const token =
    typeof window !== 'undefined' ? localStorage.getItem('tg_token') : null
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

let isRefreshing = false

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
