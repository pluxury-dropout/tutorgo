import { User, Users } from 'lucide-react'

interface Props {
  isGroup: boolean
}

export function CourseTypeBadge({ isGroup }: Props) {
  if (isGroup) {
    return (
      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium"
        style={{ background: 'var(--muted)', color: 'var(--muted-foreground)' }}>
        <Users className="h-3 w-3" />
        Групповой
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium"
      style={{ background: 'var(--primary-light)', color: 'var(--primary)' }}>
      <User className="h-3 w-3" />
      Индивидуальный
    </span>
  )
}
