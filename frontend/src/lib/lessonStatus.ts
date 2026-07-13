import type { LessonStatus } from '@/types/api'

// Урок «проведён» — детерминированная функция от времени: бэкенд-горутина
// проставляет completed раз в минуту, но клиент может показать это мгновенно.
export function effectiveStatus(l: { status: LessonStatus; scheduled_at: string; duration_minutes: number }): LessonStatus {
  if (l.status !== 'scheduled') return l.status
  const endsAt = new Date(l.scheduled_at).getTime() + l.duration_minutes * 60_000
  return Date.now() > endsAt ? 'completed' : 'scheduled'
}

export const STATUS_LABELS: Record<LessonStatus, string> = {
  scheduled: 'Запланирован',
  completed: 'Проведён',
  cancelled: 'Отменён',
  missed:    'Пропущен',
}

export const STATUS_COLORS: Record<LessonStatus, string> = {
  scheduled: 'bg-[var(--status-scheduled-bg)] text-[var(--status-scheduled-text)]',
  completed: 'bg-[var(--status-completed-bg)] text-[var(--status-completed-text)]',
  cancelled: 'bg-[var(--status-cancelled-bg)] text-[var(--status-cancelled-text)]',
  missed:    'bg-[var(--status-missed-bg)]    text-[var(--status-missed-text)]',
}

export const FC_COLORS: Record<LessonStatus, { bg: string; border: string; text: string }> = {
  scheduled: { bg: 'var(--cal-scheduled-bg)', border: 'var(--cal-scheduled-border)', text: 'var(--cal-scheduled-text)' },
  completed: { bg: 'var(--cal-completed-bg)', border: 'var(--cal-completed-border)', text: 'var(--cal-completed-text)' },
  cancelled: { bg: 'var(--cal-cancelled-bg)', border: 'var(--cal-cancelled-border)', text: 'var(--cal-cancelled-text)' },
  missed:    { bg: 'var(--cal-missed-bg)',     border: 'var(--cal-missed-border)',     text: 'var(--cal-missed-text)'     },
}
