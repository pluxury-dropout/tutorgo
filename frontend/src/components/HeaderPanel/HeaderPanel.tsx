'use client'

import { cn } from '@/lib/utils'

export type KpiSegment = {
  id:       string
  label:    string
  value:    string
  dotColor: string
  delta?:   { value: string; direction: 'up' | 'down' | 'flat' }
  meta?:    string
  loading?: boolean
}

export type HeaderPanelProps = {
  title:           string
  subtitle?:       string
  segments:        KpiSegment[]
  activeSegment:   string
  onSegmentChange: (id: string) => void
  className?:      string
}

export function HeaderPanel({
  title,
  subtitle,
  segments,
  activeSegment,
  onSegmentChange,
  className,
}: HeaderPanelProps) {
  return (
    <div className={cn('bg-card rounded-[16px] border border-border overflow-hidden', className)}>
      <div className="flex items-center px-[18px] py-[14px] pb-[12px] border-b border-border">
        <div>
          <h1 className="text-xs font-bold tracking-[-0.3px]">{title}</h1>
          {subtitle && (
            <p className="text-[10px] font-medium text-muted-foreground">{subtitle}</p>
          )}
        </div>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4">
        {segments.map((seg, i) => {
          const isActive      = seg.id === activeSegment
          const isEven        = (i + 1) % 2 === 0
          const isFirstRow    = i < 2
          const isLastDesktop = i === segments.length - 1

          const deltaColor =
            seg.delta?.direction === 'up'
              ? 'text-[var(--success)]'
              : seg.delta?.direction === 'down'
              ? 'text-[var(--danger)]'
              : 'text-muted-foreground'

          return (
            <button
              key={seg.id}
              aria-label={`${seg.label}, ${seg.value}${seg.delta ? ', ' + seg.delta.value : ''}`}
              onClick={() => onSegmentChange(seg.id)}
              style={{ all: 'unset' }}
              className={cn(
                'cursor-pointer flex flex-col gap-1 px-[18px] py-[14px] relative',
                'border-border transition-colors duration-[120ms]',
                'focus-visible:outline-2 focus-visible:outline-[var(--primary)] focus-visible:outline-offset-[-2px]',
                !isEven && 'border-r',
                isFirstRow && 'border-b',
                'md:border-b-0',
                !isLastDesktop ? 'md:border-r' : 'md:border-r-0',
                isActive
                  ? 'bg-secondary'
                  : 'hover:bg-secondary',
              )}
            >
              <div className="flex items-center gap-[5px]">
                <span
                  aria-hidden="true"
                  className="w-1.5 h-1.5 rounded-full shrink-0"
                  style={{ background: seg.dotColor }}
                />
                <span className="text-[10px] font-medium text-muted-foreground">{seg.label}</span>
              </div>

              {seg.loading ? (
                <div className="h-5 w-14 bg-secondary animate-pulse rounded" />
              ) : (
                <span className="text-sm font-bold tracking-[-0.4px] leading-none">
                  {seg.value}
                </span>
              )}

              {!seg.loading && seg.delta && (
                <span className={cn('text-[10px]', deltaColor)}>{seg.delta.value}</span>
              )}

              {!seg.loading && seg.meta && (
                <span className="text-[10px] text-muted-foreground">{seg.meta}</span>
              )}

              {isActive && (
                <span
                  className="absolute bottom-0 left-0 right-0 h-[2px]"
                  style={{ background: 'var(--primary)' }}
                />
              )}
            </button>
          )
        })}
      </div>
    </div>
  )
}
