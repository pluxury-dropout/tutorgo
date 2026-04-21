import { z } from 'zod'

export const courseSchema = z
  .object({
    type:             z.enum(['individual', 'group']),
    student_id:       z.string().optional(),
    subject:          z.string().min(2, 'Минимум 2 символа'),
    price_per_lesson: z
      .number({ invalid_type_error: 'Введите число' })
      .positive('Должно быть больше 0'),
    started_at: z.string().min(1, 'Выберите дату начала'),
    ended_at:   z.string().optional(),
  })
  .superRefine((data, ctx) => {
    if (data.type === 'individual' && !data.student_id) {
      ctx.addIssue({
        code:    z.ZodIssueCode.custom,
        message: 'Выберите ученика',
        path:    ['student_id'],
      })
    }
  })

export type CourseFormValues = z.infer<typeof courseSchema>
