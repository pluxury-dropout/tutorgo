import { cn } from '@/lib/utils'
import { LessonStatus } from '@/types/api'

type CourseStatus = 'active' | 'ended'
type Status = LessonStatus | CourseStatus

const STYLES: Record<Status, string> = {
  scheduled: 'bg-[var(--status-scheduled-bg)] text-[var(--status-scheduled-text)] ring-[var(--status-scheduled-ring)]',
  completed: 'bg-[var(--status-completed-bg)] text-[var(--status-completed-text)] ring-[var(--status-completed-ring)]',
  cancelled: 'bg-[var(--status-cancelled-bg)] text-[var(--status-cancelled-text)] ring-[var(--status-cancelled-ring)]',
  missed:    'bg-[var(--status-missed-bg)]    text-[var(--status-missed-text)]    ring-[var(--status-missed-ring)]',
  active:    'bg-[var(--status-completed-bg)] text-[var(--status-completed-text)] ring-[var(--status-completed-ring)]',
  ended:     'bg-[var(--status-cancelled-bg)] text-[var(--status-cancelled-text)] ring-[var(--status-cancelled-ring)]',
}

const LABELS: Record<Status, string> = {
  scheduled: 'Запланирован',
  completed: 'Завершён',
  cancelled: 'Отменён',
  missed:    'Пропущен',
  active:    'Активный',
  ended:     'Завершён',
}

interface StatusBadgeProps {
  status: Status
  size?: 'sm' | 'md'
}

export function StatusBadge({ status, size = 'md' }: StatusBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full font-medium ring-1',
        size === 'sm' ? 'px-2 py-0.5 text-xs' : 'px-2.5 py-0.5 text-xs',
        STYLES[status],
      )}
    >
      {LABELS[status]}
    </span>
  )
}
