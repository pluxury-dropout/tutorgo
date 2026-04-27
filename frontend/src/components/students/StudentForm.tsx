'use client'

import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'

import { studentSchema, StudentFormValues } from '@/schemas/student'
import { Student, ApiError } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

interface StudentFormProps {
  open: boolean
  onClose: () => void
  onSubmit: (data: StudentFormValues) => Promise<void>
  initial?: Student
}

export function StudentForm({ open, onClose, onSubmit, initial }: StudentFormProps) {
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<StudentFormValues>({ resolver: zodResolver(studentSchema) })

  useEffect(() => {
    reset(initial
      ? { ...initial, last_name: initial.last_name ?? undefined, email: initial.email ?? undefined, phone: initial.phone ?? undefined }
      : { first_name: '', last_name: '', email: '', phone: '' })
  }, [initial, open, reset])

  async function submit(values: StudentFormValues) {
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
          <DialogTitle>{initial ? 'Редактировать ученика' : 'Новый ученик'}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit(submit)} className="space-y-4 pt-2">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="first_name">Имя</Label>
              <Input id="first_name" placeholder="Айгерим" {...register('first_name')} />
              {errors.first_name && (
                <p className="text-xs text-destructive">{errors.first_name.message}</p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="last_name">Фамилия <span className="text-muted-foreground font-normal">(необязательно)</span></Label>
              <Input id="last_name" placeholder="Бекова" {...register('last_name')} />
              {errors.last_name && (
                <p className="text-xs text-destructive">{errors.last_name.message}</p>
              )}
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="email">Email <span className="text-muted-foreground font-normal">(необязательно)</span></Label>
            <Input id="email" type="email" placeholder="student@example.com" {...register('email')} />
            {errors.email && (
              <p className="text-xs text-destructive">{errors.email.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="phone">
              Телефон <span className="text-muted-foreground font-normal">(необязательно)</span>
            </Label>
            <Input id="phone" type="tel" placeholder="+77001234567" {...register('phone')} />
            {errors.phone && (
              <p className="text-xs text-destructive">{errors.phone.message}</p>
            )}
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
