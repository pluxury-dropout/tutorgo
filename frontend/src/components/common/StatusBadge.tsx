import { cn } from '@/lib/utils'
import { LessonStatus } from '@/types/api'
import { STATUS_LABELS, STATUS_COLORS } from '@/lib/lessonStatus'

type CourseStatus = 'active' | 'ended'
type Status = LessonStatus | CourseStatus

const STYLES: Record<Status, string> = {
  ...STATUS_COLORS,
  active: 'bg-[var(--status-completed-bg)] text-[var(--status-completed-text)]',
  ended:  'bg-[var(--status-cancelled-bg)] text-[var(--status-cancelled-text)]',
}

const LABELS: Record<Status, string> = {
  ...STATUS_LABELS,
  active: 'Активный',
  ended:  'Завершён',
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
