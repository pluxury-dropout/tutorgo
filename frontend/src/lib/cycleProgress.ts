import type { CalendarLesson } from '@/types/api'

export interface CourseCycle {
  courseId: string
  subject: string
  /** Уроков цикла уже позади */
  done: number
  /** Всего уроков в оплаченном цикле */
  size: number
}

interface Ref {
  courseId: string
  subject: string
  at: number
  future: boolean
  position: number
  size: number
}

/**
 * Прогресс текущего оплаченного цикла по каждому курсу.
 *
 * `cycle_position` приходит с бэка как номер урока внутри цикла (1..cycle_size),
 * посчитанный от платежей — см. computeCyclePositions в service/cycle.go. Курсов
 * у ученика может быть несколько, и цикл у каждого свой, поэтому считаем по
 * course_id, а не по первому попавшемуся уроку.
 *
 * Опорный урок курса — ближайший будущий: раз он ещё не проведён, позади
 * position-1 уроков. Если будущих не осталось, берём последний прошедший — он
 * сам уже позади, значит done = position.
 */
export function cycleProgress(lessons: CalendarLesson[], now = Date.now()): CourseCycle[] {
  const best = new Map<string, Ref>()

  for (const l of lessons) {
    const { cycle_position: position, cycle_size: size } = l
    if (position == null || size == null) continue

    const at = new Date(l.scheduled_at).getTime()
    const cur: Ref = {
      courseId: l.course_id,
      subject: l.subject,
      at,
      future: at + l.duration_minutes * 60_000 >= now,
      position,
      size,
    }

    const prev = best.get(l.course_id)
    // Будущий опорный урок всегда бьёт прошедший; при равенстве — среди будущих
    // берём самый ранний, среди прошедших самый поздний.
    if (
      !prev ||
      (cur.future && !prev.future) ||
      (cur.future === prev.future && (cur.future ? cur.at < prev.at : cur.at > prev.at))
    ) {
      best.set(l.course_id, cur)
    }
  }

  return [...best.values()]
    .map((r) => ({
      courseId: r.courseId,
      subject: r.subject,
      done: r.future ? r.position - 1 : r.position,
      size: r.size,
    }))
    .sort((a, b) => a.subject.localeCompare(b.subject, 'ru'))
}
