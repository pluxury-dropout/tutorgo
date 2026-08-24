'use client'

import { useMemo } from 'react'
import { useStudentCount } from '@/lib/hooks/useStudents'
import { useCourseCount } from '@/lib/hooks/useCourses'
import { useCalendar, useCurrentCycles } from '@/lib/hooks/useCalendar'
import { useRecentPayments, useMonthlyIncome, useMonthlyExpected } from '@/lib/hooks/usePayments'
import type { CalendarLesson } from '@/types/api'
import { SectionCard, SectionLink, SectionRow } from '@/components/common/SectionCard'
import { StatusBadge } from '@/components/common/StatusBadge'
import { EmptyState } from '@/components/common/EmptyState'
import { ErrorState } from '@/components/common/ErrorState'
import { GettingStarted } from '@/components/common/GettingStarted'
import { effectiveStatus } from '@/lib/lessonStatus'
import { useMinuteTick } from '@/lib/hooks/useMinuteTick'
import KanbanWidget from '@/components/tasks/KanbanWidget'
import { CalendarDays, RefreshCw, Wallet } from 'lucide-react'

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
  return '₸' + n.toLocaleString('ru-RU')
}

function fmtDate(iso: string) {
  return new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
}

const MONO = 'ui-monospace, "SF Mono", "JetBrains Mono", Menlo, monospace'

