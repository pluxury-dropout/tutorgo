'use client'

import { useState, useEffect, useCallback, useRef } from 'react'
import FullCalendar from '@fullcalendar/react'
import dayGridPlugin from '@fullcalendar/daygrid'
import timeGridPlugin from '@fullcalendar/timegrid'
import interactionPlugin from '@fullcalendar/interaction'
import type { DatesSetArg, EventClickArg, EventDropArg, DateSelectArg, EventInput } from '@fullcalendar/core'
import type { EventResizeDoneArg } from '@fullcalendar/interaction'
import ruLocale from '@fullcalendar/core/locales/ru'
import { Circle, CheckCircle2 } from 'lucide-react'

import { stripHtml } from '@/lib/stripHtml'
import { useCalendarFeed, useRescheduleLesson } from '@/lib/hooks/useCalendar'
import { useRescheduleTask } from '@/lib/hooks/useTasks'
import { useUpdateEvent } from '@/lib/hooks/useEvents'
import { warnOnConflict } from '@/lib/conflictWarning'
import { FC_COLORS, effectiveStatus } from '@/lib/lessonStatus'
import { KIND_COLORS, TASK_COLORS } from '@/lib/eventKind'
import { useMinuteTick } from '@/lib/hooks/useMinuteTick'
import { CycleBadge } from '@/components/lessons/CycleBadge'
import { LessonQuickPopover } from '@/components/lessons/LessonQuickPopover'
import { SlotCreatePopover } from '@/components/calendar/SlotCreatePopover'
import { EventQuickPopover } from '@/components/calendar/EventQuickPopover'
import { MobileWeekCalendar } from '@/components/calendar/MobileWeekCalendar'
import { StudentOnboardingDialog } from '@/components/students/StudentOnboardingDialog'
import { Button } from '@/components/ui/button'
import type { Event as TutorEvent, LessonStatus, Task } from '@/types/api'
import type { QuickLesson } from '@/components/lessons/LessonQuickPopover'

function roundToNearest30(date: Date): Date {
  const ms = 30 * 60 * 1000
  return new Date(Math.round(date.getTime() / ms) * ms)
}

function roundToNearest15(n: number): number {
  return Math.max(15, Math.round(n / 15) * 15)
}

const EDGE_ZONE = 50

const PERSONAL_KEY = 'tg_cal_show_personal'

