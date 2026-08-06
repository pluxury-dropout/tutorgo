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

import { stripHtml } from '@/lib/stripHtml'
import { useCalendar, useRescheduleLesson } from '@/lib/hooks/useCalendar'
import { useTasks, useRescheduleTask } from '@/lib/hooks/useTasks'
import { FC_COLORS, effectiveStatus } from '@/lib/lessonStatus'
import { useMinuteTick } from '@/lib/hooks/useMinuteTick'
import { CycleBadge } from '@/components/lessons/CycleBadge'
import { LessonQuickDialog } from '@/components/lessons/LessonQuickDialog'
import { TaskCreateDialog } from '@/components/tasks/TaskCreateDialog'
import { MobileWeekCalendar } from '@/components/calendar/MobileWeekCalendar'
import type { LessonStatus } from '@/types/api'
import type { QuickLesson } from '@/components/lessons/LessonQuickDialog'

const TASK_COLORS = {
  active: { bg: 'oklch(0.86 0.06 305)', border: 'oklch(0.41 0.22 305)', text: 'oklch(0.26 0.18 305)' },
  done:   { bg: 'oklch(0.92 0.03 305)', border: 'oklch(0.65 0.08 305)', text: 'oklch(0.50 0.10 305)' },
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
  useMinuteTick()
  const { mutate: reschedule } = useRescheduleLesson()
  const rescheduleTask         = useRescheduleTask()

  const [selectedLesson, setSelectedLesson] = useState<QuickLesson | null>(null)
  const [newTaskSlot, setNewTaskSlot]       = useState<{ start: Date; end: Date } | null>(null)
  const [isMobile, setIsMobile]             = useState(false)

  const calendarRef       = useRef<FullCalendar>(null)
  const edgeTimerRef      = useRef<ReturnType<typeof setTimeout> | null>(null)
  const dragSideRef       = useRef<'left' | 'right' | null>(null)
  const calendarRectRef   = useRef<DOMRect | null>(null)
  const pointerHandlerRef = useRef<((e: PointerEvent) => void) | null>(null)

  // Мобильный календарь — по ширине экрана (md=768px), а не по типу указателя:
  // ноутбуки с тачскрином дают coarse-pointer, но им нужен десктопный календарь.
  useEffect(() => {
    const mq = window.matchMedia('(max-width: 767px)')
    const update = () => setIsMobile(mq.matches)
    update()
    mq.addEventListener('change', update)
    return () => mq.removeEventListener('change', update)
  }, [])

  // Listen for navigation requests from the sidebar mini-calendar
  useEffect(() => {
    const handler = (e: Event) => {
      const iso = (e as CustomEvent<string>).detail
      calendarRef.current?.getApi().gotoDate(new Date(iso))
    }
    window.addEventListener('fc:goto', handler)
    return () => window.removeEventListener('fc:goto', handler)
  }, [])

  const [range, setRange] = useState(() => {
    const n = new Date()
    // Match FullCalendar's timeGridWeek initial range (firstDay=1 → Mon–Sun)
    const daysFromMonday = n.getDay() === 0 ? 6 : n.getDay() - 1
    const start = new Date(n)
    start.setDate(n.getDate() - daysFromMonday)
    start.setHours(0, 0, 0, 0)
    const end = new Date(start)
    end.setDate(start.getDate() + 7)
    return { from: start.toISOString(), to: end.toISOString() }
  })

  const { data: lessons = [], isPending: lessonsPending } = useCalendar(range.from, range.to)
  const { data: tasks   = [], isPending: tasksPending   } = useTasks(range.from, range.to)
  const eventsLoading = lessonsPending || tasksPending

  const lessonEvents = lessons.map((l) => {
    const status = effectiveStatus(l)
    return {
    id:              l.id,
    title:           l.is_group ? l.subject : `${l.subject}${l.student_name ? ` — ${l.student_name}` : ''}`,
    start:           l.scheduled_at,
    end:             new Date(new Date(l.scheduled_at).getTime() + l.duration_minutes * 60_000).toISOString(),
    backgroundColor: FC_COLORS[status].bg,
    borderColor:     FC_COLORS[status].border,
    textColor:       FC_COLORS[status].text,
    extendedProps: {
      type:            'lesson',
      courseId:        l.course_id,
      status:          status,
      notes:           l.notes,
      isGroup:         l.is_group,
      scheduledAt:     l.scheduled_at,
      durationMinutes: l.duration_minutes,
      cyclePosition:   l.cycle_position ?? null,
      cycleSize:       l.cycle_size ?? null,
    },
  }})

  const taskEvents = tasks.filter((t) => t.scheduled_at).map((t) => {
    const colors = t.status === 'done' ? TASK_COLORS.done : TASK_COLORS.active
    return {
      id:              t.id,
      title:           stripHtml(t.title),
      start:           t.scheduled_at!,
      end:             new Date(new Date(t.scheduled_at!).getTime() + t.duration_minutes! * 60_000).toISOString(),
      backgroundColor: colors.bg,
      borderColor:     colors.border,
      textColor:       colors.text,
      extendedProps: {
        type:   'task',
        status: t.status,
        title:  t.title,
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
        clearEdgeTimer(); armEdgeTimer('left')
      } else if (inRight && dragSideRef.current !== 'right') {
        clearEdgeTimer(); armEdgeTimer('right')
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
    // Notify sidebar mini-calendar about the displayed date range
    window.dispatchEvent(new CustomEvent('fc:datesSet', {
      detail: { start: arg.start.toISOString(), end: arg.end.toISOString() },
    }))
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
            title:            arg.event.extendedProps.title as string,
            scheduled_at:     snapped.toISOString(),
            duration_minutes: duration,
            status:           arg.event.extendedProps.status as string,
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
      {/* Mobile: full-screen compact week */}
      {isMobile && (
        <div className="h-full">
          <MobileWeekCalendar />
        </div>
      )}

      {/* Desktop: FullCalendar */}
      <LessonQuickDialog lesson={selectedLesson} onClose={() => setSelectedLesson(null)} />
      <TaskCreateDialog
        start={newTaskSlot?.start ?? null}
        end={newTaskSlot?.end ?? null}
        onClose={() => setNewTaskSlot(null)}
      />
      <div className={`${isMobile ? 'hidden' : 'flex flex-col'} h-full min-h-0 overflow-hidden`}>
        {/* Подсказка вместо пустой сетки: новичок не догадывается, что урок
            ставится кликом по слоту. Исчезает, как только в периоде есть события,
            и не показывается, пока запросы периода ещё в полёте. */}
        {events.length === 0 && !eventsLoading && (
          <div className="mb-2 shrink-0 rounded-lg border border-dashed px-3 py-2 text-center text-xs text-muted-foreground">
            Уроков в этом периоде нет. Кликни по свободному слоту, чтобы поставить урок или задачу —
            урок привязывается к курсу, так что сначала заведи курс с учеником.
          </div>
        )}
        <div className="min-h-0 flex-1">
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
          eventDurationEditable={!isMobile}
          eventDrop={handleEventDrop}
          eventResize={handleEventResize}
          eventDragStart={handleEventDragStart}
          eventDragStop={handleEventDragStop}
          eventClassNames={(arg) =>
            arg.event.end && arg.event.end < new Date() ? ['fc-event-past'] : []
          }
          eventDidMount={(arg) => {
            if (arg.event.extendedProps.type === 'task') {
              arg.el.style.height    = 'auto'
              arg.el.style.minHeight = '24px'
              arg.el.style.overflow  = 'visible'
              arg.el.style.zIndex    = '4'
            }
          }}
          eventContent={(arg) => {
            if (arg.event.extendedProps.type === 'task') {
              const done = arg.event.extendedProps.status === 'done'
              return (
                <div className="flex items-start gap-1 px-1.5 py-0.5 w-full">
                  <button
                    onClick={(e) => {
                      e.stopPropagation()
                      const task = tasks?.find(t => t.id === arg.event.id)
                      if (task) {
                        rescheduleTask.mutate({
                          id: task.id,
                          data: {
                            title: task.title,
                            scheduled_at: task.scheduled_at,
                            duration_minutes: task.duration_minutes,
                            status: task.status === 'done' ? 'not_urgent' : 'done',
                          }
                        })
                      }
                    }}
                    className="shrink-0 mt-px"
                  >
                    {done
                      ? <CheckCircle2 size={12} className="text-current" />
                      : <Circle       size={12} className="text-current" />}
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

            const cyclePosition = arg.event.extendedProps.cyclePosition as number | null
            const cycleSize     = arg.event.extendedProps.cycleSize as number | null
            const cancelled     = arg.event.extendedProps.status === 'cancelled'
            return (
              <div className="relative h-full w-full overflow-hidden">
                <div className="fc-event-time">{arg.timeText}</div>
                <div
                  className="fc-event-title"
                  style={cancelled ? { textDecoration: 'line-through' } : undefined}
                >
                  {arg.event.title}
                </div>
                {cyclePosition != null && cycleSize != null && (
                  <div className="absolute bottom-0.5 right-0.5">
                    <CycleBadge position={cyclePosition} size={cycleSize} />
                  </div>
                )}
              </div>
            )
          }}
          snapDuration="00:15:00"
          height="100%"
          expandRows={false}
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
      </div>
    </>
  )
}
