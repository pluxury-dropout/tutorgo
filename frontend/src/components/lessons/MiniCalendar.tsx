'use client'

import { useState } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface MiniCalendarProps {
  from: Date        // начало текущего периода (для подсветки)
  to: Date          // эксклюзивный конец (первый день после периода)
  onSelect: (from: Date, to: Date) => void
  onClose: () => void
}

const MONTH_NAMES = ['Январь','Февраль','Март','Апрель','Май','Июнь',
                     'Июль','Август','Сентябрь','Октябрь','Ноябрь','Декабрь']
const DAY_NAMES = ['Пн','Вт','Ср','Чт','Пт','Сб','Вс']

function isSameDay(a: Date, b: Date): boolean {
  return a.getFullYear() === b.getFullYear() &&
         a.getMonth() === b.getMonth() &&
         a.getDate() === b.getDate()
}

function startOfDay(d: Date): Date {
  const r = new Date(d)
  r.setHours(0, 0, 0, 0)
  return r
}

function addDays(d: Date, n: number): Date {
  const r = new Date(d)
  r.setDate(r.getDate() + n)
  return r
}

// Возвращает массив дней для сетки (пустые = null, дни месяца = Date)
function buildGrid(year: number, month: number): (Date | null)[] {
  const firstDay = new Date(year, month, 1)
  // getDay(): 0=Вс, 1=Пн … 6=Сб → нам нужен ISO (1=Пн, 7=Вс)
  const isoDay = firstDay.getDay() === 0 ? 7 : firstDay.getDay()
  const offset = isoDay - 1 // сколько пустых клеток перед первым
  const daysInMonth = new Date(year, month + 1, 0).getDate()

  const grid: (Date | null)[] = []
  for (let i = 0; i < offset; i++) grid.push(null)
  for (let d = 1; d <= daysInMonth; d++) grid.push(new Date(year, month, d))
  return grid
}

export function MiniCalendar({ from, to, onSelect, onClose }: MiniCalendarProps) {
  const today = startOfDay(new Date())

  // Начинаем просмотр с месяца начала текущего периода
  const [viewYear,  setViewYear]  = useState(from.getFullYear())
  const [viewMonth, setViewMonth] = useState(from.getMonth())

  // Первый выбранный день (ждём второй клик)
  const [pendingStart, setPendingStart] = useState<Date | null>(null)
  // День под курсором (для hover-превью)
  const [hoverDate, setHoverDate] = useState<Date | null>(null)

  const grid = buildGrid(viewYear, viewMonth)

  function prevMonth() {
    if (viewMonth === 0) { setViewMonth(11); setViewYear(y => y - 1) }
    else setViewMonth(m => m - 1)
  }
  function nextMonth() {
    if (viewMonth === 11) { setViewMonth(0); setViewYear(y => y + 1) }
    else setViewMonth(m => m + 1)
  }

  function handleDayClick(day: Date) {
    const d = startOfDay(day)
    if (!pendingStart) {
      // Первый клик: запоминаем начало
      setPendingStart(d)
    } else {
      if (d < pendingStart) {
        // Клик раньше начала → сброс, начинаем заново
        setPendingStart(d)
      } else {
        // Второй клик: передаём диапазон (to эксклюзивный = следующий день)
        onSelect(pendingStart, addDays(d, 1))
        setPendingStart(null)
        onClose()
      }
    }
  }

  // Вычислить эффективный диапазон для подсветки
  const effectiveStart = pendingStart ?? startOfDay(from)
  // Если ждём второй клик — показываем hover-превью, иначе текущий период
  const effectiveEnd = pendingStart
    ? (hoverDate && hoverDate >= pendingStart ? hoverDate : pendingStart)
    : startOfDay(addDays(to, -1)) // to эксклюзивный → последний включённый день

  function dayClass(day: Date | null): string {
    if (!day) return ''
    const d = startOfDay(day)
    const isStart = isSameDay(d, effectiveStart)
    const isEnd   = isSameDay(d, effectiveEnd)
    const inRange = d > effectiveStart && d < effectiveEnd
    const isToday = isSameDay(d, today)
    const isWeekend = day.getDay() === 0 || day.getDay() === 6

    return cn(
      'flex items-center justify-center text-xs cursor-pointer select-none h-8 w-full transition-colors',
      isWeekend && !isStart && !isEnd && !inRange && 'text-muted-foreground',
      !isStart && !isEnd && !inRange && !isWeekend && 'text-foreground hover:bg-accent hover:rounded-md',
      inRange && 'bg-muted text-foreground',
      (isStart || isEnd) && 'bg-foreground text-background font-semibold rounded-full',
      isToday && !isStart && !isEnd && 'font-bold',
    )
  }

  function selectCurrentWeek() {
    const now = new Date()
    const day = now.getDay()
    const diff = day === 0 ? -6 : 1 - day
    const mon = startOfDay(new Date(now))
    mon.setDate(now.getDate() + diff)
    const sun = addDays(mon, 7)
    onSelect(mon, sun)
    onClose()
  }

  function selectCurrentMonth() {
    const now = new Date()
    const start = new Date(now.getFullYear(), now.getMonth(), 1)
    const end   = new Date(now.getFullYear(), now.getMonth() + 1, 1)
    onSelect(start, end)
    onClose()
  }

  return (
    <div className="w-64 rounded-xl border bg-popover shadow-lg p-3 z-50">
      {/* Заголовок месяца */}
      <div className="flex items-center justify-between mb-2">
        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={prevMonth}>
          <ChevronLeft className="h-4 w-4" />
        </Button>
        <span className="text-sm font-semibold">
          {MONTH_NAMES[viewMonth]} {viewYear}
        </span>
        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={nextMonth}>
          <ChevronRight className="h-4 w-4" />
        </Button>
      </div>

      {/* Дни недели */}
      <div className="grid grid-cols-7 mb-1">
        {DAY_NAMES.map(d => (
          <span key={d} className="text-center text-[10px] font-medium text-muted-foreground py-1">
            {d}
          </span>
        ))}
      </div>

      {/* Сетка дней */}
      <div className="grid grid-cols-7">
        {grid.map((day, i) => (
          <div
            key={i}
            className={dayClass(day)}
            onClick={() => day && handleDayClick(day)}
            onMouseEnter={() => day && setHoverDate(startOfDay(day))}
            onMouseLeave={() => setHoverDate(null)}
          >
            {day?.getDate()}
          </div>
        ))}
      </div>

      {/* Шорткаты */}
      <div className="flex gap-2 mt-2 pt-2 border-t">
        <button
          onClick={selectCurrentWeek}
          className="flex-1 text-xs py-1.5 rounded-md border hover:bg-accent transition-colors"
        >
          Эта неделя
        </button>
        <button
          onClick={selectCurrentMonth}
          className="flex-1 text-xs py-1.5 rounded-md border hover:bg-accent transition-colors"
        >
          Этот месяц
        </button>
      </div>
    </div>
  )
}
