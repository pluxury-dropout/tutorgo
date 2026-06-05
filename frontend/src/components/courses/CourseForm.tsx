'use client'

import { useEffect } from 'react'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'

import { courseSchema, CourseFormValues } from '@/schemas/course'
import { Course, ApiError } from '@/types/api'
import { useStudents } from '@/lib/hooks/useStudents'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

interface CourseFormProps {
  open: boolean
  onClose: () => void
  onSubmit: (data: CourseFormValues) => Promise<void>
  initial?: Course
}

export function CourseForm({ open, onClose, onSubmit, initial }: CourseFormProps) {
  const { data: students = [] } = useStudents()

  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
    control,
    formState: { errors, isSubmitting },
  } = useForm<CourseFormValues>({
    resolver: zodResolver(courseSchema),
    defaultValues: { type: 'individual', subject: '', price_per_cycle: 0, lessons_per_cycle: 1, started_at: '', ended_at: '' },
  })

  const courseType      = watch('type')
  const pricePerCycle   = watch('price_per_cycle')
  const lessonsPerCycle = watch('lessons_per_cycle')
  const pricePerLesson  = lessonsPerCycle > 0 ? pricePerCycle / lessonsPerCycle : 0

  useEffect(() => {
    if (initial) {
      reset({
        type:              initial.student_id ? 'individual' : 'group',
        student_id:        initial.student_id ?? undefined,
        subject:           initial.subject,
        price_per_cycle:   initial.price_per_cycle,
        lessons_per_cycle: initial.lessons_per_cycle,
        started_at:        initial.started_at.slice(0, 10),
        ended_at:          initial.ended_at?.slice(0, 10) ?? '',
      })
    } else {
      reset({ type: 'individual', subject: '', price_per_cycle: 0, lessons_per_cycle: 1, started_at: '', ended_at: '' })
    }
  }, [initial, open, reset])

  async function submit(values: CourseFormValues) {
    try {
      await onSubmit(values)
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
          <DialogTitle>{initial ? 'Редактировать курс' : 'Новый курс'}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit(submit)} className="space-y-4 pt-2">
          {!initial && (
            <div className="space-y-1.5">
              <Label>Тип курса</Label>
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant={courseType === 'individual' ? 'default' : 'outline'}
                  onClick={() => setValue('type', 'individual')}
                >
                  Индивидуальный
                </Button>
                <Button
                  type="button"
                  variant={courseType === 'group' ? 'default' : 'outline'}
                  onClick={() => {
                    setValue('type', 'group')
                    setValue('student_id', undefined)
                  }}
                >
                  Групповой
                </Button>
              </div>
            </div>
          )}

          {courseType === 'individual' && !initial && (
            <div className="space-y-1.5">
              <Label htmlFor="student_id">Ученик</Label>
              <Controller
                name="student_id"
                control={control}
                render={({ field }) => (
                  <Select value={field.value ?? ''} onValueChange={field.onChange}>
                    <SelectTrigger id="student_id">
                      <SelectValue placeholder="Выберите ученика" />
                    </SelectTrigger>
                    <SelectContent>
                      {students.map((s) => {
                        const name = s.last_name ? `${s.first_name} ${s.last_name}` : s.first_name
                        return (
                          <SelectItem key={s.id} value={s.id} label={name}>
                            {name}
                          </SelectItem>
                        )
                      })}
                    </SelectContent>
                  </Select>
                )}
              />
              {errors.student_id && (
                <p className="text-xs text-destructive">{errors.student_id.message}</p>
              )}
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="subject">Предмет</Label>
            <Input id="subject" placeholder="Математика" {...register('subject')} />
            {errors.subject && (
              <p className="text-xs text-destructive">{errors.subject.message}</p>
            )}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="price_per_cycle">Цена за цикл (₸)</Label>
              <Input
                id="price_per_cycle"
                type="number"
                min={1}
                step="any"
                {...register('price_per_cycle', { valueAsNumber: true })}
              />
              {errors.price_per_cycle && (
                <p className="text-xs text-destructive">{errors.price_per_cycle.message}</p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="lessons_per_cycle">Уроков в цикле</Label>
              <Input
                id="lessons_per_cycle"
                type="number"
                min={1}
                step={1}
                {...register('lessons_per_cycle', { valueAsNumber: true })}
              />
              {errors.lessons_per_cycle && (
                <p className="text-xs text-destructive">{errors.lessons_per_cycle.message}</p>
              )}
            </div>
          </div>
          {pricePerLesson > 0 && (
            <p className="text-xs text-muted-foreground">
              = {Math.round(pricePerLesson).toLocaleString()} ₸ за урок
            </p>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="started_at">Дата начала</Label>
              <Input id="started_at" type="date" {...register('started_at')} />
              {errors.started_at && (
                <p className="text-xs text-destructive">{errors.started_at.message}</p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="ended_at">
                Дата окончания{' '}
                <span className="text-muted-foreground font-normal">(необязательно)</span>
              </Label>
              <Input id="ended_at" type="date" {...register('ended_at')} />
            </div>
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="outline" onClick={onClose}>
              Отмена
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
