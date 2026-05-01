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
  scheduled: { bg: 'oklch(0.92 0.05 250)', border: 'oklch(0.55 0.14 250)', text: 'oklch(0.1 0.14 250)' },
  completed: { bg: 'oklch(0.85 0.09 155)', border: 'oklch(0.74 0.04 155)', text: 'oklch(0.1 0.05 155)' },
  cancelled: { bg: 'oklch(0.8 0.002 215)', border: 'oklch(0.78 0.004 215)', text: 'oklch(0.1 0.004 215)' },
  missed:    { bg: 'oklch(0.96 0.018 25)',  border: 'oklch(0.74 0.07 25)',  text: 'oklch(0.58 0.08 25)'  },
}
