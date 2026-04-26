import { api } from './client'
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
