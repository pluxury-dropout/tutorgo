import { api, BASE_URL } from './client'
import { Tutor } from '@/types/api'

export interface TutorUpdateInput {
  email: string
  first_name: string
  last_name: string
  phone?: string
}

export interface ChangePasswordInput {
  current_password: string
  new_password:     string
}

export const tutorsApi = {
  get: (id: string) => api.get<Tutor>(`/tutors/${id}`).then((r) => r.data),
  update: (id: string, data: TutorUpdateInput) =>
    api.put<Tutor>(`/tutors/${id}`, data).then((r) => r.data),
  changePassword: (id: string, data: ChangePasswordInput) =>
    api.put(`/tutors/${id}/password`, data),
  delete: (id: string) => api.delete(`/tutors/${id}`).then(() => id),
}

// Ссылку собирает клиент: API отдаёт только токен, а хост знает лишь
// фронтенд (NEXT_PUBLIC_API_URL). См. докблок EnsureLink в handlers/ics.go.
export const icsApi = {
  ensureLink: () => api.post<{ token: string }>('/ics/link').then((r) => r.data.token),
  revokeLink: () => api.delete('/ics/link').then(() => undefined),
}

export function icsFeedUrl(token: string): string {
  return `${BASE_URL}/ics/${token}`
}
