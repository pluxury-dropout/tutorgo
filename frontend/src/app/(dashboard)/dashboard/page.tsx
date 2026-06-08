'use client'

import { useMemo } from 'react'
import Link from 'next/link'
import { useStudentCount } from '@/lib/hooks/useStudents'
import { useCourseCount } from '@/lib/hooks/useCourses'
import { useCalendar, useCurrentCycles } from '@/lib/hooks/useCalendar'
import { useRecentPayments, useMonthlyIncome, useMonthlyExpected } from '@/lib/hooks/usePayments'
import type { CalendarLesson } from '@/types/api'

function buildDateRanges() {
  const now = new Date()
  const y = now.getFullYear()
  const m = now.getMonth()
  const d = now.getDate()
  const dow = now.getDay() === 0 ? 6 : now.getDay() - 1
  return {
    todayFrom:  new Date(y, m, d).toISOString(),
    todayTo:    new Date(y, m, d, 23, 59, 59).toISOString(),
    weekStart:  new Date(y, m, d - dow).toISOString(),
    weekEnd:    new Date(y, m, d - dow + 6, 23, 59, 59).toISOString(),
    monthStart: new Date(y, m, 1).toISOString(),
    monthEnd:   new Date(y, m + 1, 0, 23, 59, 59).toISOString(),
    dateLabel:  now.toLocaleDateString('ru-RU', {
      weekday: 'long', day: 'numeric', month: 'long', year: 'numeric',
    }),
  }
}

function fmt(iso: string) {
  return new Date(iso).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
}

function fmtAmt(n: number) {
  return '₸ ' + n.toLocaleString('ru-RU')
}

function fmtDate(iso: string) {
  return new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' }).replace(' ', ' ')
}

const DOT_STYLE: React.CSSProperties = {
  display: 'inline-block', borderRadius: '50%', flexShrink: 0,
  width: 6, height: 6,
}

const MONO = 'ui-monospace, "SF Mono", "JetBrains Mono", Menlo, monospace'

const STATUS_LABEL: Record<string, string> = {
  scheduled: 'Запланирован',
  completed: 'Завершён',
  cancelled: 'Отменён',
  missed:    'Пропущен',
}

const STATUS_COLOR: Record<string, string> = {
  scheduled: 'var(--primary)',
  completed: 'var(--success)',
  cancelled: 'var(--muted-foreground)',
  missed:    'var(--warning)',
}

function LessonRow({ lesson, isFirst }: { lesson: CalendarLesson; isFirst: boolean }) {
  const cancelled = lesson.status === 'cancelled'
  const statusColor = STATUS_COLOR[lesson.status] ?? 'var(--muted-foreground)'
  const statusLabel = STATUS_LABEL[lesson.status] ?? lesson.status
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: '42px 1fr auto',
        alignItems: 'center',
        gap: 12,
        padding: '8px 0',
        borderTop: isFirst ? 'none' : '1px solid var(--border)',
      }}
    >
      <span style={{ fontFamily: MONO, fontSize: 12.5, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
        {fmt(lesson.scheduled_at)}
      </span>
      <span style={{ minWidth: 0, display: 'flex', alignItems: 'baseline', gap: 8 }}>
        <span style={{
          fontSize: 14, fontWeight: 600,
          color: cancelled ? 'var(--muted-foreground)' : 'var(--foreground)',
          textDecoration: cancelled ? 'line-through' : 'none',
          whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
        }}>
          {lesson.subject}
        </span>
        {lesson.student_name && (
          <span style={{ fontSize: 12.5, color: 'var(--muted-foreground)', whiteSpace: 'nowrap', flexShrink: 0 }}>
            {lesson.student_name}
          </span>
        )}
      </span>
      <span style={{
        fontSize: 10.5, fontWeight: 600, letterSpacing: '0.06em', textTransform: 'uppercase',
        color: statusColor,
        whiteSpace: 'nowrap',
      }}>
        {statusLabel}
      </span>
    </div>
  )
}

const WIDGET_HEAD: React.CSSProperties = {
  display: 'flex', alignItems: 'baseline', justifyContent: 'space-between',
  paddingBottom: 11, marginBottom: 4, borderBottom: '1px solid var(--border)',
}
const WIDGET_TITLE: React.CSSProperties = {
  margin: 0, fontSize: 15, fontWeight: 600,
  color: 'var(--foreground)', letterSpacing: '-0.01em',
}
const WIDGET_LINK: React.CSSProperties = {
  fontSize: 13, color: 'var(--muted-foreground)', textDecoration: 'none', fontWeight: 500,
}
const EMPTY: React.CSSProperties = {
  fontSize: 13, color: 'var(--muted-foreground)', textAlign: 'center', padding: '24px 0',
}

