import { z } from 'zod'

export const courseSchema = z.object({
  student_id:        z.string().optional(),
  student_ids:       z.array(z.string()).optional(),
  subject:           z.string().min(2, 'Минимум 2 символа'),
  price_per_cycle:   z
    .number({ error: 'Введите число' })
    .min(0, 'Не может быть отрицательной'),
  lessons_per_cycle: z
    .number({ error: 'Введите число' })
    .int('Только целое число')
    .min(1, 'Минимум 1 урок'),
  started_at: z.string().min(1, 'Выберите дату начала'),
  ended_at:   z.string().optional(),
})

export type CourseFormValues = z.infer<typeof courseSchema>
