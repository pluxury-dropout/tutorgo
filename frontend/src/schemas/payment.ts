import { z } from 'zod'

export const paymentSchema = z.object({
  amount:        z.number({ error: 'Введите сумму' }).positive('Должно быть больше 0'),
  lessons_count: z.number({ error: 'Введите число уроков' }).int().positive('Должно быть больше 0'),
  paid_at:       z.string().min(1, 'Выберите дату'),
})

export type PaymentFormValues = z.infer<typeof paymentSchema>
