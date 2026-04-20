import { z } from 'zod'

export const studentSchema = z.object({
  first_name: z.string().min(2, 'Минимум 2 символа'),
  last_name:  z.string().min(2, 'Минимум 2 символа'),
  email:      z.string().email('Неверный email'),
  phone:      z.string().min(10, 'Минимум 10 символов').optional().or(z.literal('')),
})

export type StudentFormValues = z.infer<typeof studentSchema>
