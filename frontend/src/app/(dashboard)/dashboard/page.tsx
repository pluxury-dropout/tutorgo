'use client'

import { useState, useMemo } from 'react'
import Link from 'next/link'
import { useStudentCount } from '@/lib/hooks/useStudents'
import { useCourseCount } from '@/lib/hooks/useCourses'
import { useCalendar } from '@/lib/hooks/useCalendar'
import { useRecentPayments, useMonthlyIncome } from '@/lib/hooks/usePayments'
import { FC_COLORS } from '@/lib/lessonStatus'
import { StatusBadge } from '@/components/common/StatusBadge'
import { HeaderPanel } from '@/components/HeaderPanel'
import type { KpiSegment } from '@/components/HeaderPanel'
import type { CalendarLesson } from '@/types/api'

const today = new Date()
const todayFrom = new Date(today.getFullYear(), today.getMonth(), today.getDate()).toISOString()
const todayTo   = new Date(today.getFullYear(), today.getMonth(), today.getDate(), 23, 59, 59).toISOString()

const SUBTITLE = today.toLocaleDateString('ru-RU', {
  weekday: 'long',
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})

function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
}

function formatAmount(n: number) {
  return '₸ ' + n.toLocaleString('ru-RU')
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
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
  const [activeSegment, setActiveSegment] = useState('lessons')

  const { data: studentCount = 0, isLoading: studentsLoading } = useStudentCount()
  const { data: courseCount  = 0, isLoading: coursesLoading  } = useCourseCount()
  const { data: todayLessons = [], isLoading: lessonsLoading  } = useCalendar(todayFrom, todayTo)
  const { data: recentPayments = [] }                           = useRecentPayments()
  const { data: monthlyIncome  = 0, isLoading: incomeLoading  } = useMonthlyIncome()

  const sortedLessons = useMemo(
    () => [...todayLessons].sort((a, b) =>
      new Date(a.scheduled_at).getTime() - new Date(b.scheduled_at).getTime()),
    [todayLessons],
  )

  const segments: KpiSegment[] = [
    {
      id:       'lessons',
      label:    'Уроки',
      value:    String(todayLessons.length),
      dotColor: 'var(--primary)',
      meta:     'сегодня',
      loading:  lessonsLoading,
    },
    {
      id:       'revenue',
      label:    'Доход',
      value:    formatAmount(monthlyIncome),
      dotColor: 'var(--warning)',
      meta:     'этот месяц',
      loading:  incomeLoading,
    },
    {
      id:       'students',
      label:    'Ученики',
      value:    String(studentCount),
      dotColor: 'var(--purple)',
      meta:     'всего',
      loading:  studentsLoading,
    },
    {
      id:       'courses',
      label:    'Курсы',
      value:    String(courseCount),
      dotColor: 'var(--success)',
      meta:     'активных',
      loading:  coursesLoading,
    },
  ]

  return (
    <>
      <HeaderPanel
        title="Главная"
        subtitle={SUBTITLE}
        segments={segments}
        activeSegment={activeSegment}
        onSegmentChange={setActiveSegment}
      />

      <div className="mt-6 grid grid-cols-1 md:grid-cols-[1fr_380px] gap-6">
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
