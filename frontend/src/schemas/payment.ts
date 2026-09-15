import { z } from 'zod'

export const paymentSchema = z.object({
  amount:        z.number({ error: 'Введите сумму' }).positive('Должно быть больше 0'),
  lessons_count: z.number({ error: 'Введите число уроков' }).int().positive('Должно быть больше 0'),
  paid_at:       z.string().min(1, 'Выберите дату'),
  // Адресат: форма группы спрашивает его явно, у индивидуального курса он
  // известен и не вводится — поэтому не обязателен в схеме (спека 2026-09-06, п. 6.2).
  student_id:    z.string().optional(),
})

export type PaymentFormValues = z.infer<typeof paymentSchema>
