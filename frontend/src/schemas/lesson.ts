import { z } from 'zod'

export const lessonSchema = z.object({
  scheduled_at:     z.string().min(1, 'Выберите дату и время'),
  duration_minutes: z
    .number({ error: 'Введите число' })
    .int()
    .positive('Должно быть больше 0'),
  status: z.enum(['scheduled', 'completed', 'cancelled', 'missed']),
  notes:  z.string().optional(),
})

export type LessonFormValues = z.infer<typeof lessonSchema>
