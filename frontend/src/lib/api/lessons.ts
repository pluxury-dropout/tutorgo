import { api } from './client'
import { Lesson, LessonStatus, AttendanceRecord } from '@/types/api'

export interface LessonInput {
  course_id:        string
  scheduled_at:     string
  duration_minutes: number
  notes?:           string
}

export interface LessonUpdateInput {
  scheduled_at:     string
  duration_minutes: number
  status:           LessonStatus
  notes?:           string
}

export const lessonsApi = {
  list:   (courseId: string) =>
    api.get<Lesson[]>('/lessons', { params: { course_id: courseId } }).then((r) => r.data ?? []),
  get:    (id: string) =>
    api.get<Lesson>(`/lessons/${id}`).then((r) => r.data),
  create: (data: LessonInput) =>
    api.post<Lesson>('/lessons', data).then((r) => r.data),
  update: (id: string, data: LessonUpdateInput) =>
    api.put<Lesson>(`/lessons/${id}`, data).then((r) => r.data),
  delete: (id: string) =>
    api.delete(`/lessons/${id}`).then(() => id),
  getAttendance: (lessonId: string) =>
    api.get<AttendanceRecord[]>(`/lessons/${lessonId}/attendance`).then((r) => r.data ?? []),
  updateAttendance: (lessonId: string, attendances: { student_id: string; status: string }[]) =>
    api.put(`/lessons/${lessonId}/attendance`, { attendances }).then(() => undefined),
}
