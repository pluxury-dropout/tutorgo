import { api } from './client'
import { CalendarLesson, CurrentCycleInfo } from '@/types/api'

export const calendarApi = {
  list: (from: string, to: string) =>
    api
      .get<CalendarLesson[]>('/calendar', { params: { from, to } })
      .then((r) => r.data ?? []),

  getCurrentCycles: () =>
    api
      .get<CurrentCycleInfo[]>('/dashboard/cycles')
      .then((r) => r.data ?? []),
}
