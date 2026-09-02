import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { eventsApi, EventInput } from '@/lib/api/events'
import { calendarApi, ConflictQuery } from '@/lib/api/calendar'
import type { RecurrenceScope } from '@/types/api'

// События живут только в ленте календаря — отдельного кэша под них нет,
// поэтому любая мутация просто инвалидирует ['calendar'].
function useInvalidateCalendar() {
  const qc = useQueryClient()
  return () => qc.invalidateQueries({ queryKey: ['calendar'] })
}

export function useCreateEvent() {
  const invalidate = useInvalidateCalendar()
  return useMutation({
    mutationFn: (data: EventInput) => eventsApi.create(data),
    onSuccess:  invalidate,
  })
}

export function useUpdateEvent() {
  const invalidate = useInvalidateCalendar()
  return useMutation({
    mutationFn: ({ id, data, scope }: { id: string; data: EventInput; scope?: RecurrenceScope }) =>
      eventsApi.update(id, data, scope),
    onSuccess: invalidate,
  })
}

export function useDeleteEvent() {
  const invalidate = useInvalidateCalendar()
  return useMutation({
    mutationFn: ({ id, scope }: { id: string; scope?: RecurrenceScope }) => eventsApi.delete(id, scope),
    onSuccess:  invalidate,
  })
}

/** Что уже занимает слот. Запрос уходит сразу при открытии поповера. */
export function useConflicts(q: ConflictQuery | null) {
  return useQuery({
    queryKey: ['calendar', 'conflicts', q],
    queryFn:  () => calendarApi.conflicts(q!),
    enabled:  !!q,
    staleTime: 30_000,
  })
}
