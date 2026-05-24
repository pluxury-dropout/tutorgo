'use client'

import { useState, useRef, useEffect } from 'react'
import { ChevronLeft, ChevronRight, CalendarDays } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { MiniCalendar } from './MiniCalendar'

interface PeriodPickerProps {
  from: Date
  to: Date          // эксклюзивный конец
  onChange: (from: Date, to: Date) => void
}

const MONTH_SHORT = ['янв','фев','мар','апр','мая','июн',
                     'июл','авг','сен','окт','ноя','дек']

function formatPeriodLabel(from: Date, to: Date): string {
  // to эксклюзивный, последний включённый день = to - 1 день
  const last = new Date(to)
  last.setDate(to.getDate() - 1)

  const f = from
  if (f.getMonth() === last.getMonth() && f.getFullYear() === last.getFullYear()) {
    return `${f.getDate()}–${last.getDate()} ${MONTH_SHORT[f.getMonth()]}`
  }
  return `${f.getDate()} ${MONTH_SHORT[f.getMonth()]} – ${last.getDate()} ${MONTH_SHORT[last.getMonth()]}`
}

function addDays(d: Date, n: number): Date {
  const r = new Date(d)
  r.setDate(r.getDate() + n)
  r.setHours(0, 0, 0, 0)
  return r
}

export function PeriodPicker({ from, to, onChange }: PeriodPickerProps) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  // Закрыть по клику вне
  useEffect(() => {
    if (!open) return
    function handleClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [open])

  return (
    <div className="relative" ref={containerRef}>
      <div className="flex items-center gap-1">
        {/* Стрелка назад */}
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={() => onChange(addDays(from, -7), addDays(to, -7))}
        >
          <ChevronLeft className="h-4 w-4" />
        </Button>

        {/* Кнопка периода */}
        <button
          onClick={() => setOpen(v => !v)}
          className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-md border text-sm font-medium hover:bg-accent transition-colors"
        >
          <CalendarDays className="h-3.5 w-3.5 text-primary" />
          <span>{formatPeriodLabel(from, to)}</span>
        </button>

        {/* Стрелка вперёд */}
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={() => onChange(addDays(from, 7), addDays(to, 7))}
        >
          <ChevronRight className="h-4 w-4" />
        </Button>
      </div>

      {/* Dropdown */}
      {open && (
        <div className="absolute right-0 top-full mt-1 z-50">
          <MiniCalendar
            from={from}
            to={to}
            onSelect={(f, t) => { onChange(f, t); setOpen(false) }}
            onClose={() => setOpen(false)}
          />
        </div>
      )}
    </div>
  )
}
