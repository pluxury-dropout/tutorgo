interface PageHeaderProps {
  title: string
  /** Инлайн справа от заголовка: счётчик с точкой-метрикой или подпись типа курса */
  meta?: React.ReactNode
  actions?: React.ReactNode
}

export function PageHeader({ title, meta, actions }: PageHeaderProps) {
  return (
    <div style={{ marginBottom: 18 }}>
      <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 24, flexWrap: 'wrap' }}>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 10, minWidth: 0 }}>
          <h1 style={{ margin: 0, fontSize: 20, fontWeight: 700, letterSpacing: '-0.01em', color: 'var(--foreground)' }}>
            {title}
          </h1>
          {meta && <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>{meta}</span>}
        </div>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </div>
    </div>
  )
}

const DOT: React.CSSProperties = {
  display: 'inline-block', width: 6, height: 6, borderRadius: '50%', flexShrink: 0,
}

/** Счётчик-метрика с цветной точкой — визуальная рифма с метриками на главной */
export function HeaderMetric({ color, children }: { color: string; children: React.ReactNode }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'baseline', gap: 7 }}>
      <span style={{ ...DOT, background: color, transform: 'translateY(-1px)' }} />
      {children}
    </span>
  )
}
