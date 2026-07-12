import { z } from 'zod'

export const studentLoginSchema = z.object({
  identifier: z.string().min(1, 'Введите телефон или логин'),
  password: z.string().min(6, 'Минимум 6 символов'),
})

export const acceptInviteSchema = z
  .object({
    username: z
      .string()
      .min(3, 'Минимум 3 символа')
      .max(32, 'Максимум 32 символа')
      .regex(/^[a-zA-Z0-9]+$/, 'Только латинские буквы и цифры'),
    password: z.string().min(6, 'Минимум 6 символов'),
    confirm: z.string(),
  })
  .refine((d) => d.password === d.confirm, {
    path: ['confirm'],
    message: 'Пароли не совпадают',
  })

export type StudentLoginInput = z.infer<typeof studentLoginSchema>
export type AcceptInviteInput = z.infer<typeof acceptInviteSchema>
