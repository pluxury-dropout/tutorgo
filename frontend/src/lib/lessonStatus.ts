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
  scheduled: { bg: 'oklch(0.92 0.05 250)', border: 'oklch(0.55 0.14 250)', text: 'oklch(0.35 0.14 250)' },
  completed: { bg: 'oklch(0.91 0.07 155)', border: 'oklch(0.48 0.12 155)', text: 'oklch(0.32 0.10 155)' },
  cancelled: { bg: 'oklch(0.93 0.003 215)', border: 'oklch(0.60 0.01 215)', text: 'oklch(0.45 0.01 215)' },
  missed:    { bg: 'oklch(0.92 0.05 25)',  border: 'oklch(0.58 0.20 25)',  text: 'oklch(0.38 0.18 25)'  },
}
