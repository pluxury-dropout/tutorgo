'use client'

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const HOURS   = Array.from({ length: 24 }, (_, i) => i)
const MINUTES = [0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55]
const pad     = (n: number) => String(n).padStart(2, '0')

interface TimePickerProps {
  hour:             string
  minute:           string
  onHourChange:     (h: string) => void
  onMinuteChange:   (m: string) => void
  hourPlaceholder?: string
}

export function TimePicker({ hour, minute, onHourChange, onMinuteChange, hourPlaceholder }: TimePickerProps) {
  return (
    <div className="flex gap-2">
      <Select value={hour === '' ? null : hour} onValueChange={(v) => onHourChange(v ?? '')}>
        <SelectTrigger className="w-20">
          <SelectValue placeholder={hourPlaceholder ?? '—'} />
        </SelectTrigger>
        <SelectContent className="max-h-48">
          {HOURS.map((h) => (
            <SelectItem key={h} value={String(h)}>{pad(h)}</SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={minute} onValueChange={(v) => onMinuteChange(v ?? '0')}>
        <SelectTrigger className="w-16">
          <SelectValue />
        </SelectTrigger>
        <SelectContent className="max-h-48">
          {MINUTES.map((m) => (
            <SelectItem key={m} value={String(m)}>{pad(m)}</SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
