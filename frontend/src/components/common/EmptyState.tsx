import Link from 'next/link'
import { LucideIcon } from 'lucide-react'
import { Button, buttonVariants } from '@/components/ui/button'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'

type Action =
  | { label: string; onClick: () => void }
  | { label: string; href: string }

interface EmptyStateProps {
  icon: LucideIcon
  title: string
  /** Зачем нужен раздел и что тут появится — новичок видит пустой экран впервые. */
  description?: string
  action?: Action
  /** sm — для узких карточек дашборда, md — для полноразмерных страниц. */
  size?: 'sm' | 'md'
}

export function EmptyState({ icon: Icon, title, description, action, size = 'md' }: EmptyStateProps) {
  const sm = size === 'sm'
  return (
    <Empty className={sm ? 'gap-3 px-4 py-8' : 'py-14'}>
      <EmptyHeader className={sm ? 'gap-1.5' : undefined}>
        <EmptyMedia variant="icon" className={sm ? 'size-9 rounded-full' : 'size-11 rounded-full'}>
          <Icon className={sm ? 'size-4' : 'size-5'} />
        </EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        {description && (
          <EmptyDescription className={sm ? 'text-xs/relaxed' : undefined}>
            {description}
          </EmptyDescription>
        )}
      </EmptyHeader>
      {action && (
        <EmptyContent>
          {'href' in action ? (
            <Link href={action.href} className={buttonVariants({ variant: 'outline', size: 'sm' })}>
              {action.label}
            </Link>
          ) : (
            <Button size="sm" onClick={action.onClick}>{action.label}</Button>
          )}
        </EmptyContent>
      )}
    </Empty>
  )
}
