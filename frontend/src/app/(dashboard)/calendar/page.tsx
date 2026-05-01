'use client'

import { useState, useEffect, useCallback, useRef } from 'react'
import FullCalendar from '@fullcalendar/react'
import dayGridPlugin from '@fullcalendar/daygrid'
import timeGridPlugin from '@fullcalendar/timegrid'
import interactionPlugin from '@fullcalendar/interaction'
import type { DatesSetArg, EventClickArg, EventDropArg, DateSelectArg } from '@fullcalendar/core'
import type { EventResizeDoneArg } from '@fullcalendar/interaction'
import ruLocale from '@fullcalendar/core/locales/ru'
import { Circle, CheckCircle2 } from 'lucide-react'

import { useCalendar, useRescheduleLesson } from '@/lib/hooks/useCalendar'
import { useTasks, useToggleTask, useRescheduleTask } from '@/lib/hooks/useTasks'
import { FC_COLORS } from '@/lib/lessonStatus'
import { LessonQuickDialog } from '@/components/lessons/LessonQuickDialog'
import { TaskCreateDialog } from '@/components/tasks/TaskCreateDialog'
import type { LessonStatus } from '@/types/api'
import type { QuickLesson } from '@/components/lessons/LessonQuickDialog'

const TASK_COLORS = {
  active: { bg: 'oklch(0.93 0.05 295)', border: 'oklch(0.58 0.16 295)', text: 'oklch(0.32 0.16 295)' },
  done:   { bg: 'oklch(0.96 0.015 295)', border: 'oklch(0.75 0.06 295)', text: 'oklch(0.55 0.06 295)' },
}

function roundToNearest30(date: Date): Date {
  const ms = 30 * 60 * 1000
  return new Date(Math.round(date.getTime() / ms) * ms)
}

function roundToNearest15(n: number): number {
  return Math.max(15, Math.round(n / 15) * 15)
}

const EDGE_ZONE = 50