function LessonRow({ lesson, isFirst }: { lesson: CalendarLesson; isFirst: boolean }) {
  const status = effectiveStatus(lesson)
  const cancelled = status === 'cancelled'
  return (
    <SectionRow isFirst={isFirst} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
      <span style={{ fontFamily: MONO, fontSize: 12, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums', width: 38, flexShrink: 0 }}>
        {fmt(lesson.scheduled_at)}
      </span>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{
          fontSize: 13, fontWeight: 600,
          color: cancelled ? 'var(--muted-foreground)' : 'var(--foreground)',
          textDecoration: cancelled ? 'line-through' : 'none',
          whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
        }}>
          {lesson.subject}
        </div>
        <div style={{ fontSize: 12, color: 'var(--muted-foreground)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {lesson.is_group ? 'Групповой урок' : (lesson.student_name ?? '—')}
        </div>
      </div>
      <StatusBadge status={status} size="sm" />
    </SectionRow>
  )
}

function RowsSkeleton({ rows = 3 }: { rows?: number }) {
  return (
    <div className="space-y-2" style={{ padding: '12px 18px' }}>
      {[...Array(rows)].map((_, i) => (
        <div key={i} className="h-8 rounded bg-muted animate-pulse" />
      ))}
    </div>
  )
}

export default function DashboardPage() {
  useMinuteTick()
  const { todayFrom, todayTo, weekStart, weekEnd, monthStart, monthEnd, dateLabel } = useMemo(buildDateRanges, [])
  const { data: studentCount   = 0,  isPending: studentsPending } = useStudentCount()
  const { data: courseCount    = 0,  isPending: coursesPending  } = useCourseCount()
  const {
    data: todayLessons = [], isPending: todayPending,
    isError: todayError, refetch: refetchToday,
  } = useCalendar(todayFrom, todayTo)
  const { data: weekLessons    = [] } = useCalendar(weekStart, weekEnd)
  const { data: monthLessons   = [], isPending: monthPending    } = useCalendar(monthStart, monthEnd)
  const {
    data: recentPayments = [], isPending: paymentsPending,
    isError: paymentsError, refetch: refetchPayments,
  } = useRecentPayments()
  const { data: monthlyIncome   = 0 } = useMonthlyIncome()
  const { data: monthlyExpected = 0 } = useMonthlyExpected()
  const {
    data: currentCycles = [], isPending: cyclesPending,
    isError: cyclesError, refetch: refetchCycles,
  } = useCurrentCycles()

  // Дефолты выше (0 / []) неотличимы от «данных правда нет», поэтому до первого
  // ответа сервера пустые состояния и онбординг не рисуем — иначе они мелькают
  // у препода, у которого всё давно заполнено. isPending, а не isFetching:
  // фоновый рефетч данные из кеша не стирает и мигать не должен.
  const loading = studentsPending || coursesPending || todayPending
    || monthPending || paymentsPending || cyclesPending

  const sortedLessons = useMemo(
    () => [...todayLessons].sort((a, b) =>
      new Date(a.scheduled_at).getTime() - new Date(b.scheduled_at).getTime()),
    [todayLessons],
  )

  // monthlyIncome — уже пришедшие деньги, monthlyExpected — ещё ожидаемые в этом
  // месяце. Множества непересекающиеся, поэтому прогноз кассы — их сумма, а не
  // одно «из» другого.
  const monthlyForecast = monthlyIncome + monthlyExpected

  const kpis = [
    { color: 'var(--muted-foreground)', value: todayLessons.length,     label: 'уроков сегодня' },
    { color: 'var(--warning)',          value: fmtAmt(monthlyForecast), label: 'доход'          },
    { color: 'var(--purple)',           value: studentCount,            label: 'учеников'       },
    { color: 'var(--success)',          value: courseCount,             label: 'курсов'         },
  ]

  const incomePct = monthlyForecast > 0
    ? Math.min(100, Math.round((monthlyIncome / monthlyForecast) * 100))
    : 0

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>

      {/* Заголовок */}
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
        <h1 style={{ margin: 0, fontSize: 20, fontWeight: 700, color: 'var(--foreground)', letterSpacing: '-0.01em' }}>
          Главная
        </h1>
        <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>{dateLabel}</span>
      </div>

      {/* Онбординг — сам исчезает, когда все четыре шага сделаны */}
      {!loading && (
        <GettingStarted
          hasStudents={studentCount > 0}
          hasCourses={courseCount > 0}
          hasLessons={monthLessons.length > 0}
          hasPayments={recentPayments.length > 0}
        />
      )}

      {/* KPI + доход */}
      <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
        <div style={{
          border: '1px solid var(--border)', borderRadius: 12, background: 'var(--card)',
          padding: '10px 16px', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '6px 24px',
          width: 360, flexShrink: 0,
        }}>
          {kpis.map((k, i) => (
            <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 0' }}>
              <span style={{ width: 6, height: 6, borderRadius: '50%', background: k.color, flexShrink: 0 }} />
              <span style={{ fontSize: 13.5, fontWeight: 700, color: 'var(--foreground)', fontVariantNumeric: 'tabular-nums' }}>
                {k.value}
              </span>
              <span style={{ fontSize: 12, color: 'var(--muted-foreground)' }}>{k.label}</span>
            </div>
          ))}
        </div>

        <div style={{
          border: '1px solid var(--border)', borderRadius: 12, background: 'var(--card)',
          padding: '14px 18px', flex: 1, minWidth: 240,
          display: 'flex', flexDirection: 'column', justifyContent: 'center', gap: 8,
        }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
            <span style={{ fontSize: 12, color: 'var(--muted-foreground)' }}>получено за месяц</span>
            <span style={{ fontSize: 12, color: 'var(--foreground)', fontWeight: 600, fontVariantNumeric: 'tabular-nums' }}>
              {fmtAmt(monthlyIncome)} <span style={{ color: 'var(--muted-foreground)', fontWeight: 400 }}>из {fmtAmt(monthlyForecast)}</span>
            </span>
          </div>
          <div style={{ width: '100%', height: 5, background: 'var(--muted)', borderRadius: 3, overflow: 'hidden' }}>
            <div style={{ width: `${incomePct}%`, height: '100%', background: 'var(--success)', transition: 'width 0.3s' }} />
          </div>
          <span style={{ fontSize: 11, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
            неделя {weekLessons.length} · месяц {monthLessons.length} уроков
          </span>
        </div>
      </div>

      {/* Три карточки */}
      <div className="grid grid-cols-1 md:grid-cols-3" style={{ gap: 10, alignItems: 'start' }}>

        <SectionCard title="Уроки сегодня" action={<SectionLink href="/calendar">Расписание →</SectionLink>}>
          {loading
            ? <RowsSkeleton />
            : todayError
            ? <ErrorState size="sm" what="уроки" onRetry={() => refetchToday()} />
            : sortedLessons.length === 0
            ? <EmptyState
                size="sm"
                icon={CalendarDays}
                title="Уроков сегодня нет"
                description="Здесь появится расписание дня — время, ученик и статус каждого урока"
                action={{ label: 'Открыть календарь', href: '/calendar' }}
              />
            : sortedLessons.map((l, i) => <LessonRow key={l.id} lesson={l} isFirst={i === 0} />)
          }
        </SectionCard>

        <SectionCard title="Текущие циклы" action={<SectionLink href="/courses">Курсы →</SectionLink>}>
          {loading
            ? <RowsSkeleton />
            : cyclesError
            ? <ErrorState size="sm" what="циклы" onRetry={() => refetchCycles()} />
            : currentCycles.length === 0
            ? <EmptyState
                size="sm"
                icon={RefreshCw}
                title="Активных циклов нет"
                description="Цикл — пакет уроков по курсу, после которого пора брать оплату. Появится, как только пройдёт первый урок"
              />
            : currentCycles.map((cycle, i) => {
                const complete = cycle.progress === cycle.cycle_size
                return (
                  <SectionRow key={cycle.course_id} isFirst={i === 0} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--foreground)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        {cycle.subject}
                      </div>
                      {cycle.student_name && (
                        <div style={{ fontSize: 12, color: 'var(--muted-foreground)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                          {cycle.student_name}
                        </div>
                      )}
                    </div>
                    <span style={{
                      fontSize: 11.5, fontWeight: 600, padding: '4px 8px', borderRadius: 7,
                      whiteSpace: 'nowrap', flexShrink: 0, fontVariantNumeric: 'tabular-nums',
                      color:      complete ? 'var(--success)' : 'var(--foreground)',
                      background: complete ? 'var(--status-completed-bg)' : 'var(--muted)',
                    }}>
                      {cycle.progress} / {cycle.cycle_size}
                    </span>
                    <span style={{ fontSize: 11.5, color: 'var(--muted-foreground)', flexShrink: 0, width: 52, textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
                      {fmtDate(cycle.last_at)}
                    </span>
                  </SectionRow>
                )
              })
          }
        </SectionCard>

        <SectionCard title="Последние платежи" action={<SectionLink href="/payments">Все →</SectionLink>}>
          {loading
            ? <RowsSkeleton />
            : paymentsError
            ? <ErrorState size="sm" what="платежи" onRetry={() => refetchPayments()} />
            : recentPayments.length === 0
            ? <EmptyState
                size="sm"
                icon={Wallet}
                title="Платежей пока нет"
                description="Отмеченные оплаты появятся здесь — свежие сверху, с суммой и числом оплаченных уроков"
                action={{ label: 'Отметить оплату', href: '/payments' }}
              />
            : (
              <>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr auto 52px', gap: 8, padding: '8px 18px 6px' }}>
                  <span style={{ fontSize: 10, color: 'var(--muted-foreground)', textTransform: 'uppercase', letterSpacing: '0.04em' }}>Курс</span>
                  <span style={{ fontSize: 10, color: 'var(--muted-foreground)', textTransform: 'uppercase', letterSpacing: '0.04em', textAlign: 'right' }}>Сумма</span>
                  <span style={{ fontSize: 10, color: 'var(--muted-foreground)', textTransform: 'uppercase', letterSpacing: '0.04em', textAlign: 'right' }}>Дата</span>
                </div>
                {recentPayments.map((p, i) => (
                  <SectionRow key={p.id} isFirst={i === 0} style={{ display: 'grid', gridTemplateColumns: '1fr auto 52px', gap: 8, alignItems: 'center', padding: '9px 18px' }}>
                    <div style={{ minWidth: 0 }}>
                      <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--foreground)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        {p.subject ?? '—'}
                      </div>
                      <div style={{ fontSize: 12, color: 'var(--muted-foreground)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        {p.student_name ?? 'Группа'}
                      </div>
                    </div>
                    <div style={{ textAlign: 'right' }}>
                      <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--foreground)', fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap' }}>
                        {fmtAmt(p.amount)}
                      </div>
                      <div style={{ fontSize: 11.5, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
                        {p.lessons_count} ур.
                      </div>
                    </div>
                    <span style={{ fontSize: 11.5, color: 'var(--muted-foreground)', textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
                      {fmtDate(p.paid_at)}
                    </span>
                  </SectionRow>
                ))}
              </>
            )
          }
        </SectionCard>

      </div>

      {/* Задачи */}
      <SectionCard title="Задачи">
        <KanbanWidget />
      </SectionCard>

    </div>
  )
}
