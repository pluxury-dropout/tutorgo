import { api } from './client'
import { Student, PagedResponse, OnboardingStudentInput, OnboardingResult, StudentPause, StudentOverview } from '@/types/api'

export interface StudentInput {
  first_name: string
  last_name?: string
  email?: string
  phone?: string
}

export interface PauseInput {
  /** YYYY-MM-DD */
  starts_on: string
  /** YYYY-MM-DD, включительно */
  ends_on:   string
  reason?:   string
}

export interface StudentListParams {
  page:      number
  limit:     number
  search:    string
  archived?: boolean
}

export const studentsApi = {
  list: () =>
    api.get<PagedResponse<Student>>('/students', { params: { page: 1, limit: 100 } })
      .then((r) => r.data.data),
  listPaged: (p: StudentListParams) =>
    api.get<PagedResponse<Student>>('/students', { params: p }).then((r) => r.data),
  get: (id: string) => api.get<Student>(`/students/${id}`).then((r) => r.data),
  overview: (id: string) => api.get<StudentOverview>(`/students/${id}/overview`).then((r) => r.data),
  create: (data: StudentInput) =>
    api.post<Student>('/students', data).then((r) => r.data),
  update: (id: string, data: StudentInput) =>
    api.put<Student>(`/students/${id}`, data).then((r) => r.data),
  delete: (id: string) => api.delete(`/students/${id}`).then(() => id),
  archive: (id: string) => api.post(`/students/${id}/archive`).then(() => id),
  restore: (id: string) => api.post(`/students/${id}/restore`).then(() => id),
  invite: (id: string) =>
    api.post<{ invite_token: string; expires_at: string }>(`/students/${id}/invite`)
      .then((r) => r.data),
  onboard: (data: OnboardingStudentInput) =>
    api.post<OnboardingResult>('/onboarding/student', data).then((r) => r.data),
  pauses: (id: string) =>
    api.get<StudentPause[]>(`/students/${id}/pauses`).then((r) => r.data ?? []),
  createPause: (id: string, data: PauseInput) =>
    api.post<StudentPause>(`/students/${id}/pauses`, {
      starts_on: data.starts_on + 'T00:00:00Z',
      ends_on:   data.ends_on + 'T00:00:00Z',
      reason:    data.reason?.trim() || undefined,
    }).then((r) => r.data),
  deletePause: (id: string, pauseId: string) =>
    api.delete(`/students/${id}/pauses/${pauseId}`).then(() => pauseId),
}
