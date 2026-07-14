import type { CalendarLesson } from '@/types/api'

/** За сколько минут до старта показываем кнопку. */
export const LEAD_MINUTES = 10

/**
 * Урок, который препод может начать прямо сейчас: ближайший запланированный,
 * чьё окно [старт − leadMinutes, старт + длительность] накрывает now.
 * Окно тянется до конца урока — препод мог опоздать или перезагрузить вкладку.
 */
export function pickActiveLesson(
  lessons: CalendarLesson[],
  now: Date,
  leadMinutes: number = LEAD_MINUTES,
): CalendarLesson | null {
  const t = now.getTime()

  const inWindow = lessons.filter((l) => {
    if (l.status !== 'scheduled') return false
    const start = new Date(l.scheduled_at).getTime()
    const from  = start - leadMinutes * 60_000
    const to    = start + l.duration_minutes * 60_000
    return t >= from && t <= to
  })

  if (inWindow.length === 0) return null

  return inWindow.reduce((best, l) =>
    new Date(l.scheduled_at).getTime() < new Date(best.scheduled_at).getTime() ? l : best
  )
}
