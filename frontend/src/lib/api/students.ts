import { api } from './client'
import { Student } from '@/types/api'

export interface StudentInput {
  first_name: string
  last_name: string
  email?: string
  phone?: string
}

export const studentsApi = {
  list: () => api.get<Student[]>('/students').then((r) => r.data),
  get: (id: string) => api.get<Student>(`/students/${id}`).then((r) => r.data),
  create: (data: StudentInput) =>
    api.post<Student>('/students', data).then((r) => r.data),
  update: (id: string, data: StudentInput) =>
    api.put<Student>(`/students/${id}`, data).then((r) => r.data),
  delete: (id: string) => api.delete(`/students/${id}`).then(() => id),
}
