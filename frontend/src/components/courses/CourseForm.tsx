'use client'

import { useEffect, useState } from 'react'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'
import { X } from 'lucide-react'

import { courseSchema, CourseFormValues } from '@/schemas/course'
import { Course, Student, ApiError } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { StudentCombobox, studentName } from '@/components/students/StudentCombobox'
import { SubjectCombobox } from '@/components/courses/SubjectCombobox'

interface CourseFormProps {
  open: boolean
  onClose: () => void
  onSubmit: (data: CourseFormValues) => Promise<void>
  initial?: Course
}

export function CourseForm({ open, onClose, onSubmit, initial }: CourseFormProps) {
  const [picked, setPicked] = useState<Student[]>([])

  const {
    register,
    handleSubmit,
    reset,
    watch,
    control,
    formState: { errors, isSubmitting },
  } = useForm<CourseFormValues>({
    resolver: zodResolver(courseSchema),
    defaultValues: { subject: '', lessons_per_cycle: 1, started_at: '', ended_at: '' },
  })

  const pricePerCycle   = watch('price_per_cycle')
  const lessonsPerCycle = watch('lessons_per_cycle')
  const pricePerLesson  = lessonsPerCycle > 0 ? pricePerCycle / lessonsPerCycle : 0

  useEffect(() => {
    setPicked([])
    if (initial) {
      reset({
        subject:           initial.subject,
        price_per_cycle:   initial.price_per_cycle,
        lessons_per_cycle: initial.lessons_per_cycle,
        started_at:        initial.started_at.slice(0, 10),
        ended_at:          initial.ended_at?.slice(0, 10) ?? '',
      })
    } else {
      reset({ subject: '', lessons_per_cycle: 1, started_at: '', ended_at: '' })
    }
  }, [initial, open, reset])

  function addStudent(student: Student | null) {
    if (!student || picked.some((s) => s.id === student.id)) return
    setPicked([...picked, student])
  }

  async function submit(values: CourseFormValues) {
    try {
      await onSubmit({ ...values, student_ids: picked.map((s) => s.id) })
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
          <DialogTitle>{initial ? 'Редактировать курс' : 'Новая группа'}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit(submit)} className="space-y-4 pt-2">
          {!initial && (
            <div className="space-y-1.5">
              <Label>Ученики</Label>
              <StudentCombobox value={null} onChange={addStudent} placeholder="Добавить ученика" />
              {picked.length > 0 && (
                <div className="flex flex-wrap gap-1.5 pt-1">
                  {picked.map((s) => (
                    <button
                      key={s.id}
                      type="button"
                      onClick={() => setPicked(picked.filter((p) => p.id !== s.id))}
                      className="flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs hover:bg-muted/70"
                    >
                      {studentName(s)}
                      <X className="size-3" />
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          <div className="space-y-1.5">
            <Label>Предмет</Label>
            <Controller
              name="subject"
              control={control}
              render={({ field }) => (
                <SubjectCombobox value={field.value ?? ''} onChange={field.onChange} />
              )}
            />
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
