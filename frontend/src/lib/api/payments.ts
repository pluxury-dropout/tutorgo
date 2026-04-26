import { api } from './client'
import { Payment, PaymentBalance } from '@/types/api'

export interface PaymentInput {
  course_id: string
  amount: number
  lessons_count: number
  paid_at?: string
}

export const paymentsApi = {
  list: (courseId: string) =>
    api.get<Payment[]>('/payments', { params: { course_id: courseId } }).then((r) => r.data ?? []),
  listRecent: () =>
    api.get<Payment[]>('/payments/recent').then((r) => r.data ?? []),
  create: (data: PaymentInput) =>
    api.post<Payment>('/payments', data).then((r) => r.data),
  getBalance: (courseId: string) =>
    api.get<PaymentBalance>('/payments/balance', { params: { course_id: courseId } }).then((r) => r.data),
  monthlyIncome: () =>
    api.get<{ total: number }>('/payments/monthly-income').then((r) => r.data.total),
}
