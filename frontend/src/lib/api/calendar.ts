import { api } from './client'
import { CalendarLesson } from '@/types/api'

export const calendarApi = {
  list: (from: string, to: string) =>
    api
      .get<CalendarLesson[]>('/calendar', { params: { from, to } })
      .then((r) => r.data),
}
