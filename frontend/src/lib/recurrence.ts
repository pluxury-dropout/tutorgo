// UI-режимы повтора и перевод их в правило для сервера.
// Раскатки дат на клиенте больше нет: с фазы 3 правило живёт в БД
// (recurrence_rules), вхождения материализует бэкенд, а бессрочную серию
// дотягивает ночная джоба — клиенту нечего и не для чего считать.

import type { RecurrenceInput } from '@/types/api'

export type RecurrenceType = 'weekly_same' | 'weekly_custom' | 'every_n_weeks'

export interface RecurrenceOptions {
  type:    RecurrenceType
  days?:   number[]   // ISO weekdays: 1=Пн … 7=Вс (для weekly_custom)
  n?:      number     // интервал в неделях (для every_n_weeks)
  count?:  number     // не задан — серия бессрочная либо до конца курса
}

/** Дни недели в ISO-нумерации — нужны и форме урока, и поповеру календаря. */
export const WEEK_DAYS = [
  { label: 'Пн', iso: 1 },
  { label: 'Вт', iso: 2 },
  { label: 'Ср', iso: 3 },
  { label: 'Чт', iso: 4 },
  { label: 'Пт', iso: 5 },
  { label: 'Сб', iso: 6 },
  { label: 'Вс', iso: 7 },
]

/** ISO-день даты: 1=Пн … 7=Вс (Date.getDay() считает с воскресенья). */
export function isoWeekday(d: Date): number {
  return ((d.getDay() + 6) % 7) + 1
}

/** UI-режим повтора → правило для сервера. */
export function toRecurrenceInput(opts: RecurrenceOptions): RecurrenceInput {
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone
  const base = { freq: 'weekly' as const, tz, max_count: opts.count }

  if (opts.type === 'weekly_custom') return { ...base, byweekday: opts.days }
  if (opts.type === 'every_n_weeks') return { ...base, interval_n: opts.n ?? 2 }
  return base
}
