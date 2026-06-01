import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { paymentsApi, PaymentListParams, PaymentUpdateInput } from '@/lib/api/payments'
import { courseKeys } from '@/lib/hooks/useCourses'

export const paymentKeys = {
  byCourse:        (courseId: string) => ['payments', 'course', courseId] as const,
  paged:           (p: PaymentListParams) => ['payments', 'list', p] as const,
  recent:          ['payments', 'recent'] as const,
  monthlyIncome:   ['payments', 'monthly-income'] as const,
  monthlyExpected: ['payments', 'monthly-expected'] as const,
}

export function useMonthlyIncome() {
  return useQuery({
    queryKey: paymentKeys.monthlyIncome,
    queryFn:  paymentsApi.monthlyIncome,
  })
}

export function useMonthlyExpected() {
  return useQuery({
    queryKey: paymentKeys.monthlyExpected,
    queryFn:  paymentsApi.monthlyExpected,
  })
}

export function useRecentPayments() {
  return useQuery({
    queryKey: paymentKeys.recent,
    queryFn:  paymentsApi.listRecent,
  })
}

export function usePayments(courseId: string) {
  return useQuery({
    queryKey: paymentKeys.byCourse(courseId),
    queryFn:  () => paymentsApi.list(courseId),
    enabled:  !!courseId,
  })
}

export function useCreatePayment(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: paymentsApi.create,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: paymentKeys.byCourse(courseId) })
      qc.invalidateQueries({ queryKey: courseKeys.balance(courseId) })
      qc.invalidateQueries({ queryKey: ['payments', 'list'] })
      qc.invalidateQueries({ queryKey: paymentKeys.recent })
      qc.invalidateQueries({ queryKey: paymentKeys.monthlyIncome })
    },
  })
}

export function useUpdatePayment(courseId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: PaymentUpdateInput }) =>
      paymentsApi.update(id, data),
    onSuccess: () => {
      if (courseId) {
        qc.invalidateQueries({ queryKey: paymentKeys.byCourse(courseId) })
        qc.invalidateQueries({ queryKey: courseKeys.balance(courseId) })
      }
      qc.invalidateQueries({ queryKey: ['payments', 'list'] })
      qc.invalidateQueries({ queryKey: paymentKeys.recent })
      qc.invalidateQueries({ queryKey: paymentKeys.monthlyIncome })
    },
  })
}

export function useDeletePayment(courseId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: paymentsApi.delete,
    onSuccess: () => {
      if (courseId) {
        qc.invalidateQueries({ queryKey: paymentKeys.byCourse(courseId) })
        qc.invalidateQueries({ queryKey: courseKeys.balance(courseId) })
      }
      qc.invalidateQueries({ queryKey: ['payments', 'list'] })
      qc.invalidateQueries({ queryKey: paymentKeys.recent })
      qc.invalidateQueries({ queryKey: paymentKeys.monthlyIncome })
    },
  })
}

export function usePaymentsPaged(params: PaymentListParams) {
  return useQuery({
    queryKey: paymentKeys.paged(params),
    queryFn:  () => paymentsApi.listPaged(params),
  })
}
