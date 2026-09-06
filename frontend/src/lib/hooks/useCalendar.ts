import { useQuery, useMutation, useQueryClient, keepPreviousData, type QueryClient } from '@tanstack/react-query'
import { calendarApi } from '@/lib/api/calendar'
import { lessonsApi, LessonUpdateInput } from '@/lib/api/lessons'
import { patchEntry, feedRange, inRange, type CalendarEntry, type TimePatch } from '@/lib/calendarPatch'

/** Статус урока из поповера: «провёл», «отменил». Красится сразу — ждать
 *  сервер ради смены цвета блока незачем. */
export function useUpdateLessonStatus(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: LessonUpdateInput) => lessonsApi.update(id, data),
    onMutate: (data) =>
      patchCalendarEntry(
        qc, id,
        { starts_at: data.scheduled_at, duration_minutes: data.duration_minutes },
        { status: data.status, notes: data.notes },
      ).then((previousEntries) => ({ previousEntries })),
    onError:   (_err, _vars, ctx) => rollbackCalendar(qc, ctx?.previousEntries),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['calendar'] }),
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

/** Слепок кэша календаря для отката: пары [ключ, данные]. */
export type CalendarSnapshot = ReturnType<QueryClient['getQueriesData']>

/** Оптимистичный сдвиг записи календаря — общий для уроков, событий и задач,
 *  поэтому чинит оба календаря разом: и FullCalendar, и мобильная сетка
 *  рисуются из этих же запросов. Форма записи после сдвига — в patchEntry. */
export async function patchCalendarEntry(
  qc: QueryClient,
  id: string,
  patch: TimePatch,
  extra?: Record<string, unknown>,
): Promise<CalendarSnapshot> {
  // Иначе запрос, уже летящий к серверу, приземлится поверх патча.
  await qc.cancelQueries({ queryKey: ['calendar'] })
  const previous = qc.getQueriesData({ queryKey: ['calendar'] })

  qc.setQueriesData<CalendarEntry[]>({ queryKey: ['calendar'] }, (old) =>
    Array.isArray(old) ? old.map((entry) => (entry.id === id ? patchEntry(entry, patch, extra) : entry)) : old,
  )
  return previous
}

/** Оптимистичное удаление: по id, а у «удалить все уроки курса» — по предикату.
 *  Вхождение серии удаляем одно, даже если scope шире: остальные подчистит
 *  инвалидация, а показать мгновенно можно только то, по чему кликнули. */
export async function removeCalendarEntries(
  qc: QueryClient,
  match: (entry: CalendarEntry) => boolean,
): Promise<CalendarSnapshot> {
  await qc.cancelQueries({ queryKey: ['calendar'] })
  const previous = qc.getQueriesData({ queryKey: ['calendar'] })

  qc.setQueriesData<CalendarEntry[]>({ queryKey: ['calendar'] }, (old) =>
    Array.isArray(old) ? old.filter((entry) => !match(entry)) : old,
  )
  return previous
}

/** Оптимистичная вставка в ленту. Идём по ключам вручную, а не через
 *  setQueriesData: тому не видно ключа, а нам нужен диапазон — запись кладём
 *  только в те недели, куда она попадает по времени. Временный id живёт до
 *  инвалидации: рефетч заменяет массив целиком, чистить за собой не нужно. */
export async function insertFeedEntry(
  qc: QueryClient,
  entry: CalendarEntry & { starts_at: string },
): Promise<CalendarSnapshot> {
  await qc.cancelQueries({ queryKey: ['calendar'] })
  const previous = qc.getQueriesData<CalendarEntry[]>({ queryKey: ['calendar'] })

  for (const [key, data] of previous) {
    const range = feedRange(key)
    if (!range || !Array.isArray(data) || !inRange(entry.starts_at, range)) continue
    qc.setQueryData<CalendarEntry[]>(key, [...data, entry])
  }
  return previous
}

/** Временный id оптимистичной записи — до ответа сервера. */
export function tempId(): string {
  return `tmp-${Date.now()}`
}

export function rollbackCalendar(qc: QueryClient, snapshot?: CalendarSnapshot) {
  snapshot?.forEach(([key, val]) => qc.setQueryData(key, val))
}

export function useRescheduleLesson() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: LessonUpdateInput }) =>
      lessonsApi.update(id, data),
    onMutate: ({ id, data }) =>
      patchCalendarEntry(
        qc, id,
        { starts_at: data.scheduled_at, duration_minutes: data.duration_minutes },
        { status: data.status, notes: data.notes },
      ).then((previousEntries) => ({ previousEntries })),
    onError: (_err, _vars, ctx) => rollbackCalendar(qc, ctx?.previousEntries),
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
