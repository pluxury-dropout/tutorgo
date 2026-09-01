import { useQuery, useMutation, useQueryClient, keepPreviousData } from '@tanstack/react-query'
import { calendarApi } from '@/lib/api/calendar'
import { lessonsApi, LessonUpdateInput } from '@/lib/api/lessons'

export function useUpdateLessonStatus(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: LessonUpdateInput) => lessonsApi.update(id, data),
    onSuccess:  () => qc.invalidateQueries({ queryKey: ['calendar'] }),
  })
}

/** Только уроки — дашборд и мини-календарь в сайдбаре. */
export function useCalendar(from: string, to: string) {
  return useQuery({
    queryKey:        ['calendar', from, to],
    queryFn:         () => calendarApi.list(from, to),
    enabled:         !!from && !!to,
    placeholderData: keepPreviousData,
  })
}

/** Единая лента: уроки, события и задачи со слотом — одним запросом. */
export function useCalendarFeed(from: string, to: string) {
  return useQuery({
    queryKey:        ['calendar', 'feed', from, to],
    queryFn:         () => calendarApi.feed(from, to),
    enabled:         !!from && !!to,
    placeholderData: keepPreviousData,
  })
}

export function useRescheduleLesson() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: LessonUpdateInput }) =>
      lessonsApi.update(id, data),
    onMutate: async ({ id, data }) => {
      await qc.cancelQueries({ queryKey: ['calendar'] })
      const previousEntries = qc.getQueriesData<any[]>({ queryKey: ['calendar'] })
      // Под ключом ['calendar'] лежат две формы: список уроков (scheduled_at)
      // и лента (starts_at). Патчим оба поля — лишнее просто не читается.
      qc.setQueriesData<any[]>({ queryKey: ['calendar'] }, (old) => {
        if (!old) return old
        return old.map((entry) =>
          entry.id === id
            ? {
                ...entry,
                scheduled_at:     data.scheduled_at,
                starts_at:        data.scheduled_at,
                duration_minutes: data.duration_minutes,
              }
            : entry,
        )
      })
      return { previousEntries }
    },
    onError: (_err, _vars, ctx) => {
      ctx?.previousEntries.forEach(([key, val]) => qc.setQueryData(key, val))
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['calendar'] })
    },
  })
}

export function useCurrentCycles() {
  return useQuery({
    queryKey: ['dashboard', 'cycles'],
    queryFn:  () => calendarApi.getCurrentCycles(),
  })
}
