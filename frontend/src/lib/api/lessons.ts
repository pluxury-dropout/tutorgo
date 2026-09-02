import { api } from './client'
import { Lesson, LessonStatus, AttendanceRecord, PagedResponse, RecurrenceInput, RecurrenceScope } from '@/types/api'

// Урок ставится либо в известный курс (course_id), либо по паре «ученик +
// предмет» — тогда курс найдётся или создастся на бэкенде. Второй путь — это
// постановка из календаря, где про курсы пользователь не думает.
export interface LessonInput {
  course_id?:       string
  student_id?:      string
  subject?:         string
  scheduled_at:     string
  duration_minutes: number
  notes?:           string
  /** Серия: сервер сам заведёт правило и материализует горизонт. */
  recurrence?:      RecurrenceInput
}

export interface LessonUpdateInput {
  scheduled_at:     string
  duration_minutes: number
  status:           LessonStatus
  notes?:           string
}

export interface SeriesUpdateInput {
  from_date?:       string
  new_time?:        string
  duration_minutes?: number
  notes?:           string
}

export const lessonsApi = {
  list:   (courseId: string) =>
    api.get<Lesson[]>('/lessons', { params: { course_id: courseId } }).then((r) => r.data ?? []),
  listPaged: (courseId: string, page: number, limit = 10) =>
    api.get<PagedResponse<Lesson>>('/lessons', { params: { course_id: courseId, page, limit } })
       .then((r) => r.data ?? { data: [], total: 0, page, limit }),
  listByPeriod: (courseId: string, from: string, to: string) =>
    api.get<Lesson[]>('/lessons', { params: { course_id: courseId, from, to } })
       .then((r) => r.data ?? []),
  get:    (id: string) =>
    api.get<Lesson>(`/lessons/${id}`).then((r) => r.data),
  create: (data: LessonInput) =>
    api.post<Lesson>('/lessons', data).then((r) => r.data),
  // scope нужен только вхождению серии; одиночному уроку сервер его игнорирует.
  update: (id: string, data: LessonUpdateInput, scope: RecurrenceScope = 'one') =>
    api.put<Lesson>(`/lessons/${id}`, data, { params: { scope } }).then((r) => r.data),
  delete: (id: string, scope: RecurrenceScope = 'one') =>
    api.delete(`/lessons/${id}`, { params: { scope } }).then(() => id),
  deleteByCourse: (courseId: string) =>
    api.delete('/lessons', { params: { course_id: courseId } }).then(() => undefined),
  deleteSeries: (seriesId: string, fromDate?: string, toDate?: string) =>
    api.delete(`/lessons/series/${seriesId}`, {
      params: {
        ...(fromDate && { from: fromDate }),
        ...(toDate  && { to:   toDate  }),
      },
    }).then(() => undefined),
  updateSeries: (seriesId: string, data: SeriesUpdateInput) =>
    api.patch(`/lessons/series/${seriesId}`, data).then(() => undefined),
  getAttendance: (lessonId: string) =>
    api.get<AttendanceRecord[]>(`/lessons/${lessonId}/attendance`).then((r) => r.data ?? []),
  updateAttendance: (lessonId: string, attendances: { student_id: string; status: string }[]) =>
    api.put(`/lessons/${lessonId}/attendance`, { attendances }).then(() => undefined),
}
