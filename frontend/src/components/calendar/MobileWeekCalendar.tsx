'use client'

import { useState, useRef, useEffect, useMemo, type ReactNode } from 'react'
import {
  Plus, Search, ChevronLeft, ChevronRight,
} from 'lucide-react'
import { useCalendar, useRescheduleLesson } from '@/lib/hooks/useCalendar'
import { LessonQuickDialog } from '@/components/lessons/LessonQuickDialog'
import type { CalendarLesson, LessonStatus } from '@/types/api'
import type { QuickLesson } from '@/components/lessons/LessonQuickDialog'

// ─── constants ────────────────────────────────────────────────────────────────

const HOUR_PX  = 56
const HOURS    = [8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21]
const GRID_H   = HOURS.length * HOUR_PX

// long-press before drag engages, so a normal scroll/tap isn't hijacked
const DRAG_LONG_PRESS_MS = 300
const DRAG_SLOP_PX       = 10

const DOW_MINI = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']
const MONTHS_GEN = [
  'января','февраля','марта','апреля','мая','июня',
  'июля','августа','сентября','октября','ноября','декабря',
]
const MONTHS_NOM = [
  'Январь','Февраль','Март','Апрель','Май','Июнь',
  'Июль','Август','Сентябрь','Октябрь','Ноябрь','Декабрь',
]

const STATUS_STYLE: Record<LessonStatus, { bg: string; text: string }> = {
  scheduled: { bg: 'var(--cal-scheduled-bg)', text: 'var(--cal-scheduled-text)' },
  completed: { bg: 'var(--cal-completed-bg)', text: 'var(--cal-completed-text)' },
  cancelled: { bg: 'var(--cal-cancelled-bg)', text: 'var(--cal-cancelled-text)' },
  missed:    { bg: 'var(--cal-missed-bg)',    text: 'var(--cal-missed-text)'    },
}

// ─── helpers ──────────────────────────────────────────────────────────────────

function getWeekStart(d: Date): Date {
  const day  = d.getDay()
  const diff = day === 0 ? -6 : 1 - day
  const mon  = new Date(d)
  mon.setDate(d.getDate() + diff)
  mon.setHours(0, 0, 0, 0)
  return mon
}

function isSameLocalDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth()    === b.getMonth()    &&
    a.getDate()     === b.getDate()
  )
}

function toMinutes(iso: string, durationMinutes?: number): number {
  const d = new Date(iso)
  return durationMinutes !== undefined
    ? d.getHours() * 60 + d.getMinutes() + durationMinutes
    : d.getHours() * 60 + d.getMinutes()
}

// ─── MobileWeekCalendar ───────────────────────────────────────────────────────

