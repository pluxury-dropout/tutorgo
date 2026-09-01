import type { EventKind } from '@/types/api'

export const KIND_LABELS: Record<EventKind, string> = {
  personal: 'Личное',
  work:     'Работа',
  trial:    'Пробный',
}

/** Цвет блока в сетке. Непустой event.color пользователя перекрывает эти значения. */
export const KIND_COLORS: Record<EventKind, { bg: string; border: string; text: string }> = {
  personal: { bg: 'var(--cal-personal-bg)', border: 'var(--cal-personal-border)', text: 'var(--cal-personal-text)' },
  work:     { bg: 'var(--cal-work-bg)',     border: 'var(--cal-work-border)',     text: 'var(--cal-work-text)'     },
  trial:    { bg: 'var(--cal-trial-bg)',    border: 'var(--cal-trial-border)',    text: 'var(--cal-trial-text)'    },
}

export const EVENT_KINDS: EventKind[] = ['personal', 'work', 'trial']

// Значения — в globals.css (--cal-task-*), рядом с цветами уроков и событий:
// одна тема, одно место правки, тёмная тема не забывается.
export const TASK_COLORS = {
  active: { bg: 'var(--cal-task-bg)',      border: 'var(--cal-task-border)',      text: 'var(--cal-task-text)' },
  done:   { bg: 'var(--cal-task-done-bg)', border: 'var(--cal-task-done-border)', text: 'var(--cal-task-done-text)' },
}

/** «17:00 – 18:30» — подпись пересечения в предупреждении о занятости. */
export function formatTimeRange(startsAt: string, durationMinutes: number): string {
  const start = new Date(startsAt)
  const end   = new Date(start.getTime() + durationMinutes * 60_000)
  const fmt   = (d: Date) => d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
  return `${fmt(start)} – ${fmt(end)}`
}
