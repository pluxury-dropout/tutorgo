'use client'

import { useMemo } from 'react'
import Link from 'next/link'
import { LayoutDashboard, CalendarDays, Users, BookOpen, Banknote } from 'lucide-react'
import { useStudentCount } from '@/lib/hooks/useStudents'
import { useCourseCount } from '@/lib/hooks/useCourses'
import { useCalendar } from '@/lib/hooks/useCalendar'
import { useRecentPayments, useMonthlyIncome } from '@/lib/hooks/usePayments'
import { FC_COLORS } from '@/lib/lessonStatus'
import { StatusBadge } from '@/components/common/StatusBadge'
import { PageHeader } from '@/components/common/PageHeader'
import type { CalendarLesson } from '@/types/api'

const today = new Date()
const todayFrom = new Date(today.getFullYear(), today.getMonth(), today.getDate()).toISOString()
const todayTo   = new Date(today.getFullYear(), today.getMonth(), today.getDate(), 23, 59, 59).toISOString()

function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
}

function formatAmount(n: number) {
  return '₸ ' + n.toLocaleString('ru-RU')
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
}

interface StatCardProps {
  label:      string
  value:      string | number
  note?:      string
  icon:       React.ReactNode
  iconBg:     string
  highlight?: boolean
}

function StatCard({ label, value, note, icon, iconBg, highlight }: StatCardProps) {
  return (
    <div className="bg-card rounded-[var(--radius-lg)] border border-border p-5 shadow-[var(--shadow-card)]">
      <div className="flex items-start justify-between mb-3">
        <div
          className="h-9 w-9 rounded-[9px] flex items-center justify-center"
          style={{ background: iconBg }}
        >
          {icon}
        </div>
      </div>
      <p className="text-xs font-medium text-muted-foreground mb-1">{label}</p>
      {highlight ? (
        <p className="text-[28px] font-bold leading-none bg-gradient-to-r from-amber-500 to-yellow-400 bg-clip-text text-transparent">
          {value}
        </p>
      ) : (
        <p className="text-[26px] font-bold leading-none text-foreground">{value}</p>
      )}
      {note && <p className="text-xs text-muted-foreground mt-1">{note}</p>}
    </div>
  )
}

interface LessonRowProps {
  lesson: CalendarLesson
}

function LessonRow({ lesson }: LessonRowProps) {
  const dotColor = FC_COLORS[lesson.status].border
  return (
    <div className="flex items-center gap-[14px] px-5 py-[13px] border-b border-border last:border-0 hover:bg-secondary transition-colors">
      <span className="min-w-[46px] text-xs font-semibold text-muted-foreground">
        {formatTime(lesson.scheduled_at)}
      </span>
      <span
        className="h-2 w-2 rounded-full shrink-0"
        style={{ background: dotColor }}
      />
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold truncate">{lesson.subject}</p>
        {lesson.student_name && (
          <p className="text-xs text-muted-foreground truncate">{lesson.student_name}</p>
        )}
      </div>
      <StatusBadge status={lesson.status} />
    </div>
  )
}

export default function DashboardPage() {
  const { data: studentCount = 0 } = useStudentCount()
  const { data: courseCount  = 0 } = useCourseCount()
  const { data: todayLessons = [] } = useCalendar(todayFrom, todayTo)
  const { data: recentPayments = [] } = useRecentPayments()
  const { data: monthlyIncome  = 0 } = useMonthlyIncome()

  const sortedLessons = useMemo(
    () => [...todayLessons].sort((a, b) =>
      new Date(a.scheduled_at).getTime() - new Date(b.scheduled_at).getTime()),
    [todayLessons],
  )

  return (
    <>
      <PageHeader
        title="Главная"
        icon={LayoutDashboard}
        iconBg="var(--primary-light)"
        iconColor="var(--primary)"
      />

      <div className="mt-6 grid grid-cols-2 md:grid-cols-4 gap-[14px]">
        <StatCard
          label="Уроков сегодня"
          value={todayLessons.length}
          icon={<CalendarDays className="h-4 w-4 text-primary" />}
          iconBg="var(--primary-light)"
        />
        <StatCard
          label="Доход за месяц"
          value={formatAmount(monthlyIncome)}
          icon={<Banknote className="h-4 w-4" style={{ color: 'oklch(0.52 0.18 55)' }} />}
          iconBg="var(--accent-light)"
          highlight
        />
        <StatCard
          label="Учеников"
          value={studentCount}
          icon={<Users className="h-4 w-4" style={{ color: 'oklch(0.42 0.14 280)' }} />}
          iconBg="oklch(0.94 0.03 280)"
        />
        <StatCard
          label="Курсов"
          value={courseCount}
          icon={<BookOpen className="h-4 w-4" style={{ color: 'oklch(0.36 0.10 155)' }} />}
          iconBg="oklch(0.92 0.05 155)"
        />
      </div>

      <div className="mt-6 grid grid-cols-1 md:grid-cols-[1fr_380px] gap-6">
        {/* Today's lessons */}
        <div className="bg-card rounded-[var(--radius-lg)] border border-border shadow-[var(--shadow-card)] overflow-hidden">
          <div className="flex items-center justify-between px-5 py-[18px] border-b border-border">
            <h2 className="text-sm font-semibold">Уроки сегодня</h2>
            <Link href="/calendar" className="text-xs text-primary hover:underline">
              Расписание →
            </Link>
          </div>
          {sortedLessons.length === 0 ? (
            <p className="px-5 py-8 text-sm text-muted-foreground text-center">
              Уроков на сегодня нет
            </p>
          ) : (
            sortedLessons.map((l) => <LessonRow key={l.id} lesson={l} />)
          )}
        </div>

        {/* Recent payments */}
        <div className="bg-card rounded-[var(--radius-lg)] border border-border shadow-[var(--shadow-card)] overflow-hidden">
          <div className="flex items-center justify-between px-5 py-[18px] border-b border-border">
            <h2 className="text-sm font-semibold">Последние платежи</h2>
            <Link href="/payments" className="text-xs text-primary hover:underline">
              Все →
            </Link>
          </div>
          {recentPayments.length === 0 ? (
            <p className="px-5 py-8 text-sm text-muted-foreground text-center">
              Платежей пока нет
            </p>
          ) : (
            recentPayments.map((p) => (
              <div key={p.id} className="flex items-center gap-3 px-5 py-3 border-b border-border last:border-0">
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-semibold truncate">{formatAmount(p.amount)}</p>
                  <p className="text-xs text-muted-foreground">{p.lessons_count} урок(ов)</p>
                </div>
                <span className="text-[11px] text-muted-foreground shrink-0">{formatDate(p.paid_at)}</span>
              </div>
            ))
          )}
        </div>
      </div>
    </>
  )
}