export function MobileWeekCalendar() {
  const today = useMemo(() => new Date(), [])

  const [weekStart, setWeekStart] = useState(() => getWeekStart(today))
  const [selDay, setSelDay]       = useState<number>(() => {
    const ws   = getWeekStart(today)
    const diff = Math.round((today.getTime() - ws.getTime()) / 86_400_000)
    return Math.max(0, Math.min(6, diff))
  })
  const [selectedLesson, setSelectedLesson] = useState<QuickLesson | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)

  // ─── data fetching ──────────────────────────────────────────────────────────

  const rangeFrom = weekStart.toISOString()
  const rangeTo   = useMemo(() => {
    const d = new Date(weekStart)
    d.setDate(d.getDate() + 7)
    return d.toISOString()
  }, [weekStart])

  const { data: lessons = [] } = useCalendar(rangeFrom, rangeTo)
  const reschedule = useRescheduleLesson()

  // ─── drag-to-reschedule (touch long-press) ─────────────────────────────────

  const dragTimerRef    = useRef<ReturnType<typeof setTimeout> | null>(null)
  const dragStartRef    = useRef<{ x: number; y: number } | null>(null)
  const dragEngagedRef  = useRef(false)
  const dragTargetRef   = useRef<HTMLDivElement | null>(null)
  const gridRef         = useRef<HTMLDivElement>(null)
  const [drag, setDrag] = useState<{ lesson: CalendarLesson; pointerId: number; deltaY: number; deltaX: number } | null>(null)

  function clearDragTimer() {
    if (dragTimerRef.current) { clearTimeout(dragTimerRef.current); dragTimerRef.current = null }
    dragStartRef.current = null
    if (dragTargetRef.current) {
      dragTargetRef.current.style.touchAction = ''
      dragTargetRef.current = null
    }
  }

  function handleEventPointerDown(e: React.PointerEvent<HTMLDivElement>, lesson: CalendarLesson) {
    dragStartRef.current = { x: e.clientX, y: e.clientY }
    const target = e.currentTarget
    dragTargetRef.current = target
    // ponytail: touch-action must be set at pointerdown, not later — browser evaluates it once on touch start
    target.style.touchAction = 'none'
    const pointerId = e.pointerId
    dragTimerRef.current = setTimeout(() => {
      target.setPointerCapture(pointerId)
      dragEngagedRef.current = true
      setDrag({ lesson, pointerId, deltaY: 0, deltaX: 0 })
    }, DRAG_LONG_PRESS_MS)
  }

  function handleEventPointerMove(e: React.PointerEvent<HTMLDivElement>) {
    if (drag && e.pointerId === drag.pointerId) {
      setDrag(d => d && {
        ...d,
        deltaY: e.clientY - (dragStartRef.current?.y ?? e.clientY),
        deltaX: e.clientX - (dragStartRef.current?.x ?? e.clientX),
      })
      return
    }
    if (dragStartRef.current) {
      const dx = Math.abs(e.clientX - dragStartRef.current.x)
      const dy = Math.abs(e.clientY - dragStartRef.current.y)
      if (dx > DRAG_SLOP_PX || dy > DRAG_SLOP_PX) clearDragTimer()
    }
  }

  function handleEventPointerUp(e: React.PointerEvent<HTMLDivElement>) {
    clearDragTimer()
    if (!drag || e.pointerId !== drag.pointerId) return
    e.currentTarget.style.touchAction = ''
    const minutesDelta = Math.round(drag.deltaY / HOUR_PX * 2) * 30
    const colWidth     = gridRef.current ? gridRef.current.clientWidth / 7 : 0
    const dayDelta     = colWidth > 0 ? Math.round(drag.deltaX / colWidth) : 0
    if (minutesDelta !== 0 || dayDelta !== 0) {
      const base     = new Date(drag.lesson.scheduled_at)
      base.setDate(base.getDate() + dayDelta)
      const newStart = new Date(base.getTime() + minutesDelta * 60_000)
      reschedule.mutate({
        id:   drag.lesson.id,
        data: {
          scheduled_at:     newStart.toISOString(),
          duration_minutes: drag.lesson.duration_minutes,
          status:           drag.lesson.status,
          notes:            drag.lesson.notes,
        },
      })
    }
    setDrag(null)
  }

  function handleEventPointerCancel(e: React.PointerEvent<HTMLDivElement>) {
    clearDragTimer()
    if (drag && e.pointerId === drag.pointerId) {
      e.currentTarget.style.touchAction = ''
      setDrag(null)
    }
  }

  // ─── auto-scroll ────────────────────────────────────────────────────────────

  useEffect(() => {
    if (scrollRef.current) {
      const h   = today.getHours()
      const top = Math.max(0, (h - HOURS[0]) * HOUR_PX - 60)
      scrollRef.current.scrollTop = top
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // ─── now line ───────────────────────────────────────────────────────────────

  const nowTop = useMemo(() => {
    const h = today.getHours()
    const m = today.getMinutes()
    if (h < HOURS[0] || h > HOURS[HOURS.length - 1]) return null
    return (h - HOURS[0]) * HOUR_PX + (m / 60) * HOUR_PX
  }, [today])

  // ─── layout: overlap columns per day ───────────────────────────────────────

  const layoutByDay = useMemo(() => {
    const result: Record<number, (CalendarLesson & { _col: number; _cols: number })[]> = {}

    for (let d = 0; d < 7; d++) {
      const dayDate = new Date(weekStart)
      dayDate.setDate(dayDate.getDate() + d)

      const dayLessons = lessons
        .filter(l => isSameLocalDay(new Date(l.scheduled_at), dayDate))
        .sort((a, b) => a.scheduled_at.localeCompare(b.scheduled_at))
        .map(l => ({ ...l, _col: 0, _cols: 1 }))

      // pack into columns, but only within connected overlap clusters —
      // a lesson with no overlap anywhere that day must stay full-width
      const out: typeof dayLessons = []
      let cluster: typeof dayLessons = []
      let clusterCols: (typeof dayLessons)[] = []
      let clusterEnd = -Infinity

      const flushCluster = () => {
        for (const ev of cluster) ev._cols = clusterCols.length
        out.push(...cluster)
        cluster = []
        clusterCols = []
      }

      for (const ev of dayLessons) {
        const startMin = toMinutes(ev.scheduled_at)
        const endMin   = toMinutes(ev.scheduled_at, ev.duration_minutes)

        if (startMin >= clusterEnd) flushCluster()
        clusterEnd = Math.max(clusterEnd, endMin)

        let placed = false
        for (let c = 0; c < clusterCols.length; c++) {
          const last    = clusterCols[c][clusterCols[c].length - 1]
          const lastEnd = toMinutes(last.scheduled_at, last.duration_minutes)
          if (lastEnd <= startMin) {
            clusterCols[c].push(ev)
            ev._col = c
            placed  = true
            break
          }
        }
        if (!placed) {
          clusterCols.push([ev])
          ev._col = clusterCols.length - 1
        }
        cluster.push(ev)
      }
      flushCluster()

      result[d] = out
    }
    return result
  }, [lessons, weekStart])

  // ─── week navigation ────────────────────────────────────────────────────────

  function prevWeek() {
    setWeekStart(ws => { const d = new Date(ws); d.setDate(d.getDate() - 7); return d })
  }
  function nextWeek() {
    setWeekStart(ws => { const d = new Date(ws); d.setDate(d.getDate() + 7); return d })
  }

  // ─── open lesson popover ────────────────────────────────────────────────────

  function openLesson(l: CalendarLesson) {
    if (dragEngagedRef.current) { dragEngagedRef.current = false; return }
    setSelectedLesson({
      id:              l.id,
      courseId:        l.course_id,
      title:           l.is_group
        ? l.subject
        : `${l.subject}${l.student_name ? ` — ${l.student_name}` : ''}`,
      status:          l.status,
      notes:           l.notes,
      isGroup:         l.is_group,
      scheduledAt:     l.scheduled_at,
      durationMinutes: l.duration_minutes,
    })
  }

  // ─── header labels ──────────────────────────────────────────────────────────

  const weekEnd     = new Date(weekStart)
  weekEnd.setDate(weekEnd.getDate() + 6)
  const titleMonth  = weekStart.getMonth() === weekEnd.getMonth()
    ? MONTHS_NOM[weekStart.getMonth()]
    : `${MONTHS_NOM[weekStart.getMonth()]} – ${MONTHS_NOM[weekEnd.getMonth()]}`

  // ─── render ─────────────────────────────────────────────────────────────────

  return (
    <>
      <LessonQuickDialog lesson={selectedLesson} onClose={() => setSelectedLesson(null)} />

      <div style={{ height: '100%', display: 'flex', flexDirection: 'column', background: 'var(--background)', overflow: 'hidden' }}>

        {/* ── Header ── */}
        <header style={{
          display: 'flex', alignItems: 'center', gap: 8,
          padding: '12px 14px 8px', flexShrink: 0,
          borderBottom: '1px solid var(--border)',
          background: 'var(--background)',
        }}>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 17, fontWeight: 600, letterSpacing: '-0.01em' }}>
              {titleMonth} {weekStart.getFullYear()}
            </div>
            <div style={{ fontSize: 11.5, color: 'var(--muted-foreground)', marginTop: 1 }}>
              {weekStart.getDate()} – {weekEnd.getDate()} {MONTHS_GEN[weekEnd.getMonth()]}
            </div>
          </div>
          <IconBtn onClick={prevWeek}><ChevronLeft size={16} /></IconBtn>
          <IconBtn onClick={nextWeek}><ChevronRight size={16} /></IconBtn>
          <IconBtn><Search size={15} /></IconBtn>
        </header>

        {/* ── Day strip ── */}
        <div style={{
          display: 'flex', padding: '8px 6px 8px 30px',
          borderBottom: '1px solid var(--border)',
          background: 'var(--background)', flexShrink: 0,
        }}>
          {[0, 1, 2, 3, 4, 5, 6].map(d => {
            const date    = new Date(weekStart)
            date.setDate(date.getDate() + d)
            const isToday = isSameLocalDay(date, today)
            const isSel   = d === selDay
            return (
              <button
                key={d}
                onClick={() => setSelDay(d)}
                style={{
                  flex: 1, background: 'transparent', border: 0, cursor: 'pointer',
                  padding: '2px 0', display: 'flex', flexDirection: 'column',
                  alignItems: 'center', gap: 2,
                }}
              >
                <span style={{ fontSize: 10, color: 'var(--muted-foreground)', fontWeight: 500, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                  {DOW_MINI[d]}
                </span>
                <span style={{
                  fontSize: 14, fontWeight: 600,
                  color:       isToday ? 'var(--background)' : isSel ? 'var(--foreground)' : 'var(--muted-foreground)',
                  background:  isToday ? 'var(--foreground)'  : isSel ? 'var(--muted)'      : 'transparent',
                  width: 26, height: 26, borderRadius: 999,
                  display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                  fontVariantNumeric: 'tabular-nums',
                }}>
                  {date.getDate()}
                </span>
              </button>
            )
          })}
        </div>

        {/* ── Calendar grid ── */}
        <div
          ref={scrollRef}
          style={{ flex: 1, overflow: 'auto', background: 'var(--background)', minHeight: 0 }}
        >
          <div ref={gridRef} style={{ display: 'flex', position: 'relative', height: GRID_H }}>

            {/* Time gutter */}
            <div style={{ width: 30, flexShrink: 0, position: 'relative', height: GRID_H }}>
              {HOURS.map((h, i) => (
                <div key={h} style={{
                  position: 'absolute', top: i * HOUR_PX - 6, right: 4,
                  fontSize: 11, color: 'var(--muted-foreground)',
                  fontVariantNumeric: 'tabular-nums', lineHeight: 1,
                }}>
                  {i === 0 ? '' : String(h).padStart(2, '0')}
                </div>
              ))}
            </div>

            {/* Day columns */}
            {[0, 1, 2, 3, 4, 5, 6].map(d => {
              const layout  = layoutByDay[d]
              const dayDate = new Date(weekStart)
              dayDate.setDate(dayDate.getDate() + d)
              const isToday = isSameLocalDay(dayDate, today)

              return (
                <div key={d} style={{
                  flex: 1, position: 'relative', height: GRID_H, minWidth: 0,
                  borderLeft: '1px solid var(--border)',
                }}>
                  {/* Hour rows */}
                  {HOURS.map((_, hi) => (
                    <div key={hi} style={{
                      position: 'absolute', left: 0, right: 0,
                      top: hi * HOUR_PX, height: HOUR_PX,
                      borderBottom: '1px solid var(--border)',
                      opacity: 0.4,
                    }} />
                  ))}

                  {/* Now indicator */}
                  {isToday && nowTop !== null && (
                    <div style={{
                      position: 'absolute', left: 0, right: 0, top: nowTop,
                      borderTop: '1.5px solid rgba(239,68,68,0.6)',
                      pointerEvents: 'none', zIndex: 5,
                    }}>
                      <div style={{
                        position: 'absolute', left: -4, top: -4.5,
                        width: 8, height: 8, borderRadius: 999,
                        background: 'rgba(239,68,68,0.6)',
                      }} />
                    </div>
                  )}

                  {/* Events */}
                  {layout?.map(ev => {
                    const startMin = toMinutes(ev.scheduled_at)
                    const endMin   = startMin + ev.duration_minutes
                    const top      = (startMin - HOURS[0] * 60) / 60 * HOUR_PX
                    const height   = Math.max(20, (endMin - startMin) / 60 * HOUR_PX - 2)
                    const colW     = 100 / ev._cols
                    const left     = colW * ev._col
                    const style    = STATUS_STYLE[ev.status]
                    const isPast    = new Date(ev.scheduled_at).getTime() + ev.duration_minutes * 60_000 < Date.now()
                    const isDragged = drag?.lesson.id === ev.id

                    return (
                      <div
                        key={ev.id}
                        onClick={() => openLesson(ev)}
                        onPointerDown={(e) => handleEventPointerDown(e, ev)}
                        onPointerMove={handleEventPointerMove}
                        onPointerUp={handleEventPointerUp}
                        onPointerCancel={handleEventPointerCancel}
                        style={{
                          position: 'absolute',
                          top, height,
                          left:   `calc(${left}% + 1px)`,
                          width:  `calc(${colW}% - 2px)`,
                          background: style.bg,
                          color:      style.text,
                          borderRadius: 4,
                          padding: '2px 4px',
                          fontSize: 9.5, lineHeight: 1.15,
                          cursor: 'pointer',
                          overflow: 'hidden',
                          zIndex:    isDragged ? 6 : 2,
                          userSelect: 'none',
                          WebkitUserSelect: 'none',
                          filter:          isPast              ? 'brightness(0.9)'    : undefined,
                          textDecoration:  ev.status === 'cancelled' ? 'line-through' : undefined,
                          transform:  isDragged ? `translate(${drag.deltaX}px,${drag.deltaY}px) scale(1.03)` : undefined,
                          boxShadow:  isDragged ? '0 6px 16px -4px rgba(0,0,0,0.35)' : undefined,
                        }}
                      >
                        <div style={{ fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {ev.subject}
                        </div>
                        {height > 30 && !ev.is_group && ev.student_name && (
                          <div style={{ fontSize: 9, opacity: 0.75, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                            {ev.student_name}
                          </div>
                        )}
                      </div>
                    )
                  })}
                </div>
              )
            })}
          </div>
        </div>

        {/* ── FAB ── */}
        <button
          style={{
            position: 'fixed', right: 16, bottom: 'calc(env(safe-area-inset-bottom, 0px) + 66px)',
            width: 52, height: 52, borderRadius: 999,
            background: 'var(--foreground)', color: 'var(--background)',
            border: 0, cursor: 'pointer',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            boxShadow: '0 8px 24px -6px rgba(0,0,0,0.28)',
            zIndex: 30,
          }}
          aria-label="Новое занятие"
        >
          <Plus size={22} strokeWidth={2} />
        </button>

      </div>
    </>
  )
}

// ─── small helper ─────────────────────────────────────────────────────────────

function IconBtn({ onClick, children }: { onClick?: () => void; children: ReactNode }) {
  return (
    <button
      onClick={onClick}
      style={{
        width: 30, height: 30,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        borderRadius: 6, border: 0, background: 'transparent',
        cursor: 'pointer', color: 'var(--muted-foreground)',
      }}
    >
      {children}
    </button>
  )
}
