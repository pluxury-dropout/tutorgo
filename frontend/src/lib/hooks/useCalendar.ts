import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { calendarApi } from '@/lib/api/calendar'
import { lessonsApi, LessonUpdateInput } from '@/lib/api/lessons'

export function useCalendar(from: string, to: string) {
  return useQuery({
    queryKey: ['calendar', from, to],
    queryFn:  () => calendarApi.list(from, to),
    enabled:  !!from && !!to,
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
      qc.setQueriesData<any[]>({ queryKey: ['calendar'] }, (old) => {
        if (!old) return old
        return old.map((lesson) =>
          lesson.id === id
            ? { ...lesson, scheduled_at: data.scheduled_at, duration_minutes: data.duration_minutes }
            : lesson,
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
