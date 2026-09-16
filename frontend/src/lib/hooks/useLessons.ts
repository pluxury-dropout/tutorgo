import { useQuery, useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { lessonsApi, LessonInput, LessonUpdateInput } from '@/lib/api/lessons'
import {
  patchCalendarEntry, insertFeedEntry, removeCalendarEntries, rollbackCalendar, tempId,
} from '@/lib/hooks/useCalendar'
import { studentKeys } from '@/lib/hooks/useStudents'
import type { Lesson, RecurrenceScope } from '@/types/api'

export const lessonKeys = {
  byCourse:   (courseId: string) => ['lessons', 'course', courseId] as const,
  detail:     (id: string)       => ['lessons', id] as const,
  attendance: (lessonId: string) => ['lessons', lessonId, 'attendance'] as const,
}

// Любая правка уроков видна на двух экранах: список курса и календарь
// (он же «Уроки сегодня» на дашборде). Сбрасывать только lessons мало —
// у ['calendar'] свой ключ и staleTime 2 минуты, поэтому созданная серия
// не появлялась в расписании, пока кэш не протухнет сам.
function invalidateLessonViews(qc: QueryClient, courseId: string, studentId?: string) {
  qc.invalidateQueries({ queryKey: lessonKeys.byCourse(courseId) })
  qc.invalidateQueries({ queryKey: ['calendar'] })
  if (studentId) qc.invalidateQueries({ queryKey: studentKeys.overview(studentId) })
}

export function useLessons(courseId: string) {
  return useQuery({
    queryKey: lessonKeys.byCourse(courseId),
    queryFn:  () => lessonsApi.list(courseId),
    enabled:  !!courseId,
  })
}

export function useLessonsPaged(courseId: string, page: number) {
  return useQuery({
    queryKey: [...lessonKeys.byCourse(courseId), page] as const,
    queryFn:  () => lessonsApi.listPaged(courseId, page),
    enabled:  !!courseId,
  })
}

export function useLessonsByPeriod(courseId: string, from: string, to: string) {
  return useQuery({
    queryKey: [...lessonKeys.byCourse(courseId), 'period', from, to] as const,
    queryFn:  () => lessonsApi.listByPeriod(courseId, from, to),
    enabled:  !!courseId && !!from && !!to,
  })
}

export function useLesson(id: string) {
  return useQuery({
    queryKey: lessonKeys.detail(id),
    queryFn:  () => lessonsApi.get(id),
    enabled:  !!id,
  })
}

export function useAttendance(lessonId: string) {
  return useQuery({
    queryKey: lessonKeys.attendance(lessonId),
    queryFn:  () => lessonsApi.getAttendance(lessonId),
    enabled:  !!lessonId,
  })
}

export function useCreateLesson(courseId: string, studentId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: LessonInput) => lessonsApi.create(data),
    onSuccess:  () => invalidateLessonViews(qc, courseId, studentId),
  })
}

// Урок из календаря: course_id заранее неизвестен — курс может создаться на
// лету, — поэтому инвалидируем оба дерева целиком, а не конкретный курс.
// Серия отличается только полем recurrence: раскатку дат делает сервер.
export function useCreateSlotLesson() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: LessonInput): Promise<Lesson> => lessonsApi.create(data),
    // В блок пишем предмет: имя ученика сервер подставит в title при рефетче,
    // а из формы сюда приходит только его id. Серия показывается первым
    // вхождением — остальные даты раскатывает сервер.
    onMutate: (data) => {
      const id = tempId()
      return insertFeedEntry(qc, {
        id, type: 'lesson',
        title:            data.subject ?? 'Урок',
        starts_at:        data.scheduled_at,
        duration_minutes: data.duration_minutes,
        lesson: {
          id,
          course_id:        data.course_id ?? '',
          scheduled_at:     data.scheduled_at,
          duration_minutes: data.duration_minutes,
          status:           'scheduled',
          notes:            data.notes ?? '',
          subject:          data.subject ?? 'Урок',
          student_name:     null,
          is_group:         false,
        },
      }).then((previousEntries) => ({ previousEntries }))
    },
    onError: (_err, _vars, ctx) => rollbackCalendar(qc, ctx?.previousEntries),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['lessons'] })
      qc.invalidateQueries({ queryKey: ['calendar'] })
    },
  })
}

export function useUpdateLesson(id: string, courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ data, scope }: { data: LessonUpdateInput; scope?: RecurrenceScope }) =>
      lessonsApi.update(id, data, scope),
    onMutate: ({ data }) =>
      patchCalendarEntry(
        qc, id,
        { starts_at: data.scheduled_at, duration_minutes: data.duration_minutes },
        { status: data.status, notes: data.notes },
      ).then((previousEntries) => ({ previousEntries })),
    onError: (_err, _vars, ctx) => rollbackCalendar(qc, ctx?.previousEntries),
    onSuccess: (updated) => {
      invalidateLessonViews(qc, courseId)
      qc.setQueryData(lessonKeys.detail(id), updated)
    },
  })
}

export function useDeleteLesson(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, scope }: { id: string; scope?: RecurrenceScope }) => lessonsApi.delete(id, scope),
    onMutate:  ({ id }) =>
      removeCalendarEntries(qc, (e) => e.id === id).then((previousEntries) => ({ previousEntries })),
    onError:   (_err, _vars, ctx) => rollbackCalendar(qc, ctx?.previousEntries),
    onSuccess: () => invalidateLessonViews(qc, courseId),
  })
}

export function useDeleteLessonsByCourse(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => lessonsApi.deleteByCourse(courseId),
    // Курс у записи ленты лежит во вложенном уроке — по нему и чистим.
    onMutate:  () =>
      removeCalendarEntries(qc, (e) => {
        const lesson = e.lesson as { course_id?: string } | undefined
        return e.type === 'lesson' && lesson?.course_id === courseId
      }).then((previousEntries) => ({ previousEntries })),
    onError:   (_err, _vars, ctx) => rollbackCalendar(qc, ctx?.previousEntries),
    onSuccess: () => invalidateLessonViews(qc, courseId),
  })
}

export function useUpdateAttendance(lessonId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (attendances: { student_id: string; status: string }[]) =>
      lessonsApi.updateAttendance(lessonId, attendances),
    onSuccess: () => qc.invalidateQueries({ queryKey: lessonKeys.attendance(lessonId) }),
  })
}
