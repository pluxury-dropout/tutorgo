'use client'

import { useRouter } from 'next/navigation'
import { HandCoins } from 'lucide-react'

import { useDebts } from '@/lib/hooks/usePayments'
import { SectionCard } from '@/components/common/SectionCard'
import { EmptyState } from '@/components/common/EmptyState'
import { ErrorState } from '@/components/common/ErrorState'

const fmtAmt = (n: number) => '₸' + Math.round(n).toLocaleString('ru-RU')

const COLS = '1fr 110px 120px'

/** «Кто мне должен» (спека 2026-09-06, п. 6.5): строка на ученика с разбивкой по
 *  предметам, клик ведёт в карточку. Кнопки «Напомнить» нет — канал доставки не
 *  выбран. Ушедшие из группы и архивные тоже здесь: архив долг не прощает. */
export function DebtsList() {
  const router = useRouter()
  const { data: debts = [], isLoading, isError, refetch } = useDebts()

  return (
    <SectionCard>
      <div style={{
        display: 'grid', gridTemplateColumns: COLS, alignItems: 'baseline', gap: 12,
        padding: '10px 18px 8px', borderBottom: '1px solid var(--border)',
      }}>
        {(['Ученик', 'Урок', 'Долг'] as const).map((label, i) => (
          <span key={label} style={{
            fontSize: 11.5, fontWeight: 500, color: 'var(--muted-foreground)',
            letterSpacing: '0.05em', textTransform: 'uppercase',
            ...(i === 2 ? { textAlign: 'right' } : {}),
          }}>{label}</span>
        ))}
      </div>

      {isLoading ? (
        <div className="space-y-2" style={{ padding: '10px 18px' }}>
          {[...Array(3)].map((_, i) => <div key={i} className="h-4 rounded bg-muted animate-pulse" />)}
        </div>
      ) : isError ? (
        <ErrorState what="долги" onRetry={() => refetch()} />
      ) : debts.length === 0 ? (
        <EmptyState
          icon={HandCoins}
          title="Никто не должен"
          description="Здесь появятся ученики, у которых проведённых уроков больше, чем оплачено"
        />
      ) : (
        <>
          {debts.map((d, i) => (
            <div
              key={d.student_id}
              role="button"
              tabIndex={0}
              className="hover:bg-muted/30"
              style={{
                display: 'grid', gridTemplateColumns: COLS, alignItems: 'center', gap: 12,
                padding: '10px 18px', cursor: 'pointer',
                borderTop: i === 0 ? 'none' : '1px solid var(--row-border)',
              }}
              onClick={() => router.push(`/students/${d.student_id}`)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); router.push(`/students/${d.student_id}`) }
              }}
            >
              <div style={{ minWidth: 0 }}>
                <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--foreground)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {d.student_name}
                </div>
                <div style={{ fontSize: 12, color: 'var(--muted-foreground)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {d.courses.map((c) => `${c.subject} — ${c.lessons_owed} ур.`).join(' · ')}
                </div>
              </div>
              <span style={{ fontSize: 13, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
                {d.next_lesson_at
                  ? new Date(d.next_lesson_at).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
                  : '—'}
              </span>
              <div style={{ textAlign: 'right' }}>
                <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--foreground)', fontVariantNumeric: 'tabular-nums' }}>
                  {fmtAmt(d.amount_owed)}
                </div>
                <div style={{ fontSize: 12, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
                  {d.lessons_owed} ур.
                </div>
              </div>
            </div>
          ))}
          <p style={{ fontSize: 12, color: 'var(--muted-foreground)', padding: '8px 18px 12px', margin: 0 }}>
            Сумма — уроки × цена урока курса; у пакетов, которые не делятся на уроки нацело, она приблизительна.
          </p>
        </>
      )}
    </SectionCard>
  )
}
