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
  open:          boolean
  onClose:       () => void
  onSubmit:      (data: PaymentFormValues) => Promise<void>
  pricePerLesson: number
}

export function PaymentForm({ open, onClose, onSubmit, pricePerLesson }: PaymentFormProps) {
  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<PaymentFormValues>({
    resolver: zodResolver(paymentSchema),
    defaultValues: { amount: 0, lessons_count: 0, paid_at: '' },
  })

  const amount = watch('amount')

  useEffect(() => {
    if (open) reset({ amount: 0, lessons_count: 0, paid_at: new Date().toISOString().slice(0, 10) })
  }, [open, reset])

  useEffect(() => {
    if (pricePerLesson > 0 && amount > 0) {
      setValue('lessons_count', Math.floor(amount / pricePerLesson))
    }
  }, [amount, pricePerLesson, setValue])

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
          <DialogTitle>Новая оплата</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit(submit)} className="space-y-4 pt-2">
          <div className="space-y-1.5">
            <Label htmlFor="amount">Сумма (₸)</Label>
            <Input
              id="amount"
              type="number"
              min={1}
              step="any"
              {...register('amount', { valueAsNumber: true })}
            />
            {errors.amount && (
              <p className="text-xs text-destructive">{errors.amount.message}</p>
            )}
          </div>

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
