import { z } from 'zod'

export const tutorProfileSchema = z.object({
  first_name: z.string().min(2, 'Минимум 2 символа'),
  last_name:  z.string().min(2, 'Минимум 2 символа'),
  email:      z.string().email('Неверный email'),
  phone:      z.string().min(10, 'Минимум 10 символов').optional().or(z.literal('')),
})

export type TutorProfileValues = z.infer<typeof tutorProfileSchema>

export const changePasswordSchema = z.object({
  current_password: z.string().min(6, 'Минимум 6 символов'),
  new_password:     z.string().min(6, 'Минимум 6 символов'),
  confirm_password: z.string().min(6, 'Минимум 6 символов'),
}).refine((d) => d.new_password === d.confirm_password, {
  message: 'Пароли не совпадают',
  path:    ['confirm_password'],
})

export type ChangePasswordValues = z.infer<typeof changePasswordSchema>
