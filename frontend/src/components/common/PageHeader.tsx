import { LucideIcon } from 'lucide-react'

interface PageHeaderProps {
  title: string
  description?: string
  actions?: React.ReactNode
  icon?: LucideIcon
  iconBg?: string
  iconColor?: string
}

export function PageHeader({ title, description, actions, icon: Icon, iconBg, iconColor }: PageHeaderProps) {
  return (
    <div className="flex items-start justify-between mb-6">
      <div className="flex items-center gap-3">
        {Icon && (
          <div
            className="h-9 w-9 rounded-xl flex items-center justify-center shrink-0"
            style={{ background: iconBg ?? 'var(--primary-light)' }}
          >
            <Icon className="h-[18px] w-[18px]" style={{ color: iconColor ?? 'var(--primary)' }} />
          </div>
        )}
        <div>
          <h1 className="text-[22px] font-bold tracking-[-0.4px]">{title}</h1>
          {description && (
            <p className="text-sm text-muted-foreground mt-1">{description}</p>
          )}
        </div>
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  )
}
