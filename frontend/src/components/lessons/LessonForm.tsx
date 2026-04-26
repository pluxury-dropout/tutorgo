'use client'

import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'

import { lessonSchema, LessonFormValues } from '@/schemas/lesson'
import { Lesson, ApiError } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

const STATUS_LABELS: Record<string, string> = {
  scheduled:  'Запланирован',
  completed:  'Проведён',
  cancelled:  'Отменён',
  missed:     'Пропущен',
}

const WEEK_DAYS = [
  { label: 'Пн', iso: 1 },
  { label: 'Вт', iso: 2 },
  { label: 'Ср', iso: 3 },
  { label: 'Чт', iso: 4 },
  { label: 'Пт', iso: 5 },
  { label: 'Сб', iso: 6 },
  { label: 'Вс', iso: 7 },
]

export type RecurrenceType = 'weekly_same' | 'weekly_custom' | 'every_n_weeks'

export interface RecurrenceOptions {
  type:    RecurrenceType
  days?:   number[]   // ISO weekdays: 1=Mon … 7=Sun (for weekly_custom)
  n?:      number     // interval in weeks (for every_n_weeks)
  count?:  number     // if undefined — generate until courseEndAt or 52 weeks
}

interface LessonFormProps {
  open:         boolean
  onClose:      () => void
  onSubmit:     (data: LessonFormValues, recurrence?: RecurrenceOptions) => Promise<void>
  initial?:     Lesson
  courseEndAt?: string   // ISO date — upper bound for open-ended recurrence
}

const HOURS   = Array.from({ length: 24 }, (_, i) => i)
const MINUTES = [0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55]
const pad     = (n: number) => n.toString().padStart(2, '0')

