import { api } from './client'
import { Course, CourseBalance, Enrollment, PagedResponse } from '@/types/api'

export interface CourseInput {
  student_id?: string
  subject: string
  price_per_lesson: number
  started_at: string
  ended_at?: string
}

export interface CourseListParams {
  page:   number
  limit:  number
  search: string
}

export const coursesApi = {
  list: () =>
    api.get<PagedResponse<Course>>('/courses', { params: { page: 1, limit: 100 } })
      .then((r) => r.data.data ?? []),
  listPaged: (p: CourseListParams) =>
    api.get<PagedResponse<Course>>('/courses', { params: p }).then((r) => r.data),
  get: (id: string) => api.get<Course>(`/courses/${id}`).then((r) => r.data),
  create: (data: CourseInput) =>
    api.post<Course>('/courses', data).then((r) => r.data),
  update: (id: string, data: Omit<CourseInput, 'student_id'>) =>
    api.put<Course>(`/courses/${id}`, data).then((r) => r.data),
  delete: (id: string) => api.delete(`/courses/${id}`).then(() => id),
  getBalance: (id: string) =>
    api.get<CourseBalance>(`/payments/balance?course_id=${id}`).then((r) => r.data),
  getEnrollments: (id: string) =>
    api.get<Enrollment[]>(`/courses/${id}/enrollments`).then((r) => r.data ?? []),
  addEnrollment: (courseId: string, studentId: string) =>
    api.post<Enrollment>(`/courses/${courseId}/enrollments`, { student_id: studentId })
      .then((r) => r.data),
  removeEnrollment: (courseId: string, studentId: string) =>
    api.delete(`/courses/${courseId}/enrollments/${studentId}`).then(() => studentId),
  listByStudent: (studentId: string) =>
    api.get<Course[]>(`/students/${studentId}/courses`).then((r) => r.data ?? []),
}
