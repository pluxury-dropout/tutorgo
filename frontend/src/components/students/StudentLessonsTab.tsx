'use client'

import { useState } from 'react'
import { useLessonsByPeriod } from '@/lib/hooks/useLessons'
import { PeriodPicker } from '@/components/lessons/PeriodPicker'
import { STATUS_LABELS, STATUS_COLORS } from '@/lib/lessonStatus'
import { CycleBadge } from '@/components/lessons/CycleBadge'
import type { StudentCourseSummary } from '@/types/api'

function currentWeekRange(): { from: Date; to: Date } {
  const now = new Date()
  const day = now.getDay()
  const diff = day === 0 ? -6 : 1 - day
  const from = new Date(now)
  from.setDate(now.getDate() + diff)
  from.setHours(0, 0, 0, 0)
  const to = new Date(from)
  to.setDate(from.getDate() + 7)
  return { from, to }
}

/** Уроки одного курса ученика — период листается независимо для каждого
 *  предмета: у математики и физики разное расписание (спека, п. 7.2). */
function CourseLessons({ course }: { course: StudentCourseSummary }) {
  const [period, setPeriod] = useState(currentWeekRange)
  const { data: lessons = [] } = useLessonsByPeriod(course.course_id, period.from.toISOString(), period.to.toISOString())

  return (
    <div className="border rounded-xl bg-card p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-semibold">{course.subject}</h3>
        <PeriodPicker from={period.from} to={period.to} onChange={(from, to) => setPeriod({ from, to })} />
      </div>
      {lessons.length === 0 ? (
        <p className="text-sm text-muted-foreground">Уроков в этом периоде нет</p>
      ) : (
        <div className="space-y-1">
          {lessons.map((l) => (
            <div key={l.id} className="flex items-center justify-between py-1.5 border-b last:border-0 text-sm">
              <span className="text-muted-foreground">
                {new Date(l.scheduled_at).toLocaleString('ru-RU', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })}
              </span>
              <div className="flex items-center gap-2">
                {l.cycle_position != null && l.cycle_size != null && <CycleBadge position={l.cycle_position} size={l.cycle_size} />}
                <span className={`text-xs px-2 py-0.5 rounded-full ${STATUS_COLORS[l.status] ?? ''}`}>{STATUS_LABELS[l.status] ?? l.status}</span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export function StudentLessonsTab({ courses }: { courses: StudentCourseSummary[] }) {
  if (courses.length === 0) return <p className="text-sm text-muted-foreground">Нет курсов</p>
  return <div className="space-y-4">{courses.map((c) => <CourseLessons key={c.course_id} course={c} />)}</div>
}
