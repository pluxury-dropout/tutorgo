import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { paymentsApi } from '@/lib/api/payments'
import { courseKeys } from '@/lib/hooks/useCourses'

export const paymentKeys = {
  byCourse:      (courseId: string) => ['payments', 'course', courseId] as const,
  recent:        ['payments', 'recent'] as const,
  monthlyIncome: ['payments', 'monthly-income'] as const,
}

export function useMonthlyIncome() {
  return useQuery({
    queryKey: paymentKeys.monthlyIncome,
    queryFn:  paymentsApi.monthlyIncome,
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
    },
  })
}
