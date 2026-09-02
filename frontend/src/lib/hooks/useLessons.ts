import { useQuery, useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { lessonsApi, LessonInput, LessonUpdateInput, SeriesUpdateInput } from '@/lib/api/lessons'
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
function invalidateLessonViews(qc: QueryClient, courseId: string) {
  qc.invalidateQueries({ queryKey: lessonKeys.byCourse(courseId) })
  qc.invalidateQueries({ queryKey: ['calendar'] })
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

export function useCreateLesson(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: LessonInput) => lessonsApi.create(data),
    onSuccess:  () => invalidateLessonViews(qc, courseId),
  })
}

// Урок из календаря: course_id заранее неизвестен — курс может создаться на
// лету, — поэтому инвалидируем оба дерева целиком, а не конкретный курс.
// Серия отличается только полем recurrence: раскатку дат делает сервер.
export function useCreateSlotLesson() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: LessonInput): Promise<Lesson> => lessonsApi.create(data),
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
    onSuccess:  () => invalidateLessonViews(qc, courseId),
  })
}

export function useDeleteLessonsByCourse(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => lessonsApi.deleteByCourse(courseId),
    onSuccess:  () => invalidateLessonViews(qc, courseId),
  })
}

export function useDeleteSeries(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ seriesId, fromDate, toDate }: { seriesId: string; fromDate?: string; toDate?: string }) =>
      lessonsApi.deleteSeries(seriesId, fromDate, toDate),
    onSuccess: () => invalidateLessonViews(qc, courseId),
  })
}

export function useUpdateSeries(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ seriesId, data }: { seriesId: string; data: SeriesUpdateInput }) =>
      lessonsApi.updateSeries(seriesId, data),
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
