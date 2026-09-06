import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { eventsApi, EventInput } from '@/lib/api/events'
import { calendarApi, ConflictQuery } from '@/lib/api/calendar'
import {
  patchCalendarEntry, insertFeedEntry, removeCalendarEntries, rollbackCalendar, tempId,
} from '@/lib/hooks/useCalendar'
import type { RecurrenceScope } from '@/types/api'

// События живут только в ленте календаря — отдельного кэша под них нет,
// поэтому любая мутация просто инвалидирует ['calendar'].
function useInvalidateCalendar() {
  const qc = useQueryClient()
  return () => qc.invalidateQueries({ queryKey: ['calendar'] })
}

export function useCreateEvent() {
  const qc         = useQueryClient()
  const invalidate = useInvalidateCalendar()
  return useMutation({
    mutationFn: (data: EventInput) => eventsApi.create(data),
    // Серию показываем первым вхождением: остальные раскатывает сервер, их
    // принесёт инвалидация.
    onMutate: (data) => {
      const id = tempId()
      return insertFeedEntry(qc, {
        id, type: 'event',
        title:            data.title,
        starts_at:        data.starts_at,
        duration_minutes: data.duration_minutes,
        event: { ...data, id, kind: data.kind ?? 'personal' },
      }).then((previousEntries) => ({ previousEntries }))
    },
    onError:   (_err, _vars, ctx) => rollbackCalendar(qc, ctx?.previousEntries),
    onSuccess: invalidate,
  })
}

export function useUpdateEvent() {
  const qc         = useQueryClient()
  const invalidate = useInvalidateCalendar()
  return useMutation({
    mutationFn: ({ id, data, scope }: { id: string; data: EventInput; scope?: RecurrenceScope }) =>
      eventsApi.update(id, data, scope),
    // Правку одного вхождения показываем сразу — перетащенное событие иначе
    // отскакивает на старое место до ответа сервера. Правка серии (scope ≠ one)
    // трогает и соседние вхождения, их досчитает инвалидация.
    onMutate: ({ id, data }) =>
      patchCalendarEntry(
        qc, id,
        { starts_at: data.starts_at, duration_minutes: data.duration_minutes, title: data.title },
        data as unknown as Record<string, unknown>,
      ).then((previousEntries) => ({ previousEntries })),
    onError:   (_err, _vars, ctx) => rollbackCalendar(qc, ctx?.previousEntries),
    onSuccess: invalidate,
  })
}

export function useDeleteEvent() {
  const qc         = useQueryClient()
  const invalidate = useInvalidateCalendar()
  return useMutation({
    mutationFn: ({ id, scope }: { id: string; scope?: RecurrenceScope }) => eventsApi.delete(id, scope),
    onMutate:  ({ id }) =>
      removeCalendarEntries(qc, (e) => e.id === id).then((previousEntries) => ({ previousEntries })),
    onError:   (_err, _vars, ctx) => rollbackCalendar(qc, ctx?.previousEntries),
    onSuccess: invalidate,
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
