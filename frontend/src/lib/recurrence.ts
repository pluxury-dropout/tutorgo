// Клиентская раскатка правила повторения в конкретные даты.
// Живёт здесь, а не в LessonForm, потому что нужна в двух местах: форма считает
// превью для кнопки, страница курса шлёт результат в POST /lessons/bulk.
// Временная механика — в фазе 3 правило переезжает в БД (recurrence_rules).

export type RecurrenceType = 'weekly_same' | 'weekly_custom' | 'every_n_weeks'

export interface RecurrenceOptions {
  type:    RecurrenceType
  days?:   number[]   // ISO weekdays: 1=Пн … 7=Вс (для weekly_custom)
  n?:      number     // интервал в неделях (для every_n_weeks)
  count?:  number     // не задан — генерируем до конца курса либо до горизонта
}

/** Потолок по датам, когда не заданы ни count, ни ended_at курса. */
export const MAX_HORIZON_MONTHS = 12
/** Второй предохранитель: сколько уроков максимум за один сабмит. */
export const MAX_OCCURRENCES = 200

/** Верхняя граница генерации; null — ограничения по дате нет (задан count). */
function horizonFor(base: Date, opts: RecurrenceOptions, courseEndAt?: string | null): Date | null {
  if (opts.count !== undefined) return null
  if (courseEndAt) {
    const end = new Date(courseEndAt)
    // ended_at хранится как полночь UTC; урок в этот день ещё внутри курса.
    end.setUTCHours(23, 59, 59, 999)
    return end
  }
  const end = new Date(base)
  end.setMonth(base.getMonth() + MAX_HORIZON_MONTHS)
  return end
}

export function generateDates(baseISO: string, opts: RecurrenceOptions, courseEndAt?: string | null): string[] {
  const base    = new Date(baseISO)
  const limit   = Math.min(opts.count ?? MAX_OCCURRENCES, MAX_OCCURRENCES)
  const endDate = horizonFor(base, opts, courseEndAt)

  function within(d: Date) {
    return !endDate || d <= endDate
  }

  const results: Date[] = []

  if (opts.type === 'weekly_same') {
    for (let i = 0; i < limit; i++) {
      const d = new Date(base)
      d.setDate(base.getDate() + 7 * i)
      if (!within(d)) break
      results.push(d)
    }
  } else if (opts.type === 'every_n_weeks') {
    const n = opts.n ?? 2
    for (let i = 0; i < limit; i++) {
      const d = new Date(base)
      d.setDate(base.getDate() + 7 * n * i)
      if (!within(d)) break
      results.push(d)
    }
  } else if (opts.type === 'weekly_custom') {
    const days = (opts.days ?? []).slice().sort((a, b) => a - b)
    if (days.length === 0) return []   // форма не даёт отправить; тихий один урок — хуже пустоты
    const jsDay  = base.getDay()
    const toMon  = jsDay === 0 ? -6 : 1 - jsDay
    const monday = new Date(base)
    monday.setDate(base.getDate() + toMon)
    monday.setHours(base.getHours(), base.getMinutes(), 0, 0)

    for (let week = 0; results.length < limit; week++) {
      for (const isoDay of days) {
        if (results.length >= limit) break
        const d = new Date(monday)
        d.setDate(monday.getDate() + 7 * week + (isoDay - 1))
        if (!within(d)) { week = 9999; break }
        if (d >= base) results.push(d)
      }
      if (week > 200) break
    }
  }

  return results.map((d) => d.toISOString())
}

const LESSON_PLURAL: Record<Intl.LDMLPluralRule, string> = {
  one:   'урок',
  few:   'урока',
  many:  'уроков',
  other: 'урока',
  two:   'урока',
  zero:  'уроков',
}
const pluralRules = new Intl.PluralRules('ru-RU')

/** «53 урока» / «1 урок» — склонение без ручной таблицы остатков. */
export function lessonsPlural(n: number): string {
  return `${n} ${LESSON_PLURAL[pluralRules.select(n)]}`
}
