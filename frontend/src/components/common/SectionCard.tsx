import Link from 'next/link'

interface SectionCardProps {
  title?: React.ReactNode
  /** Ссылка или кнопка справа в шапке карточки */
  action?: React.ReactNode
  /** Внутренний отступ тела; по умолчанию 0 — строки идут edge-to-edge */
  bodyPadding?: string | number
  children: React.ReactNode
  className?: string
  style?: React.CSSProperties
}

/**
 * Обведённая карточка-секция: рамка + радиус, опциональная шапка с разделителем,
 * тело со строками. Единый визуальный «формат» для всех страниц.
 */
export function SectionCard({ title, action, bodyPadding = 0, children, className, style }: SectionCardProps) {
  return (
    <section
      className={className}
      style={{
        border: '1px solid var(--border)',
        borderRadius: 12,
        background: 'var(--card)',
        overflow: 'hidden',
        ...style,
      }}
    >
      {(title || action) && (
        <header
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 12,
            padding: '14px 18px',
            borderBottom: '1px solid var(--border)',
          }}
        >
          {title && (
            <span style={{ fontSize: 13.5, fontWeight: 600, color: 'var(--foreground)', letterSpacing: '-0.01em' }}>
              {title}
            </span>
          )}
          {action}
        </header>
      )}
      <div style={{ padding: bodyPadding }}>{children}</div>
    </section>
  )
}

/** Ссылка-действие в шапке карточки (стиль «Расписание →») */
export function SectionLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <Link href={href} style={{ fontSize: 12, color: 'var(--muted-foreground)', textDecoration: 'none', fontWeight: 500 }}>
      {children}
    </Link>
  )
}

/** Строка тела карточки с разделителем сверху (кроме первой) */
export function SectionRow({
  children,
  isFirst,
  style,
  className,
  ...rest
}: {
  children: React.ReactNode
  isFirst?: boolean
  style?: React.CSSProperties
  className?: string
} & React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={className}
      style={{
        padding: '12px 18px',
        borderTop: isFirst ? 'none' : '1px solid var(--row-border)',
        ...style,
      }}
      {...rest}
    >
      {children}
    </div>
  )
}
