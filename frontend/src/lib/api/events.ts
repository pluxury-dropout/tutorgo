import { api } from './client'
import { Event, EventKind, RecurrenceInput, RecurrenceScope } from '@/types/api'

export interface EventInput {
  title: string
  kind?: EventKind
  starts_at: string
  duration_minutes: number
  color?: string
  location?: string
  notes?: string
  /** Только при создании: превращает событие в серию. */
  recurrence?: RecurrenceInput
}

export const eventsApi = {
  create: (data: EventInput) =>
    api.post<Event>('/events', data).then((r) => r.data),
  // scope нужен только вхождению серии; для одиночного события сервер его игнорирует.
  update: (id: string, data: EventInput, scope: RecurrenceScope = 'one') =>
    api.put<Event>(`/events/${id}`, data, { params: { scope } }).then((r) => r.data),
  delete: (id: string, scope: RecurrenceScope = 'one') =>
    api.delete(`/events/${id}`, { params: { scope } }).then(() => id),
}
