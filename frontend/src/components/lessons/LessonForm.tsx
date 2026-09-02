'use client'

import { useEffect, useState } from 'react'
import { useForm, useWatch, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'

import { lessonSchema, LessonFormValues } from '@/schemas/lesson'
import { Lesson, ApiError } from '@/types/api'
import { STATUS_LABELS } from '@/lib/lessonStatus'
import { generateDates, lessonsPlural, WEEK_DAYS, RecurrenceOptions, RecurrenceType } from '@/lib/recurrence'
import { useConflicts } from '@/lib/hooks/useEvents'
import { formatTimeRange } from '@/lib/eventKind'
import { AlertTriangle } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { TimePicker } from '@/components/ui/time-picker'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const REC_TYPE_LABELS: Record<RecurrenceType, string> = {
  weekly_same:   'Каждую неделю в этот день',
  weekly_custom: 'Каждую неделю по выбранным дням',
  every_n_weeks: 'Каждые N недель',
}

interface LessonFormProps {
  open:         boolean
  onClose:      () => void
  onSubmit:     (data: LessonFormValues, recurrence?: RecurrenceOptions) => Promise<void>
  initial?:     Lesson
  courseEndAt?: string   // ISO date — upper bound for open-ended recurrence
}


export function LessonForm({ open, onClose, onSubmit, initial, courseEndAt }: LessonFormProps) {
  const {
    register,
    handleSubmit,
    reset,
    setValue,
    control,
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
        scheduled_at:     `${d}T${String(h).padStart(2,'0')}:${String(m).padStart(2,'0')}`,
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

  const timeStr = `${String(Number(hourVal)).padStart(2,'0')}:${String(Number(minVal)).padStart(2,'0')}`

  useEffect(() => {
    if (dateVal) setValue('scheduled_at', `${dateVal}T${timeStr}`)
  }, [dateVal, timeStr, setValue])

  const recurrence: RecurrenceOptions | undefined = recEnabled && !initial
    ? {
        type:  recType,
        days:  recType === 'weekly_custom' ? recDays : undefined,
        n:     recType === 'every_n_weeks' ? recN : undefined,
        count: recCount !== '' ? recCount : undefined,
      }
    : undefined

  // Превью считаем той же функцией, что потом раскатает серию, — иначе кнопка врёт.
  const daysMissing = recurrence?.type === 'weekly_custom' && recDays.length === 0
  const preview = recurrence && dateVal && !daysMissing
    ? generateDates(new Date(`${dateVal}T${timeStr}`).toISOString(), recurrence, courseEndAt)
    : []
  const lastDate = preview.length
    ? new Date(preview[preview.length - 1]).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short', year: 'numeric' })
    : ''

  // Занятость слота. Предупреждение не блокирует сохранение: наложение бывает
  // осознанным, а решает репетитор. При правке урок исключает сам себя.
  // useWatch, а не watch(): watch() возвращает функцию, из-за которой React
  // Compiler отключает мемоизацию всего компонента.
  const durationMinutes = useWatch({ control, name: 'duration_minutes' })
  const { data: conflicts = [] } = useConflicts(
    dateVal && durationMinutes > 0
      ? {
          starts_at:        new Date(`${dateVal}T${timeStr}`).toISOString(),
          duration_minutes: durationMinutes,
          ...(initial ? { exclude_type: 'lesson' as const, exclude_id: initial.id } : {}),
        }
      : null,
  )

  function toggleDay(iso: number) {
    setRecDays((prev) =>
      prev.includes(iso) ? prev.filter((d) => d !== iso) : [...prev, iso]
    )
  }

  async function submit(values: LessonFormValues) {
    try {
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
              <TimePicker
                hour={hourVal}
                minute={minVal}
                onHourChange={setHourVal}
                onMinuteChange={setMinVal}
              />
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

          {conflicts.length > 0 && (
            <p className="flex items-start gap-1.5 rounded-md bg-[var(--cal-missed-bg)] px-2 py-1.5 text-xs text-[var(--cal-missed-text)]">
              <AlertTriangle className="mt-px size-3.5 shrink-0" />
              <span>
                Пересекается с{' '}
                {conflicts.map((c) => `«${c.title}» ${formatTimeRange(c.starts_at, c.duration_minutes)}`).join(', ')}
              </span>
            </p>
          )}

          {initial && (
            <div className="space-y-1.5">
              <Label htmlFor="status">Статус</Label>
              <Controller
                name="status"
                control={control}
                render={({ field }) => (
                  <Select value={field.value ?? 'scheduled'} onValueChange={field.onChange}>
                    <SelectTrigger id="status" className="w-full">
                      <SelectValue>{STATUS_LABELS[field.value ?? 'scheduled']}</SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      {Object.entries(STATUS_LABELS).map(([v, label]) => (
                        <SelectItem key={v} value={v}>{label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
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
                    <Select value={recType} onValueChange={(v) => setRecType(v as RecurrenceType)}>
                      <SelectTrigger className="w-full">
                        <SelectValue>{REC_TYPE_LABELS[recType]}</SelectValue>
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="weekly_same" label="Каждую неделю в этот день">Каждую неделю в этот день</SelectItem>
                        <SelectItem value="weekly_custom" label="Каждую неделю по выбранным дням">Каждую неделю по выбранным дням</SelectItem>
                        <SelectItem value="every_n_weeks" label="Каждые N недель">Каждые N недель</SelectItem>
                      </SelectContent>
                    </Select>
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
                      {daysMissing && (
                        <p className="text-xs text-destructive">Выберите хотя бы один день недели</p>
                      )}
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
            <Button type="submit" disabled={isSubmitting || daysMissing}>
              {isSubmitting
                ? 'Создание...'
                : recurrence
                  ? preview.length
                    ? `Создать ${lessonsPlural(preview.length)} (до ${lastDate})`
                    : 'Создать уроки'
                  : 'Сохранить'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
