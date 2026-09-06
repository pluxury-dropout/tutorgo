'use client'

import { useState, useRef, useEffect, useMemo, type ReactNode } from 'react'
import { ChevronLeft, ChevronRight, Eye, EyeOff } from 'lucide-react'
import { useCalendarFeed, useRescheduleLesson } from '@/lib/hooks/useCalendar'
import { useUpdateEvent } from '@/lib/hooks/useEvents'
import { useRescheduleTask } from '@/lib/hooks/useTasks'
import { effectiveStatus } from '@/lib/lessonStatus'
import { KIND_COLORS, TASK_COLORS } from '@/lib/eventKind'
import { stripHtml } from '@/lib/stripHtml'
import { warnOnConflict } from '@/lib/conflictWarning'
import { dropSlot, HOUR_PX, HOURS, GRID_H, GUTTER_PX } from '@/lib/weekGridDrop'
import { useMinuteTick } from '@/lib/hooks/useMinuteTick'
import { LessonQuickPopover } from '@/components/lessons/LessonQuickPopover'
import { EventQuickPopover } from '@/components/calendar/EventQuickPopover'
import type { CalendarItem, CalendarLesson, Event as TutorEvent, LessonStatus } from '@/types/api'
import type { QuickLesson } from '@/components/lessons/LessonQuickPopover'

// ─── constants ────────────────────────────────────────────────────────────────

// long-press before drag engages, so a normal scroll/tap isn't hijacked
const DRAG_LONG_PRESS_MS = 300
const DRAG_SLOP_PX       = 10
// У края сетки неделя листается сама, как на десктопе (armEdgeTimer в page.tsx).
// Зона узкая и пауза длиннее, чем там: колонка на телефоне ~50px, широкая зона
// съела бы понедельник и воскресенье — на них стало бы не бросить.
const EDGE_ZONE_PX  = 18
const EDGE_FLIP_MS  = 600

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

/** Цвет блока: у урока — статус, у события — вид (свой цвет перекрывает), у задачи — готовность. */
function itemStyle(item: CalendarItem): { bg: string; text: string } {
  if (item.type === 'lesson') return STATUS_STYLE[effectiveStatus(item.lesson)]
  if (item.type === 'event') {
    const kind = KIND_COLORS[item.event.kind]
    return { bg: item.event.color || kind.bg, text: kind.text }
  }
  return item.task.status === 'done' ? TASK_COLORS.done : TASK_COLORS.active
}

/** Подпись блока: у урока — предмет (имя ученика уходит во вторую строку). */
function itemLabel(item: CalendarItem): string {
  if (item.type === 'lesson') return item.lesson.subject
  if (item.type === 'task')   return stripHtml(item.title)
  return item.title
}

/** Запись ленты плюс её колонка в кластере пересечений. */
type PlacedItem = CalendarItem & { _col: number; _cols: number }

/** Перетаскиваемый блок. Позиция — абсолютная (clientX/Y пальца), а не дельта:
 *  во время drag неделя может пролистаться, и дельта от точки захвата потеряла
 *  бы смысл. Размеры и точка захвата внутри карточки нужны призраку — он
 *  fixed-оверлей и живёт, даже когда исходная карточка ушла с экрана. */
type DragState = {
  item:      CalendarItem
  pointerId: number
  grabX: number; grabY: number
  w: number; h: number
  x: number; y: number
}

// ─── MobileWeekCalendar ───────────────────────────────────────────────────────

/** showPersonal приходит пропом от страницы: она владеет состоянием и пишет его
 *  в localStorage. Своё чтение ключа не перерисовало бы сетку при переключении,
 *  а тулбар FullCalendar с тем же тумблером на телефоне скрыт — отсюда своя
 *  кнопка в шапке: показать расписание ученику с экрана телефона вероятнее,
 *  чем с десктопа. */