export default function DashboardPage() {
  const { todayFrom, todayTo, weekStart, weekEnd, monthStart, monthEnd, dateLabel } = useMemo(buildDateRanges, [])
  const { data: studentCount  = 0  } = useStudentCount()
  const { data: courseCount   = 0  } = useCourseCount()
  const { data: todayLessons  = [] } = useCalendar(todayFrom, todayTo)
  const { data: weekLessons   = [] } = useCalendar(weekStart, weekEnd)
  const { data: monthLessons  = [] } = useCalendar(monthStart, monthEnd)
  const { data: recentPayments = [] } = useRecentPayments()
  const { data: monthlyIncome  = 0  } = useMonthlyIncome()
  const { data: monthlyExpected = 0 } = useMonthlyExpected()
  const { data: currentCycles = [] } = useCurrentCycles()

  const sortedLessons = useMemo(
    () => [...todayLessons].sort((a, b) =>
      new Date(a.scheduled_at).getTime() - new Date(b.scheduled_at).getTime()),
    [todayLessons],
  )

  const metrics = [
    { color: 'var(--primary)',  value: todayLessons.length,        label: 'уроков сегодня' },
    { color: 'var(--warning)',  value: fmtAmt(monthlyExpected),    label: 'доход'          },
    { color: 'var(--purple)',   value: studentCount,                label: 'учеников'       },
    { color: 'var(--success)',  value: courseCount,                 label: 'курсов'         },
  ]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20, maxWidth: 900 }}>

      {/* V5 Reductive Header */}
      <div style={{ borderBottom: '1px solid var(--border)', paddingBottom: 14 }}>
        <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 24, flexWrap: 'wrap' }}>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
            <span style={{ fontSize: 17, fontWeight: 600, letterSpacing: '-0.01em', color: 'var(--foreground)' }}>
              Главная
            </span>
            <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>{dateLabel}</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 18, flexWrap: 'wrap' }}>
            {metrics.map((m, i) => (
              <span key={i} style={{ display: 'inline-flex', alignItems: 'baseline', gap: 7, fontVariantNumeric: 'tabular-nums' }}>
                <span style={{ ...DOT_STYLE, background: m.color, marginBottom: 1 }} />
                <span style={{ fontSize: 15, fontWeight: 650 as React.CSSProperties['fontWeight'], color: 'var(--foreground)' }}>
                  {m.value}
                </span>
                <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>{m.label}</span>
              </span>
            ))}
          </div>
        </div>
        <div style={{ marginTop: 8, fontSize: 12.5, color: 'var(--muted-foreground)', textAlign: 'right', fontVariantNumeric: 'tabular-nums', opacity: 0.75 }}>
          неделя {weekLessons.length} · месяц {monthLessons.length} уроков · получено {fmtAmt(monthlyIncome)} из {fmtAmt(monthlyExpected)}
        </div>
      </div>

      {/* Table Widgets */}
      <div className="grid grid-cols-1 md:grid-cols-2" style={{ gap: 36, alignItems: 'start' }}>

        {/* Уроки сегодня */}
        <section style={{ display: 'flex', flexDirection: 'column' }}>
          <header style={WIDGET_HEAD}>
            <h2 style={WIDGET_TITLE}>Уроки сегодня</h2>
            <Link href="/calendar" style={WIDGET_LINK}>Расписание →</Link>
          </header>
          {sortedLessons.length === 0
            ? <p style={EMPTY}>Уроков на сегодня нет</p>
            : sortedLessons.map((l, i) => <LessonRow key={l.id} lesson={l} isFirst={i === 0} />)
          }
        </section>

        {/* Последние платежи */}
        <section style={{ display: 'flex', flexDirection: 'column' }}>
          <header style={WIDGET_HEAD}>
            <h2 style={WIDGET_TITLE}>Последние платежи</h2>
            <Link href="/payments" style={WIDGET_LINK}>Все →</Link>
          </header>
          {recentPayments.length === 0
            ? <p style={EMPTY}>Платежей пока нет</p>
            : recentPayments.map((p, i) => (
                <div
                  key={p.id}
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '1fr auto 64px',
                    alignItems: 'baseline',
                    gap: 12,
                    padding: '8px 0',
                    borderTop: i === 0 ? 'none' : '1px solid var(--border)',
                  }}
                >
                  <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--foreground)', fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap' }}>
                    {fmtAmt(p.amount)}
                  </span>
                  <span style={{ fontSize: 12.5, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
                    {p.lessons_count} ур.
                  </span>
                  <span style={{ fontSize: 12.5, color: 'var(--muted-foreground)', textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
                    {fmtDate(p.paid_at)}
                  </span>
                </div>
              ))
          }
        </section>

        {/* Текущие циклы */}
        <section style={{ display: 'flex', flexDirection: 'column' }}>
          <header style={WIDGET_HEAD}>
            <h2 style={WIDGET_TITLE}>Текущие циклы</h2>
            <Link href="/courses" style={WIDGET_LINK}>Курсы →</Link>
          </header>
          {currentCycles.length === 0
            ? <p style={EMPTY}>Нет активных циклов</p>
            : currentCycles.map((cycle, i) => {
                const complete = cycle.progress === cycle.cycle_size
                return (
                  <div
                    key={cycle.course_id}
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '1fr auto 52px',
                      alignItems: 'center',
                      gap: 12,
                      padding: '8px 0',
                      borderTop: i === 0 ? 'none' : '1px solid var(--border)',
                    }}
                  >
                    <div>
                      <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--foreground)' }}>
                        {cycle.subject}
                      </span>
                      {cycle.student_name && (
                        <div style={{ fontSize: 12.5, color: 'var(--muted-foreground)', marginTop: 1 }}>
                          {cycle.student_name}
                        </div>
                      )}
                    </div>
                    <span style={{
                      fontSize: 12,
                      fontWeight: 600,
                      padding: '2px 7px',
                      borderRadius: 6,
                      border: '1px solid',
                      whiteSpace: 'nowrap',
                      color:       complete ? 'var(--success)' : 'var(--foreground)',
                      borderColor: complete ? 'color-mix(in srgb, var(--success) 40%, transparent)' : 'var(--border)',
                      background:  complete ? 'color-mix(in srgb, var(--success) 10%, transparent)' : 'var(--muted)',
                    }}>
                      {cycle.progress} / {cycle.cycle_size}
                    </span>
                    <span style={{
                      fontSize: 12.5,
                      color: 'var(--muted-foreground)',
                      textAlign: 'right',
                      fontVariantNumeric: 'tabular-nums',
                    }}>
                      {fmtDate(cycle.last_at)}
                    </span>
                  </div>
                )
              })
          }
        </section>

      </div>
    </div>
  )
}
