import { api } from './client'
import { Event, EventKind } from '@/types/api'

export interface EventInput {
  title: string
  kind?: EventKind
  starts_at: string
  duration_minutes: number
  color?: string
  location?: string
  notes?: string
}

export const eventsApi = {
  create: (data: EventInput) =>
    api.post<Event>('/events', data).then((r) => r.data),
  update: (id: string, data: EventInput) =>
    api.put<Event>(`/events/${id}`, data).then((r) => r.data),
  delete: (id: string) =>
    api.delete(`/events/${id}`).then(() => id),
}
