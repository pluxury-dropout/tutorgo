'use client'

import { useMemo } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { effectiveStatus, FC_COLORS, STATUS_LABELS } from '@/lib/lessonStatus'
import { buildGrid, dayKey, isSameLocalDay, shiftMonth } from '@/lib/monthGrid'
import type { CalendarLesson } from '@/types/api'

const MONTHS = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
]
const DOW = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']

// Метки идут в столбик и занимают всю ширину клетки, поэтому упирается всё в
// высоту ряда: больше двух — и сетка вытягивается на пол-экрана. Остальные
// сворачиваем в «+N».
const MAX_MARKERS = 2

const timeFmt = new Intl.DateTimeFormat('ru-RU', { hour: '2-digit', minute: '2-digit' })

function Marker({ lesson }: { lesson: CalendarLesson }) {
  const status = effectiveStatus(lesson)
  const c = FC_COLORS[status]
  const title = [
    timeFmt.format(new Date(lesson.scheduled_at)),
    lesson.subject,
    lesson.cycle_position != null && lesson.cycle_size != null
      ? `урок ${lesson.cycle_position} из ${lesson.cycle_size}`
      : null,
    STATUS_LABELS[status].toLowerCase(),
  ]
    .filter(Boolean)
    .join(' · ')

  return (
    <span
      title={title}
      style={{
        display: 'block',
        width: '100%',
        padding: '1px 2px',
        borderRadius: 5,
        // tabular-nums держит одинаковую ширину цифр — столбик времён не пляшет.
        fontVariantNumeric: 'tabular-nums',
        fontSize: 10.5,
        fontWeight: 600,
        lineHeight: 1.4,
        textAlign: 'center',
        whiteSpace: 'nowrap',
        background: c.bg,
        color: c.text,
        border: `1px solid ${c.border}`,
      }}
    >
      {timeFmt.format(new Date(lesson.scheduled_at))}
      {/* Номер в оплаченном цикле — надстрочным, чтобы не спорить со временем. */}
      {lesson.cycle_position != null && (
        <sup style={{ fontSize: 8, marginLeft: 1, opacity: 0.7 }}>{lesson.cycle_position}</sup>
      )}
    </span>
  )
}

interface LessonsCalendarProps {
  lessons: CalendarLesson[]
  /** Выбранный день — он же задаёт показанный месяц. */
  selected: Date
  onSelect: (day: Date) => void
  /** «Сегодня»; приходит снаружи, чтобы подсветка обновлялась по тику часов. */
  now: number
}

/**
 * Месячная сетка уроков. Полностью управляемая: показанный месяц выводится из
 * selected, поэтому пролистывание месяца двигает и выбранный день — одно
 * состояние вместо двух, и список под календарём никогда не показывает день из
 * невидимого месяца.
 */
export function LessonsCalendar({ lessons, selected, onSelect, now }: LessonsCalendarProps) {
  const byDay = useMemo(() => {
    const m = new Map<string, CalendarLesson[]>()
    for (const l of lessons) {
      const k = dayKey(new Date(l.scheduled_at))
      const bucket = m.get(k)
      if (bucket) bucket.push(l)
      else m.set(k, [l])
    }
    return m
  }, [lessons])

  const grid = buildGrid(selected.getFullYear(), selected.getMonth())
  const today = new Date(now)

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={() => onSelect(shiftMonth(selected, -1))}
          aria-label="Предыдущий месяц"
        >
          <ChevronLeft className="h-4 w-4" />
        </Button>
        <span style={{ fontSize: 14.5, fontWeight: 600 }}>
          {MONTHS[selected.getMonth()]} {selected.getFullYear()}
        </span>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={() => onSelect(shiftMonth(selected, 1))}
          aria-label="Следующий месяц"
        >
          <ChevronRight className="h-4 w-4" />
        </Button>
      </div>

      <div className="grid grid-cols-7">
        {DOW.map((d) => (
          <span key={d} className="text-center text-[11px] font-medium text-muted-foreground py-1">
            {d}
          </span>
        ))}
      </div>

      <div className="grid grid-cols-7 gap-y-0.5">
        {grid.map((day, i) => {
          if (!day) return <div key={i} />

          const items = byDay.get(dayKey(day)) ?? []
          const isSelected = isSameLocalDay(day, selected)
          const isToday = isSameLocalDay(day, today)

          return (
            <button
              key={i}
              type="button"
              onClick={() => onSelect(day)}
              aria-pressed={isSelected}
              className="flex flex-col items-center gap-0.5 px-0.5 py-1 rounded-lg transition-colors hover:bg-accent"
              style={{
                minHeight: 46,
                background: isSelected ? 'var(--muted)' : undefined,
                outline: isSelected ? '1px solid var(--border)' : undefined,
              }}
            >
              <span
                style={{
                  fontSize: 13,
                  fontWeight: isToday ? 700 : 400,
                  color: isToday ? 'var(--primary)' : 'var(--foreground)',
                }}
              >
                {day.getDate()}
              </span>
              <span className="w-full flex flex-col gap-0.5">
                {items.slice(0, MAX_MARKERS).map((l) => (
                  <Marker key={l.id} lesson={l} />
                ))}
                {items.length > MAX_MARKERS && (
                  <span
                    style={{
                      fontSize: 9.5,
                      fontWeight: 600,
                      textAlign: 'center',
                      color: 'var(--muted-foreground)',
                    }}
                  >
                    +{items.length - MAX_MARKERS}
                  </span>
                )}
              </span>
            </button>
          )
        })}
      </div>
    </div>
  )
}
