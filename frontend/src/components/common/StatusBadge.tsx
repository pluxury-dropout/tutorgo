import { cn } from '@/lib/utils'
import { LessonStatus } from '@/types/api'

type CourseStatus = 'active' | 'ended'
type Status = LessonStatus | CourseStatus

const STYLES: Record<Status, string> = {
  scheduled: 'bg-[var(--status-scheduled-bg)] text-[var(--status-scheduled-text)]',
  completed: 'bg-[var(--status-completed-bg)] text-[var(--status-completed-text)]',
  cancelled: 'bg-[var(--status-cancelled-bg)] text-[var(--status-cancelled-text)]',
  missed:    'bg-[var(--status-missed-bg)]    text-[var(--status-missed-text)]',
  active:    'bg-[var(--status-completed-bg)] text-[var(--status-completed-text)]',
  ended:     'bg-[var(--status-cancelled-bg)] text-[var(--status-cancelled-text)]',
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
        'inline-flex items-center rounded-[20px] font-semibold',
        size === 'sm' ? 'px-2 py-px text-[11px]' : 'px-[9px] py-[3px] text-xs',
        STYLES[status],
      )}
    >
      {LABELS[status]}
    </span>
  )
}