export default function CalendarPage() {
  useMinuteTick()
  const { mutate: reschedule } = useRescheduleLesson()
  const rescheduleTask         = useRescheduleTask()
  const updateEvent            = useUpdateEvent()

  // Вместе с уроком/событием/слотом храним якорь, у которого открыть поповер:
  // блок в сетке и прямоугольник выделенного слота соответственно.
  const [selectedLesson, setSelectedLesson] = useState<{ lesson: QuickLesson; el: HTMLElement } | null>(null)
  const [selectedEvent, setSelectedEvent]   = useState<{ event: TutorEvent; el: HTMLElement } | null>(null)
  const [newTaskSlot, setNewTaskSlot]       = useState<{ start: Date; end: Date; rect: DOMRect } | null>(null)
  const [isMobile, setIsMobile]             = useState(false)
  const [onboardOpen, setOnboardOpen]       = useState(false)
  // Тумблер «показывать личное»: нужен, когда репетитор показывает расписание
  // ученику. Читаем localStorage лениво с window-guard — как в stores/auth.
  const [showPersonal, setShowPersonal] = useState(
    () => typeof window === 'undefined' || localStorage.getItem(PERSONAL_KEY) !== '0',
  )

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

  useEffect(() => {
    localStorage.setItem(PERSONAL_KEY, showPersonal ? '1' : '0')
  }, [showPersonal])

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

  const { data: items = [], isPending: eventsLoading } = useCalendarFeed(range.from, range.to)

  // Одна лента — один проход. Общие поля дают сетку, вложенный объект по типу —
  // цвета и то, что читают поповеры.
  const events = items.flatMap((item): EventInput[] => {
    const end = new Date(new Date(item.starts_at).getTime() + item.duration_minutes * 60_000).toISOString()

    if (item.type === 'lesson') {
      const l = item.lesson
      const status = effectiveStatus(l)
      return [{
        id:              item.id,
        title:           item.title,
        start:           item.starts_at,
        end,
        backgroundColor: FC_COLORS[status].bg,
        borderColor:     FC_COLORS[status].border,
        textColor:       FC_COLORS[status].text,
        extendedProps: {
          type:            'lesson',
          courseId:        l.course_id,
          status:          status,
          notes:           l.notes,
          isGroup:         l.is_group,
          scheduledAt:     item.starts_at,
          durationMinutes: item.duration_minutes,
          cyclePosition:   l.cycle_position ?? null,
          cycleSize:       l.cycle_size ?? null,
        },
      }]
    }

    if (item.type === 'event') {
      const e = item.event
      if (e.kind === 'personal' && !showPersonal) return []
      const colors = KIND_COLORS[e.kind]
      return [{
        id:              item.id,
        title:           item.title,
        start:           item.starts_at,
        end,
        backgroundColor: e.color || colors.bg,
        borderColor:     e.color || colors.border,
        textColor:       colors.text,
        extendedProps: { type: 'event', event: e },
      }]
    }

    const t = item.task
    const colors = t.status === 'done' ? TASK_COLORS.done : TASK_COLORS.active
    return [{
      id:              item.id,
      title:           stripHtml(item.title),
      start:           item.starts_at,
      end,
      backgroundColor: colors.bg,
      borderColor:     colors.border,
      textColor:       colors.text,
      extendedProps: { type: 'task', status: t.status, title: t.title, task: t },
    }]
  })

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
    if (arg.event.extendedProps.type === 'event') {
      setSelectedEvent({ el: arg.el, event: arg.event.extendedProps.event as TutorEvent })
      return
    }
    const p = arg.event.extendedProps
    setSelectedLesson({
      el: arg.el,
      lesson: {
        id:              arg.event.id,
        courseId:        p.courseId,
        title:           arg.event.title,
        status:          p.status as LessonStatus,
        notes:           p.notes ?? '',
        isGroup:         p.isGroup,
        scheduledAt:     p.scheduledAt,
        durationMinutes: p.durationMinutes,
      },
    })
  }

  function handleSelect(arg: DateSelectArg) {
    // Прямоугольник снимаем до unselect() — подсветка слота тут же исчезнет.
    const rect = document.querySelector('.fc-highlight')?.getBoundingClientRect()
      ?? new DOMRect(arg.jsEvent?.clientX ?? 0, arg.jsEvent?.clientY ?? 0, 0, 0)
    setNewTaskSlot({ start: arg.start, end: arg.end, rect })
    calendarRef.current?.getApi().unselect()
  }

  function handleEventDrop(arg: EventDropArg) {
    const start    = arg.event.start!
    const end      = arg.event.end ?? new Date(start.getTime() + 60 * 60_000)
    const snapped  = roundToNearest30(start)
    const duration = Math.round((end.getTime() - start.getTime()) / 60_000)

    if (arg.event.extendedProps.type === 'event') {
      const e = arg.event.extendedProps.event as TutorEvent
      updateEvent.mutate(
        {
          id:   e.id,
          data: {
            title:            e.title,
            kind:             e.kind,
            starts_at:        snapped.toISOString(),
            duration_minutes: duration,
            color:            e.color,
            location:         e.location,
            notes:            e.notes,
          },
        },
        {
          onError:   () => arg.revert(),
          onSuccess: () => warnOnConflict({
            starts_at:        snapped.toISOString(),
            duration_minutes: duration,
            exclude_type:     'event',
            exclude_id:       e.id,
          }),
        },
      )
      return
    }

    // Задачи занятостью не считаются — для них проверки нет.
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
      {
        onError:   () => arg.revert(),
        onSuccess: () => warnOnConflict({
          starts_at:        snapped.toISOString(),
          duration_minutes: duration,
          exclude_type:     'lesson',
          exclude_id:       arg.event.id,
        }),
      },
    )
  }

  function handleEventResize(arg: EventResizeDoneArg) {
    const start    = arg.event.start!
    const end      = arg.event.end!
    const rawDur   = Math.round((end.getTime() - start.getTime()) / 60_000)
    const duration = roundToNearest15(rawDur)

    if (arg.event.extendedProps.type === 'event') {
      const e = arg.event.extendedProps.event as TutorEvent
      updateEvent.mutate(
        { id: e.id, data: { ...e, duration_minutes: duration } },
        { onError: () => arg.revert() },
      )
      return
    }

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
          <MobileWeekCalendar
            showPersonal={showPersonal}
            onTogglePersonal={() => setShowPersonal(v => !v)}
          />
        </div>
      )}

      {/* Desktop: FullCalendar */}
      <LessonQuickPopover
        lesson={selectedLesson?.lesson ?? null}
        anchor={selectedLesson?.el ?? null}
        onClose={() => setSelectedLesson(null)}
      />
      <SlotCreatePopover
        start={newTaskSlot?.start ?? null}
        end={newTaskSlot?.end ?? null}
        anchor={newTaskSlot?.rect ?? null}
        onClose={() => setNewTaskSlot(null)}
      />
      <EventQuickPopover
        event={selectedEvent?.event ?? null}
        anchor={selectedEvent?.el ?? null}
        onClose={() => setSelectedEvent(null)}
      />
      <div className={`${isMobile ? 'hidden' : 'flex flex-col'} h-full min-h-0 overflow-hidden`}>
        {/* Подсказка вместо пустой сетки: новичок не догадывается, что урок
            ставится кликом по слоту. Исчезает, как только в периоде есть события,
            и не показывается, пока запросы периода ещё в полёте. */}
        {events.length === 0 && !eventsLoading && (
          <div className="mb-2 flex shrink-0 items-center justify-center gap-2 rounded-lg border border-dashed px-3 py-2 text-center text-xs text-muted-foreground">
            <span>
              В этом периоде пусто. Кликни по свободному слоту, чтобы поставить урок,
              событие или задачу, либо
            </span>
            <Button size="sm" variant="outline" onClick={() => setOnboardOpen(true)}>
              Добавить ученика
            </Button>
          </div>
        )}
        <StudentOnboardingDialog open={onboardOpen} onClose={() => setOnboardOpen(false)} />
        <div className="min-h-0 flex-1">
        <FullCalendar
          ref={calendarRef}
          plugins={[dayGridPlugin, timeGridPlugin, interactionPlugin]}
          initialView="timeGridWeek"
          headerToolbar={{
            left:   'prev,next today',
            center: 'title',
            right:  'togglePersonal dayGridMonth,timeGridWeek,timeGridDay',
          }}
          // Тумблер живёт в тулбаре календаря, а не отдельной строкой над сеткой:
          // своя строка съедала высоту и висела без хозяина.
          customButtons={{
            togglePersonal: {
              text:  showPersonal ? 'Личные: вкл' : 'Личные: выкл',
              click: () => setShowPersonal((v) => !v),
            },
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
                      const task = arg.event.extendedProps.task as Task | undefined
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