export default function CalendarPage() {
  const { mutate: reschedule } = useRescheduleLesson()
  const toggleTask             = useToggleTask()
  const rescheduleTask         = useRescheduleTask()

  const [selectedLesson, setSelectedLesson] = useState<QuickLesson | null>(null)
  const [newTaskSlot, setNewTaskSlot]       = useState<{ start: Date; end: Date } | null>(null)
  const [isTouch, setIsTouch]               = useState(false)

  const calendarRef       = useRef<FullCalendar>(null)
  const edgeTimerRef      = useRef<ReturnType<typeof setTimeout> | null>(null)
  const dragSideRef       = useRef<'left' | 'right' | null>(null)
  const calendarRectRef   = useRef<DOMRect | null>(null)
  const pointerHandlerRef = useRef<((e: PointerEvent) => void) | null>(null)

  useEffect(() => {
    setIsTouch(window.matchMedia('(pointer: coarse)').matches)
  }, [])

  const now = new Date()
  const [range, setRange] = useState({
    from: new Date(now.getFullYear(), now.getMonth(), 1).toISOString(),
    to:   new Date(now.getFullYear(), now.getMonth() + 1, 0, 23, 59, 59).toISOString(),
  })

  const { data: lessons = [] } = useCalendar(range.from, range.to)
  const { data: tasks   = [] } = useTasks(range.from, range.to)

  const lessonEvents = lessons.map((l) => ({
    id:              l.id,
    title:           l.is_group ? l.subject : `${l.subject}${l.student_name ? ` — ${l.student_name}` : ''}`,
    start:           l.scheduled_at,
    end:             new Date(new Date(l.scheduled_at).getTime() + l.duration_minutes * 60_000).toISOString(),
    backgroundColor: FC_COLORS[l.status].bg,
    borderColor:     FC_COLORS[l.status].border,
    textColor:       FC_COLORS[l.status].text,
    extendedProps: {
      type:            'lesson',
      courseId:        l.course_id,
      status:          l.status,
      notes:           l.notes,
      isGroup:         l.is_group,
      scheduledAt:     l.scheduled_at,
      durationMinutes: l.duration_minutes,
    },
  }))

  const taskEvents = tasks.map((t) => {
    const colors = t.done ? TASK_COLORS.done : TASK_COLORS.active
    return {
      id:              t.id,
      title:           t.title,
      start:           t.scheduled_at,
      end:             new Date(new Date(t.scheduled_at).getTime() + t.duration_minutes * 60_000).toISOString(),
      backgroundColor: colors.bg,
      borderColor:     colors.border,
      textColor:       colors.text,
      extendedProps: {
        type:  'task',
        done:  t.done,
        title: t.title,
      },
    }
  })

  const events = [...lessonEvents, ...taskEvents]

  function refreshCalendarRect() {
    const el = document.querySelector('.fc-view-harness')
    if (el) calendarRectRef.current = el.getBoundingClientRect()
  }

  function clearEdgeTimer() {
    if (edgeTimerRef.current) {
      clearTimeout(edgeTimerRef.current)
      edgeTimerRef.current = null
    }
    dragSideRef.current = null
  }

  function armEdgeTimer(side: 'left' | 'right') {
    dragSideRef.current = side
    edgeTimerRef.current = setTimeout(() => {
      const api = calendarRef.current?.getApi()
      if (api) {
        side === 'left' ? api.prev() : api.next()
        dragSideRef.current = null
        requestAnimationFrame(() => refreshCalendarRect())
      }
    }, 500)
  }

  function handleEventDragStart() {
    refreshCalendarRect()

    const handler = (e: PointerEvent) => {
      const rect = calendarRectRef.current
      if (!rect) return

      const inLeft  = e.clientX < rect.left + EDGE_ZONE
      const inRight = e.clientX > rect.right - EDGE_ZONE

      if (inLeft && dragSideRef.current !== 'left') {
        clearEdgeTimer()
        armEdgeTimer('left')
      } else if (inRight && dragSideRef.current !== 'right') {
        clearEdgeTimer()
        armEdgeTimer('right')
      } else if (!inLeft && !inRight && dragSideRef.current !== null) {
        clearEdgeTimer()
      }
    }

    pointerHandlerRef.current = handler
    document.addEventListener('pointermove', handler)
  }

  function handleEventDragStop() {
    clearEdgeTimer()
    if (pointerHandlerRef.current) {
      document.removeEventListener('pointermove', pointerHandlerRef.current)
      pointerHandlerRef.current = null
    }
  }

  const handleDatesSet = useCallback((arg: DatesSetArg) => {
    setRange({ from: arg.start.toISOString(), to: arg.end.toISOString() })
  }, [])

  function handleEventClick(arg: EventClickArg) {
    if (arg.event.extendedProps.type === 'task') return
    const p = arg.event.extendedProps
    setSelectedLesson({
      id:              arg.event.id,
      courseId:        p.courseId,
      title:           arg.event.title,
      status:          p.status as LessonStatus,
      notes:           p.notes ?? '',
      isGroup:         p.isGroup,
      scheduledAt:     p.scheduledAt,
      durationMinutes: p.durationMinutes,
    })
  }

  function handleSelect(arg: DateSelectArg) {
    setNewTaskSlot({ start: arg.start, end: arg.end })
    calendarRef.current?.getApi().unselect()
  }

  function handleEventDrop(arg: EventDropArg) {
    const start    = arg.event.start!
    const end      = arg.event.end!
    const snapped  = roundToNearest30(start)
    const duration = Math.round((end.getTime() - start.getTime()) / 60_000)

    if (arg.event.extendedProps.type === 'task') {
      rescheduleTask.mutate(
        {
          id:   arg.event.id,
          data: {
            title:           arg.event.extendedProps.title as string,
            scheduled_at:    snapped.toISOString(),
            duration_minutes: duration,
            done:            arg.event.extendedProps.done as boolean,
          },
        },
        { onError: () => arg.revert() },
      )
      return
    }

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
      <LessonQuickDialog lesson={selectedLesson} onClose={() => setSelectedLesson(null)} />
      <TaskCreateDialog
        start={newTaskSlot?.start ?? null}
        end={newTaskSlot?.end ?? null}
        onClose={() => setNewTaskSlot(null)}
      />
      <div className="rounded-lg border p-4">
        <FullCalendar
          ref={calendarRef}
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
          selectable={true}
          unselectAuto={false}
          select={handleSelect}
          eventDurationEditable={!isTouch}
          eventDrop={handleEventDrop}
          eventResize={handleEventResize}
          eventDragStart={handleEventDragStart}
          eventDragStop={handleEventDragStop}
          eventDidMount={(arg) => {
            if (arg.event.end && arg.event.end < new Date()) {
              arg.el.style.opacity = '0.45'
            }
            if (arg.event.extendedProps.type === 'task') {
              arg.el.style.height    = 'auto'
              arg.el.style.minHeight = '24px'
              arg.el.style.overflow  = 'visible'
              arg.el.style.zIndex    = '4'
            }
          }}
          eventContent={(arg) => {
            if (arg.event.extendedProps.type === 'task') {
              const done = arg.event.extendedProps.done as boolean
              return (
                <div className="flex items-start gap-1 px-1.5 py-0.5 w-full">
                  <button
                    onClick={(e) => {
                      e.stopPropagation()
                      toggleTask.mutate(arg.event.id)
                    }}
                    className="shrink-0 mt-px"
                  >
                    {done
                      ? <CheckCircle2 size={12} className="text-current" />
                      : <Circle      size={12} className="text-current" />}
                  </button>
                  <span
                    className="text-[11px] font-semibold leading-tight break-words min-w-0"
                    style={done ? { textDecoration: 'line-through', opacity: 0.65 } : undefined}
                  >
                    {arg.event.title}
                  </span>
                </div>
              )
            }
            return (
              <>
                <div className="fc-event-time">{arg.timeText}</div>
                <div className="fc-event-title">{arg.event.title}</div>
              </>
            )
          }}
          snapDuration="00:15:00"
          height="90vh"
          eventLongPressDelay={300}
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
