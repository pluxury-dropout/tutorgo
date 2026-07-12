import { studentHttp } from './studentClient'
import type { CalendarLesson, LessonTask } from '@/types/api'
import type { RoomTokenResponse } from './calls'

export interface StudentProfile {
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

  tasks: (lessonId: string) =>
    studentHttp.get<LessonTask[]>(`/student/lessons/${lessonId}/tasks`).then((r) => r.data),

  setTaskDone: (taskId: string, done: boolean) =>
    studentHttp.patch<void>(`/student/lesson-tasks/${taskId}`, { done }),

  boardToken: (lessonId: string) =>
    studentHttp
      .get<{ invite_token: string; page_id: string }>(`/student/lessons/${lessonId}/board-token`)
      .then((r) => r.data),
}
