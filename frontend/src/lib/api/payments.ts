import { api } from './client'
import { Payment, PaymentBalance, PagedResponse } from '@/types/api'

export interface PaymentInput {
  course_id: string
  amount: number
  lessons_count: number
  paid_at?: string
}

export type PaymentUpdateInput = Omit<PaymentInput, 'course_id'>

export interface PaymentListParams {
  page:  number
  limit: number
}

export const paymentsApi = {
  list: (courseId: string) =>
    api.get<PagedResponse<Payment>>('/payments', { params: { course_id: courseId } })
      .then((r) => r.data.data ?? []),
  listPaged: (p: PaymentListParams) =>
    api.get<PagedResponse<Payment>>('/payments', { params: p }).then((r) => r.data),
  listRecent: () =>
    api.get<Payment[]>('/payments/recent').then((r) => r.data ?? []),
  create: (data: PaymentInput) => {
    const payload = { ...data, paid_at: data.paid_at ? data.paid_at + 'T00:00:00Z' : undefined }
    return api.post<Payment>('/payments', payload).then((r) => r.data)
  },
  update: (id: string, data: PaymentUpdateInput) => {
    const payload = { ...data, paid_at: data.paid_at ? data.paid_at + 'T00:00:00Z' : undefined }
    return api.put<Payment>(`/payments/${id}`, payload).then((r) => r.data)
  },
  delete: (id: string) =>
    api.delete(`/payments/${id}`).then(() => id),
  getBalance: (courseId: string) =>
    api.get<PaymentBalance>('/payments/balance', { params: { course_id: courseId } })
      .then((r) => r.data),
  monthlyIncome: () =>
    api.get<{ total: number }>('/payments/monthly-income').then((r) => r.data.total),
  monthlyExpected: () =>
    api.get<{ total: number }>('/payments/monthly-expected').then((r) => r.data.total),
}
