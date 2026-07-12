'use client'

import { useEffect, useState } from 'react'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Trash2 } from 'lucide-react'

import { lessonSchema, LessonFormValues } from '@/schemas/lesson'
import { Lesson, ApiError } from '@/types/api'
import { STATUS_LABELS } from '@/lib/lessonStatus'
import { lessonsApi } from '@/lib/api/lessons'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { TimePicker } from '@/components/ui/time-picker'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

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

const REC_TYPE_LABELS: Record<RecurrenceType, string> = {
  weekly_same:   'Каждую неделю в этот день',
  weekly_custom: 'Каждую неделю по выбранным дням',
  every_n_weeks: 'Каждые N недель',
}

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

  const [newTask, setNewTask] = useState('')

  const lessonId = initial?.id ?? ''
  const queryClient = useQueryClient()

  const tasksQuery = useQuery({
    queryKey: ['lesson-tasks', lessonId],
    queryFn:  () => lessonsApi.tasks(lessonId),
    enabled:  !!lessonId,
  })

  const createTaskMut = useMutation({
    mutationFn: (title: string) => lessonsApi.createTask(lessonId, { title }),
    onSuccess: () => {
      setNewTask('')
      queryClient.invalidateQueries({ queryKey: ['lesson-tasks', lessonId] })
    },
    onError: (err) => toast.error((err as unknown as ApiError).message ?? 'Ошибка добавления задачи'),
  })

  const deleteTaskMut = useMutation({
    mutationFn: (taskId: string) => lessonsApi.deleteTask(taskId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['lesson-tasks', lessonId] }),
    onError: (err) => toast.error((err as unknown as ApiError).message ?? 'Ошибка удаления задачи'),
  })

  function addTask() {
    const title = newTask.trim()
    if (!title) return
    createTaskMut.mutate(title)
  }

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
    setNewTask('')
  }, [initial, open, reset])

  useEffect(() => {
    if (dateVal) setValue('scheduled_at', `${dateVal}T${String(Number(hourVal)).padStart(2,'0')}:${String(Number(minVal)).padStart(2,'0')}`)
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

          {/* Tasks — only when the lesson already exists */}
          {initial && (
            <div className="space-y-2">
              <Label>Задачи</Label>
              {tasksQuery.data && tasksQuery.data.length > 0 && (
                <ul className="space-y-1.5">
                  {tasksQuery.data.map((task) => (
                    <li
                      key={task.id}
                      className="flex items-center justify-between gap-2 rounded-md border border-input px-3 py-2 text-sm"
                    >
                      <span className="truncate">{task.title}</span>
                      <button
                        type="button"
                        onClick={() => deleteTaskMut.mutate(task.id)}
                        disabled={deleteTaskMut.isPending}
                        className="text-muted-foreground hover:text-destructive shrink-0"
                        aria-label="Удалить задачу"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </li>
                  ))}
                </ul>
              )}
              {tasksQuery.data && tasksQuery.data.length === 0 && (
                <p className="text-xs text-muted-foreground">Пока нет задач</p>
              )}
              <div className="flex gap-2">
                <Input
                  className="flex-1"
                  placeholder="Добавить задачу..."
                  value={newTask}
                  onChange={(e) => setNewTask(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      addTask()
                    }
                  }}
                />
                <Button
                  type="button"
                  variant="outline"
                  onClick={addTask}
                  disabled={createTaskMut.isPending || !newTask.trim()}
                >
                  Добавить
                </Button>
              </div>
            </div>
          )}

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
