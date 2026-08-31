import { api } from './client'
import { CalendarItem, CalendarLesson, CurrentCycleInfo } from '@/types/api'

/** Что занимает выбранный слот. Задачи занятостью не считаются. */
export interface ConflictQuery {
  starts_at: string
  duration_minutes: number
  exclude_type?: 'lesson' | 'event'
  exclude_id?: string
}

export const calendarApi = {
  /** Только уроки — для дашборда и мини-календаря в сайдбаре. */
  list: (from: string, to: string) =>
    api
      .get<CalendarLesson[]>('/calendar', { params: { from, to } })
      .then((r) => r.data ?? []),

  /** Единая лента: уроки + события + задачи со слотом. */
  feed: (from: string, to: string) =>
    api
      .get<{ items: CalendarItem[] }>('/calendar/feed', { params: { from, to } })
      .then((r) => r.data?.items ?? []),

  conflicts: (q: ConflictQuery) =>
    api
      .get<{ conflicts: CalendarItem[] }>('/calendar/conflicts', { params: q })
      .then((r) => r.data?.conflicts ?? []),

  getCurrentCycles: () =>
    api
      .get<CurrentCycleInfo[]>('/dashboard/cycles')
      .then((r) => r.data ?? []),
}
