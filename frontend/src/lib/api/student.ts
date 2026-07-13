import { studentHttp } from './studentClient'
import type { CalendarLesson, StudentCourse, StudentHomework } from '@/types/api'
import type { RoomTokenResponse } from './calls'

export interface StudentProfile {
  id: string
  first_name: string
  last_name: string
  phone: string
  username: string
}

export type LessonsFilter = 'upcoming' | 'past'

// withCredentials на auth-путях: backend ставит/читает refresh-cookie ученика.
export const studentApi = {
  acceptInvite: (data: { token: string; username: string; password: string }) =>
    studentHttp
      .post<{ access_token: string }>('/student/auth/accept-invite', data, { withCredentials: true })
      .then((r) => r.data),

  login: (data: { identifier: string; password: string }) =>
    studentHttp
      .post<{ access_token: string }>('/student/auth/login', data, { withCredentials: true })
      .then((r) => r.data),

  logout: () =>
    studentHttp.post('/student/auth/logout', {}, { withCredentials: true }).catch(() => {}),

  me: () => studentHttp.get<StudentProfile>('/student/me').then((r) => r.data),

  lessons: (filter: LessonsFilter) =>
    studentHttp
      .get<CalendarLesson[]>('/student/lessons', { params: { filter } })
      .then((r) => r.data),

  changePassword: (data: { old_password: string; new_password: string }) =>
    studentHttp
      .post<{ access_token: string }>('/student/password', data, { withCredentials: true })
      .then((r) => r.data),

  roomToken: (lessonId: string) =>
    studentHttp
      .post<RoomTokenResponse>(`/student/lessons/${lessonId}/room-token`)
      .then((r) => r.data),

  homework: () =>
    studentHttp.get<StudentHomework[]>('/student/homework').then((r) => r.data),

  boardToken: (lessonId: string) =>
    studentHttp
      .get<{ invite_token: string; page_id: string }>(`/student/lessons/${lessonId}/board-token`)
      .then((r) => r.data),

  courses: () => studentHttp.get<StudentCourse[]>('/student/courses').then((r) => r.data),

  courseBoardToken: (courseId: string) =>
    studentHttp
      .get<{ invite_token: string; page_id: string }>(`/student/courses/${courseId}/board-token`)
      .then((r) => r.data),
}
