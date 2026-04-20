import { api } from './client'
import { Payment, PaymentBalance } from '@/types/api'

export interface PaymentInput {
  course_id: string
  amount: number
  lessons_count: number
  paid_at?: string
}

export const paymentsApi = {
  list: () => api.get<Payment[]>('/payments').then((r) => r.data),
  create: (data: PaymentInput) =>
    api.post<Payment>('/payments', data).then((r) => r.data),
  getBalance: () =>
    api.get<PaymentBalance>('/payments/balance').then((r) => r.data),
}
