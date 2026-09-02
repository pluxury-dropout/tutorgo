import { z } from 'zod'

// type больше не вопрос к пользователю: его задаёт точка входа — «Добавить курс»
// или «Создать группу». Поле осталось только как признак внутри формы.
export const courseSchema = z
  .object({
    type:              z.enum(['individual', 'group']),
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
