'use client'

import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'

import { paymentSchema, PaymentFormValues } from '@/schemas/payment'
import { ApiError } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

interface PaymentFormProps {
  open:             boolean
  onClose:          () => void
  onSubmit:         (data: PaymentFormValues) => Promise<void>
  pricePerLesson:   number
  lessonsPerCycle?: number
  initialValues?:   PaymentFormValues
  paymentId?:       string
}

export function PaymentForm({
  open,
  onClose,
  onSubmit,
  pricePerLesson,
  lessonsPerCycle = 0,
  initialValues,
  paymentId,
}: PaymentFormProps) {
  const isEdit = !!paymentId

  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
    formState: { errors, isSubmitting, dirtyFields },
  } = useForm<PaymentFormValues>({
    resolver: zodResolver(paymentSchema),
    defaultValues: { paid_at: '' },
  })

  const amount       = watch('amount')
  const lessonsCount = watch('lessons_count')

  useEffect(() => {
    if (open) {
      if (initialValues) {
        reset(initialValues)
      } else {
        // Пачка курса — типичное «оплатил как всегда», форма готова к сохранению сразу
        reset({
          paid_at:       new Date().toISOString().slice(0, 10),
          lessons_count: lessonsPerCycle > 0 ? lessonsPerCycle : undefined,
        })
      }
    }
  }, [open, reset, initialValues, lessonsPerCycle])

  // Односторонняя подстановка: уроки → сумма, пока сумму не тронули руками.
  // dirtyFields.amount — родной флаг RHF, взводится только ручным вводом в
  // поле (наши setValue ниже не помечают dirty), двусторонней связи нет.
  useEffect(() => {
    if (!isEdit && !dirtyFields.amount && pricePerLesson > 0 && lessonsCount > 0) {
      setValue('amount', lessonsCount * pricePerLesson, { shouldValidate: true })
    }
  }, [lessonsCount, pricePerLesson, setValue, isEdit, dirtyFields.amount])

  // Расхождение показываем, но не блокируем сохранение — скидки и округления законны
  const impliedPrice   = lessonsCount > 0 ? amount / lessonsCount : 0
  const priceMismatch  =
    pricePerLesson > 0 && lessonsCount > 0 && amount > 0 && Math.round(impliedPrice) !== pricePerLesson

  async function submit(values: PaymentFormValues) {
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
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Редактировать оплату' : 'Новая оплата'}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit(submit)} className="space-y-4 pt-2">
          <div className="space-y-1.5">
            <Label htmlFor="lessons_count">
              Уроков оплачено{' '}
              <span className="text-muted-foreground font-normal">
                (цена за урок: {pricePerLesson.toLocaleString()} ₸)
              </span>
            </Label>
            <Input
              id="lessons_count"
              type="number"
              min={1}
              {...register('lessons_count', { valueAsNumber: true })}
            />
            {errors.lessons_count && (
              <p className="text-xs text-destructive">{errors.lessons_count.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="amount">Сумма (₸)</Label>
            <Input
              id="amount"
              type="number"
              min={1}
              step="any"
              {...register('amount', { valueAsNumber: true })}
            />
            {errors.amount ? (
              <p className="text-xs text-destructive">{errors.amount.message}</p>
            ) : priceMismatch ? (
              <p className="text-xs text-muted-foreground">
                выходит {Math.round(impliedPrice).toLocaleString()} ₸ за урок вместо{' '}
                {pricePerLesson.toLocaleString()}
              </p>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="paid_at">Дата оплаты</Label>
            <Input id="paid_at" type="date" {...register('paid_at')} />
            {errors.paid_at && (
              <p className="text-xs text-destructive">{errors.paid_at.message}</p>
            )}
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="outline" onClick={onClose}>Отмена</Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
