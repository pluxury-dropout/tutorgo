import { LessonStatus } from '@/types/api'

export const STATUS_LABELS: Record<LessonStatus, string> = {
  scheduled: 'Запланирован',
  completed: 'Проведён',
  cancelled: 'Отменён',
  missed:    'Пропущен',
}

export const STATUS_COLORS: Record<LessonStatus, string> = {
  scheduled: 'bg-blue-100 text-blue-700',
  completed: 'bg-green-100 text-green-700',
  cancelled: 'bg-gray-100 text-gray-500',
  missed:    'bg-red-100 text-red-700',
}

export const FC_COLORS: Record<LessonStatus, { bg: string; border: string; text: string }> = {
  scheduled: { bg: 'var(--cal-scheduled-bg)', border: 'var(--cal-scheduled-border)', text: 'var(--cal-scheduled-text)' },
  completed: { bg: 'var(--cal-completed-bg)', border: 'var(--cal-completed-border)', text: 'var(--cal-completed-text)' },
  cancelled: { bg: 'var(--cal-cancelled-bg)', border: 'var(--cal-cancelled-border)', text: 'var(--cal-cancelled-text)' },
  missed:    { bg: 'var(--cal-missed-bg)',     border: 'var(--cal-missed-border)',     text: 'var(--cal-missed-text)'     },
}
