'use client'

import { useState, useCallback } from 'react'
import FullCalendar from '@fullcalendar/react'
import dayGridPlugin from '@fullcalendar/daygrid'
import timeGridPlugin from '@fullcalendar/timegrid'
import interactionPlugin from '@fullcalendar/interaction'
import type { DatesSetArg, EventClickArg, EventDropArg } from '@fullcalendar/core'
import type { EventResizeDoneArg } from '@fullcalendar/interaction'
import ruLocale from '@fullcalendar/core/locales/ru'
import { useRouter } from 'next/navigation'

import { useCalendar, useRescheduleLesson } from '@/lib/hooks/useCalendar'
import { FC_COLORS } from '@/lib/lessonStatus'
import { PageHeader } from '@/components/common/PageHeader'
import type { LessonStatus } from '@/types/api'

function roundToNearest30(date: Date): Date {
  const ms = 30 * 60 * 1000
  return new Date(Math.round(date.getTime() / ms) * ms)
}

function roundToNearest15(n: number): number {
  return Math.max(15, Math.round(n / 15) * 15)
}

export default function CalendarPage() {
  const router = useRouter()
  const { mutate: reschedule } = useRescheduleLesson()

  const now = new Date()
  const [range, setRange] = useState({
    from: new Date(now.getFullYear(), now.getMonth(), 1).toISOString(),
    to:   new Date(now.getFullYear(), now.getMonth() + 1, 0, 23, 59, 59).toISOString(),
  })

  const { data: lessons = [] } = useCalendar(range.from, range.to)

  const events = lessons.map((l) => ({
    id:              l.id,
    title:           l.is_group ? l.subject : `${l.subject}${l.student_name ? ` — ${l.student_name}` : ''}`,
    start:           l.scheduled_at,
    end:             new Date(new Date(l.scheduled_at).getTime() + l.duration_minutes * 60_000).toISOString(),
    backgroundColor: FC_COLORS[l.status].bg,
    borderColor:     FC_COLORS[l.status].border,
    textColor:       FC_COLORS[l.status].text,
    extendedProps:   { courseId: l.course_id, status: l.status, notes: l.notes },
  }))

  const handleDatesSet = useCallback((arg: DatesSetArg) => {
    setRange({ from: arg.start.toISOString(), to: arg.end.toISOString() })
  }, [])

  function handleEventClick(arg: EventClickArg) {
    router.push(`/courses/${arg.event.extendedProps.courseId}`)
  }

  function handleEventDrop(arg: EventDropArg) {
    const start    = arg.event.start!
    const end      = arg.event.end!
    const snapped  = roundToNearest30(start)
    const duration = Math.round((end.getTime() - start.getTime()) / 60_000)

    reschedule(
      {
        id:   arg.event.id,
        data: {
          scheduled_at:     snapped.toISOString(),
          duration_minutes: duration,
          status:           arg.event.extendedProps.status as LessonStatus,
          notes:            arg.event.extendedProps.notes ?? '',
        },
      },
      { onError: () => arg.revert() },
    )
  }

  function handleEventResize(arg: EventResizeDoneArg) {
    const start    = arg.event.start!
    const end      = arg.event.end!
    const rawDur   = Math.round((end.getTime() - start.getTime()) / 60_000)
    const duration = roundToNearest15(rawDur)

    reschedule(
      {
        id:   arg.event.id,
        data: {
          scheduled_at:     start.toISOString(),
          duration_minutes: duration,
          status:           arg.event.extendedProps.status as LessonStatus,
          notes:            arg.event.extendedProps.notes ?? '',
        },
      },
      { onError: () => arg.revert() },
    )
  }

  return (
    <>
      <PageHeader title="Расписание" />
      <div className="mt-4 rounded-lg border p-4">
        <FullCalendar
          plugins={[dayGridPlugin, timeGridPlugin, interactionPlugin]}
          initialView="timeGridWeek"
          headerToolbar={{
            left:   'prev,next today',
            center: 'title',
            right:  'dayGridMonth,timeGridWeek,timeGridDay',
          }}
          locale={ruLocale}
          firstDay={1}
          events={events}
          datesSet={handleDatesSet}
          eventClick={handleEventClick}
          editable={true}
          eventDrop={handleEventDrop}
          eventResize={handleEventResize}
          snapDuration="00:15:00"
          height="70vh"
          allDaySlot={false}
          slotDuration="00:30:00"
          slotLabelInterval="01:00:00"
          scrollTime="08:00:00"
          nowIndicator={true}
          slotMinTime="07:00:00"
          slotMaxTime="23:00:00"
          buttonText={{ today: 'Сегодня', month: 'Месяц', week: 'Неделя', day: 'День' }}
        />
      </div>
    </>
  )
}