export function LessonForm({ open, onClose, onSubmit, initial, courseEndAt }: LessonFormProps) {
  const {
    register,
    handleSubmit,
    reset,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<LessonFormValues>({ resolver: zodResolver(lessonSchema) })

  const [dateVal, setDateVal] = useState('')
  const [hourVal, setHourVal] = useState('9')
  const [minVal,  setMinVal]  = useState('0')

  const [recEnabled,  setRecEnabled]  = useState(false)
  const [recType,     setRecType]     = useState<RecurrenceType>('weekly_same')
  const [recDays,     setRecDays]     = useState<number[]>([])
  const [recN,        setRecN]        = useState(2)
  const [recCount,    setRecCount]    = useState<number | ''>('')

  useEffect(() => {
    if (initial) {
      const dt   = new Date(initial.scheduled_at)
      const year = dt.getFullYear()
      const mon  = String(dt.getMonth() + 1).padStart(2, '0')
      const day  = String(dt.getDate()).padStart(2, '0')
      const d    = `${year}-${mon}-${day}`
      const h    = dt.getHours()
      const m  = Math.round(dt.getMinutes() / 5) * 5 % 60
      setDateVal(d)
      setHourVal(String(h))
      setMinVal(String(m))
      reset({
        scheduled_at:     `${d}T${pad(h)}:${pad(m)}`,
        duration_minutes: initial.duration_minutes,
        status:           initial.status,
        notes:            initial.notes ?? '',
      })
    } else {
      setDateVal('')
      setHourVal('9')
      setMinVal('0')
      reset({ scheduled_at: '', duration_minutes: 60, status: 'scheduled', notes: '' })
    }
    setRecEnabled(false)
    setRecType('weekly_same')
    setRecDays([])
    setRecN(2)
    setRecCount('')
  }, [initial, open, reset])

  useEffect(() => {
    if (dateVal) setValue('scheduled_at', `${dateVal}T${pad(Number(hourVal))}:${pad(Number(minVal))}`)
  }, [dateVal, hourVal, minVal, setValue])

  function toggleDay(iso: number) {
    setRecDays((prev) =>
      prev.includes(iso) ? prev.filter((d) => d !== iso) : [...prev, iso]
    )
  }

  async function submit(values: LessonFormValues) {
    try {
      let recurrence: RecurrenceOptions | undefined
      if (recEnabled && !initial) {
        recurrence = {
          type:  recType,
          days:  recType === 'weekly_custom' ? recDays : undefined,
          n:     recType === 'every_n_weeks' ? recN : undefined,
          count: recCount !== '' ? recCount : undefined,
        }
      }
      await onSubmit(values, recurrence)
      onClose()
    } catch (err) {
      const e = err as ApiError
      toast.error(e.message ?? 'Ошибка сохранения')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{initial ? 'Редактировать урок' : 'Новый урок'}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit(submit)} className="space-y-4 pt-2">
          <div className="space-y-1.5">
            <Label>Дата и время</Label>
            <div className="flex gap-2">
              <Input
                type="date"
                className="flex-1"
                value={dateVal}
                onChange={(e) => setDateVal(e.target.value)}
              />
              <select
                value={hourVal}
                onChange={(e) => setHourVal(e.target.value)}
                className="h-9 rounded-md border border-input bg-transparent px-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                {HOURS.map((h) => (
                  <option key={h} value={h}>{pad(h)}</option>
                ))}
              </select>
              <select
                value={minVal}
                onChange={(e) => setMinVal(e.target.value)}
                className="h-9 rounded-md border border-input bg-transparent px-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                {MINUTES.map((m) => (
                  <option key={m} value={m}>{pad(m)}</option>
                ))}
              </select>
            </div>
            {/* hidden field keeps scheduled_at registered */}
            <input type="hidden" {...register('scheduled_at')} />
            {errors.scheduled_at && (
              <p className="text-xs text-destructive">{errors.scheduled_at.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="duration_minutes">Длительность (минут)</Label>
            <Input
              id="duration_minutes"
              type="number"
              min={1}
              {...register('duration_minutes', { valueAsNumber: true })}
            />
            {errors.duration_minutes && (
              <p className="text-xs text-destructive">{errors.duration_minutes.message}</p>
            )}
          </div>

          {initial && (
            <div className="space-y-1.5">
              <Label htmlFor="status">Статус</Label>
              <select
                id="status"
                {...register('status')}
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                {Object.entries(STATUS_LABELS).map(([v, label]) => (
                  <option key={v} value={v}>{label}</option>
                ))}
              </select>
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="notes">
              Заметки <span className="text-muted-foreground font-normal">(необязательно)</span>
            </Label>
            <Input id="notes" placeholder="Тема урока..." {...register('notes')} />
          </div>

          {/* Recurrence — only when creating */}
          {!initial && (
            <div className="border rounded-md p-3 space-y-3">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={recEnabled}
                  onChange={(e) => setRecEnabled(e.target.checked)}
                  className="rounded"
                />
                <span className="text-sm font-medium">Повторять</span>
              </label>

              {recEnabled && (
                <>
                  <div className="space-y-1.5">
                    <Label>Тип повторения</Label>
                    <select
                      value={recType}
                      onChange={(e) => setRecType(e.target.value as RecurrenceType)}
                      className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                    >
                      <option value="weekly_same">Каждую неделю в этот день</option>
                      <option value="weekly_custom">Каждую неделю по выбранным дням</option>
                      <option value="every_n_weeks">Каждые N недель</option>
                    </select>
                  </div>

                  {recType === 'weekly_custom' && (
                    <div className="space-y-1.5">
                      <Label>Дни недели</Label>
                      <div className="flex gap-1">
                        {WEEK_DAYS.map(({ label, iso }) => (
                          <button
                            key={iso}
                            type="button"
                            onClick={() => toggleDay(iso)}
                            className={`h-8 w-9 rounded text-xs font-medium transition-colors ${
                              recDays.includes(iso)
                                ? 'bg-primary text-primary-foreground'
                                : 'border border-input hover:bg-muted'
                            }`}
                          >
                            {label}
                          </button>
                        ))}
                      </div>
                    </div>
                  )}

                  {recType === 'every_n_weeks' && (
                    <div className="space-y-1.5">
                      <Label>Интервал (недель)</Label>
                      <Input
                        type="number"
                        min={2}
                        max={12}
                        value={recN}
                        onChange={(e) => setRecN(Number(e.target.value))}
                      />
                    </div>
                  )}

                  <div className="space-y-1.5">
                    <Label>
                      Количество уроков{' '}
                      <span className="text-muted-foreground font-normal">(необязательно)</span>
                    </Label>
                    <Input
                      type="number"
                      min={2}
                      max={200}
                      placeholder={courseEndAt ? 'До конца курса' : 'Например, 20'}
                      value={recCount}
                      onChange={(e) => setRecCount(e.target.value === '' ? '' : Number(e.target.value))}
                    />
                    <p className="text-xs text-muted-foreground">
                      {courseEndAt
                        ? 'Оставьте пустым — уроки создадутся до окончания курса'
                        : 'Оставьте пустым — создастся на 1 год вперёд'}
                    </p>
                  </div>
                </>
              )}
            </div>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="outline" onClick={onClose}>Отмена</Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting
                ? 'Создание...'
                : recEnabled && !initial
                  ? recCount !== '' ? `Создать ${recCount} уроков` : 'Создать уроки'
                  : 'Сохранить'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
