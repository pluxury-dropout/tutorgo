import { z } from 'zod'

export const loginSchema = z.object({
  credential: z
    .string()
    .min(1, 'Введите email или телефон'),
  password: z.string().min(6, 'Минимум 6 символов'),
})

export const registerSchema = z.object({
  first_name: z.string().min(2, 'Минимум 2 символа'),
  last_name:  z.string().min(2, 'Минимум 2 символа'),
  email:      z.string().email('Неверный email'),
  phone:      z.string().min(10, 'Минимум 10 символов').optional().or(z.literal('')),
  password:   z.string().min(6, 'Минимум 6 символов'),
})

export type LoginInput    = z.infer<typeof loginSchema>
export type RegisterInput = z.infer<typeof registerSchema>
