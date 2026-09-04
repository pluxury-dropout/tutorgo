import { api } from './client'
import { Student, PagedResponse, OnboardingStudentInput, OnboardingResult } from '@/types/api'

export interface StudentInput {
  first_name: string
  last_name?: string
  email?: string
  phone?: string
}

export interface StudentListParams {
  page:   number
  limit:  number
  search: string
}

export const studentsApi = {
  list: () =>
    api.get<PagedResponse<Student>>('/students', { params: { page: 1, limit: 100 } })
      .then((r) => r.data.data),
  listPaged: (p: StudentListParams) =>
    api.get<PagedResponse<Student>>('/students', { params: p }).then((r) => r.data),
  get: (id: string) => api.get<Student>(`/students/${id}`).then((r) => r.data),
  create: (data: StudentInput) =>
    api.post<Student>('/students', data).then((r) => r.data),
  update: (id: string, data: StudentInput) =>
    api.put<Student>(`/students/${id}`, data).then((r) => r.data),
  delete: (id: string) => api.delete(`/students/${id}`).then(() => id),
  invite: (id: string) =>
    api.post<{ invite_token: string; expires_at: string }>(`/students/${id}/invite`)
      .then((r) => r.data),
  onboard: (data: OnboardingStudentInput) =>
    api.post<OnboardingResult>('/onboarding/student', data).then((r) => r.data),
}