export function MobileWeekCalendar({
  showPersonal,
  onTogglePersonal,
}: {
  showPersonal: boolean
  onTogglePersonal: () => void
}) {
  useMinuteTick()
  const today = useMemo(() => new Date(), [])

  const [weekStart, setWeekStart] = useState(() => getWeekStart(today))
  const [selDay, setSelDay]       = useState<number>(() => {
    const ws   = getWeekStart(today)
    const diff = Math.round((today.getTime() - ws.getTime()) / 86_400_000)
    return Math.max(0, Math.min(6, diff))
  })
  const [selectedLesson, setSelectedLesson] = useState<{ lesson: QuickLesson; el: HTMLElement } | null>(null)
  const [selectedEvent, setSelectedEvent]   = useState<{ event: TutorEvent; el: HTMLElement } | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)

  // ─── data fetching ──────────────────────────────────────────────────────────

  const rangeFrom = weekStart.toISOString()
  const rangeTo   = useMemo(() => {
    const d = new Date(weekStart)
    d.setDate(d.getDate() + 7)
    return d.toISOString()
  }, [weekStart])

  const { data: items = [] } = useCalendarFeed(rangeFrom, rangeTo)
  const reschedule     = useRescheduleLesson()
  const updateEvent    = useUpdateEvent()
  const rescheduleTask = useRescheduleTask()

  // ─── drag-to-reschedule (touch long-press) ─────────────────────────────────

  const dragTimerRef   = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pressRef       = useRef<{ x: number; y: number } | null>(null)
  const dragEngagedRef = useRef(false)
  const gridRef        = useRef<HTMLDivElement>(null)
  const edgeTimerRef   = useRef<ReturnType<typeof setInterval> | null>(null)
  const edgeSideRef    = useRef<'left' | 'right' | null>(null)

  const [drag, setDrag] = useState<DragState | null>(null)
  // Логика drag живёт на window (см. эффект ниже), а тот читает состояние из
  // ref: карточка под пальцем может исчезнуть на смене недели. Ref — источник
  // истины, state нужен только чтобы перерисовать призрак.
  const dragRef = useRef<DragState | null>(null)

  function applyDrag(next: DragState | null) {
    dragRef.current = next
    setDrag(next)
  }

  function clearPress() {
    if (dragTimerRef.current) { clearTimeout(dragTimerRef.current); dragTimerRef.current = null }
    pressRef.current = null
  }

  function clearEdgeTimer() {
    if (edgeTimerRef.current) { clearInterval(edgeTimerRef.current); edgeTimerRef.current = null }
    edgeSideRef.current = null
  }

  function handleCardPointerDown(e: React.PointerEvent<HTMLDivElement>, item: CalendarItem) {
    dragEngagedRef.current = false
    // currentTarget обнуляется после обработчика — геометрию снимаем сразу
    const rect      = e.currentTarget.getBoundingClientRect()
    const x         = e.clientX
    const y         = e.clientY
    const pointerId = e.pointerId
    pressRef.current = { x, y }
    // ponytail: touch-action:none lives statically in the card style — setting it here is too
    // late, the browser locks scroll behavior at touchstart (before this pointerdown fires)
    dragTimerRef.current = setTimeout(() => {
      dragEngagedRef.current = true
      applyDrag({
        item, pointerId,
        grabX: x - rect.left, grabY: y - rect.top,
        w: rect.width, h: rect.height,
        x, y,
      })
    }, DRAG_LONG_PRESS_MS)
  }

  /** Куда упал блок: колонка под его центром и время под его верхом — в той
   *  неделе, что показана сейчас (за drag её могло пролистать у края). */
  function commitDrag(d: DragState) {
    const grid = gridRef.current
    if (!grid) return
    const { day, minutes } = dropSlot(
      { left: d.x - d.grabX, top: d.y - d.grabY, width: d.w },
      grid.getBoundingClientRect(),
      d.item.duration_minutes,
    )

    const start = new Date(weekStart)
    start.setDate(start.getDate() + day)
    start.setHours(0, minutes, 0, 0)
    if (start.getTime() === new Date(d.item.starts_at).getTime()) return

    const iso      = start.toISOString()
    const duration = d.item.duration_minutes

    if (d.item.type === 'lesson') {
      const l = d.item.lesson
      reschedule.mutate(
        { id: l.id, data: { scheduled_at: iso, duration_minutes: duration, status: l.status, notes: l.notes } },
        { onSuccess: () => warnOnConflict({ starts_at: iso, duration_minutes: duration, exclude_type: 'lesson', exclude_id: l.id }) },
      )
      return
    }

    if (d.item.type === 'event') {
      const ev = d.item.event
      updateEvent.mutate(
        {
          id:   ev.id,
          data: {
            title:            ev.title,
            kind:             ev.kind,
            starts_at:        iso,
            duration_minutes: duration,
            color:            ev.color,
            location:         ev.location,
            notes:            ev.notes,
          },
        },
        { onSuccess: () => warnOnConflict({ starts_at: iso, duration_minutes: duration, exclude_type: 'event', exclude_id: ev.id }) },
      )
      return
    }

    // Задачи занятостью не считаются — для них проверки нет.
    const t = d.item.task
    rescheduleTask.mutate({
      id:   t.id,
      data: { title: t.title, scheduled_at: iso, duration_minutes: duration, status: t.status },
    })
  }

  function endDrag(e: PointerEvent, commit: boolean) {
    clearPress()
    clearEdgeTimer()
    const d = dragRef.current
    if (!d || e.pointerId !== d.pointerId) return
    applyDrag(null)
    if (commit) commitDrag(d)
  }
  // latest-ref: слушатели window ставятся один раз, а завершение drag читает
  // свежие weekStart и мутации.
  const endDragRef = useRef(endDrag)
  useEffect(() => { endDragRef.current = endDrag })

  useEffect(() => {
    const onMove = (e: PointerEvent) => {
      const d = dragRef.current
      if (d) {
        if (e.pointerId !== d.pointerId) return
        const next = { ...d, x: e.clientX, y: e.clientY }
        dragRef.current = next
        setDrag(next)

        // край сетки — листаем неделю, пока палец там
        const grid = gridRef.current
        if (!grid) return
        const r    = grid.getBoundingClientRect()
        const side = e.clientX < r.left + EDGE_ZONE_PX  ? 'left'
                   : e.clientX > r.right - EDGE_ZONE_PX ? 'right'
                   : null
        if (side !== edgeSideRef.current) {
          clearEdgeTimer()
          if (side) {
            edgeSideRef.current = side
            edgeTimerRef.current = setInterval(() => {
              setWeekStart(ws => {
                const next = new Date(ws)
                next.setDate(next.getDate() + (side === 'left' ? -7 : 7))
                return next
              })
            }, EDGE_FLIP_MS)
          }
        }
        return
      }
      const p = pressRef.current
      if (p && (Math.abs(e.clientX - p.x) > DRAG_SLOP_PX || Math.abs(e.clientY - p.y) > DRAG_SLOP_PX)) {
        clearPress()
      }
    }
    const onUp     = (e: PointerEvent) => endDragRef.current(e, true)
    const onCancel = (e: PointerEvent) => endDragRef.current(e, false)

    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
    window.addEventListener('pointercancel', onCancel)
    return () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
      window.removeEventListener('pointercancel', onCancel)
      clearPress()
      clearEdgeTimer()
    }
  }, [])

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
    const result: Record<number, PlacedItem[]> = {}
    const visible = items.filter(
      it => !(it.type === 'event' && it.event.kind === 'personal' && !showPersonal),
    )

    for (let d = 0; d < 7; d++) {
      const dayDate = new Date(weekStart)
      dayDate.setDate(dayDate.getDate() + d)

      const dayItems: PlacedItem[] = visible
        .filter(it => isSameLocalDay(new Date(it.starts_at), dayDate))
        .sort((a, b) => a.starts_at.localeCompare(b.starts_at))
        .map(it => ({ ...it, _col: 0, _cols: 1 }))

      // pack into columns, but only within connected overlap clusters —
      // an item with no overlap anywhere that day must stay full-width
      const out: PlacedItem[] = []
      let cluster: PlacedItem[] = []
      let clusterCols: PlacedItem[][] = []
      let clusterEnd = -Infinity

      const flushCluster = () => {
        for (const ev of cluster) ev._cols = clusterCols.length
        out.push(...cluster)
        cluster = []
        clusterCols = []
      }

      for (const ev of dayItems) {
        const startMin = toMinutes(ev.starts_at)
        const endMin   = toMinutes(ev.starts_at, ev.duration_minutes)

        if (startMin >= clusterEnd) flushCluster()
        clusterEnd = Math.max(clusterEnd, endMin)

        let placed = false
        for (let c = 0; c < clusterCols.length; c++) {
          const last    = clusterCols[c][clusterCols[c].length - 1]
          const lastEnd = toMinutes(last.starts_at, last.duration_minutes)
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
  }, [items, weekStart, showPersonal])

  // ─── week navigation ────────────────────────────────────────────────────────

  function prevWeek() {
    setWeekStart(ws => { const d = new Date(ws); d.setDate(d.getDate() - 7); return d })
  }
  function nextWeek() {
    setWeekStart(ws => { const d = new Date(ws); d.setDate(d.getDate() + 7); return d })
  }

  // ─── open lesson popover ────────────────────────────────────────────────────

  function openLesson(l: CalendarLesson, el: HTMLElement) {
    if (dragEngagedRef.current) { dragEngagedRef.current = false; return }
    setSelectedLesson({
      el,
      lesson: {
        id:              l.id,
        courseId:        l.course_id,
        title:           l.is_group
          ? l.subject
          : `${l.subject}${l.student_name ? ` — ${l.student_name}` : ''}`,
        status:          effectiveStatus(l),
        notes:           l.notes,
        isGroup:         l.is_group,
        scheduledAt:     l.scheduled_at,
        durationMinutes: l.duration_minutes,
      },
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
      <LessonQuickPopover
        lesson={selectedLesson?.lesson ?? null}
        anchor={selectedLesson?.el ?? null}
        onClose={() => setSelectedLesson(null)}
      />
      <EventQuickPopover
        event={selectedEvent?.event ?? null}
        anchor={selectedEvent?.el ?? null}
        onClose={() => setSelectedEvent(null)}
      />

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
          <IconBtn
            onClick={onTogglePersonal}
            label={showPersonal ? 'Скрыть личные события' : 'Показать личные события'}
          >
            {showPersonal ? <Eye size={16} /> : <EyeOff size={16} />}
          </IconBtn>
          <IconBtn onClick={prevWeek} label="Предыдущая неделя"><ChevronLeft size={16} /></IconBtn>
          <IconBtn onClick={nextWeek} label="Следующая неделя"><ChevronRight size={16} /></IconBtn>
        </header>

        {/* ── Day strip ── */}
        <div style={{
          display: 'flex', padding: `8px 6px 8px ${GUTTER_PX}px`,
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
            <div style={{ width: GUTTER_PX, flexShrink: 0, position: 'relative', height: GRID_H }}>
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
                    const startMin = toMinutes(ev.starts_at)
                    const endMin   = startMin + ev.duration_minutes
                    const top      = (startMin - HOURS[0] * 60) / 60 * HOUR_PX
                    const height   = Math.max(20, (endMin - startMin) / 60 * HOUR_PX - 2)
                    const colW     = 100 / ev._cols
                    const left     = colW * ev._col
                    const style    = itemStyle(ev)
                    const isPast    = new Date(ev.starts_at).getTime() + ev.duration_minutes * 60_000 < Date.now()
                    const isDragged = drag?.item.id === ev.id
                    // Время берём с верхнего уровня ленты: оптимистичный патч
                    // переноса правит его, а не вложенный урок, — иначе второй
                    // подряд перенос отсчитывался бы от старого слота.
                    const lesson    = ev.type === 'lesson'
                      ? { ...ev.lesson, scheduled_at: ev.starts_at, duration_minutes: ev.duration_minutes }
                      : null
                    const struck    = (ev.type === 'lesson' && ev.lesson.status === 'cancelled')
                      || (ev.type === 'task' && ev.task.status === 'done')

                    return (
                      <div
                        key={`${ev.type}:${ev.id}`}
                        onClick={(e) => {
                          // клик после переноса гасим: он может прилететь и на
                          // чужую карточку — палец отпустили над ней
                          if (dragEngagedRef.current) { dragEngagedRef.current = false; return }
                          if (lesson) openLesson(lesson, e.currentTarget)
                          else if (ev.type === 'event') setSelectedEvent({ event: ev.event, el: e.currentTarget })
                        }}
                        onPointerDown={(e) => handleCardPointerDown(e, ev)}
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
                          zIndex: 2,
                          userSelect: 'none',
                          WebkitUserSelect: 'none',
                          // ponytail: must be static — see handleCardPointerDown.
                          touchAction: 'none',
                          filter:          isPast ? 'brightness(0.9)' : undefined,
                          textDecoration:  struck ? 'line-through'    : undefined,
                          opacity:    isDragged ? 0.35 : undefined,
                        }}
                      >
                        <div style={{ fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {itemLabel(ev)}
                        </div>
                        {height > 30 && lesson && !lesson.is_group && lesson.student_name && (
                          <div style={{ fontSize: 9, opacity: 0.75, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                            {lesson.student_name}
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

        {/* Призрак перетаскиваемого блока: fixed, поэтому переживает и смену
            недели, и перезапрос ленты — исходной карточки к дропу может уже
            не быть в DOM. */}
        {drag && (
          <div style={{
            position: 'fixed',
            left: drag.x - drag.grabX, top: drag.y - drag.grabY,
            width: drag.w, height: drag.h,
            background: itemStyle(drag.item).bg,
            color:      itemStyle(drag.item).text,
            borderRadius: 4, padding: '2px 4px',
            fontSize: 9.5, lineHeight: 1.15,
            fontWeight: 600,
            overflow: 'hidden',
            pointerEvents: 'none',
            zIndex: 60,
            transform: 'scale(1.03)',
            boxShadow: '0 6px 16px -4px rgba(0,0,0,0.35)',
          }}>
            {itemLabel(drag.item)}
          </div>
        )}

      </div>
    </>
  )
}

// ─── small helper ─────────────────────────────────────────────────────────────

function IconBtn({ onClick, label, children }: { onClick?: () => void; label?: string; children: ReactNode }) {
  return (
    <button
      onClick={onClick}
      aria-label={label}
      title={label}
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
