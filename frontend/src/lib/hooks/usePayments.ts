import { useQuery, useMutation, useQueryClient, QueryClient } from '@tanstack/react-query'
import { paymentsApi, PaymentListParams, PaymentUpdateInput } from '@/lib/api/payments'
import { courseKeys } from '@/lib/hooks/useCourses'
import { studentKeys } from '@/lib/hooks/useStudents'

export const paymentKeys = {
  byCourse:        (courseId: string) => ['payments', 'course', courseId] as const,
  paged:           (p: PaymentListParams) => ['payments', 'list', p] as const,
  recent:          ['payments', 'recent'] as const,
  monthlyIncome:   ['payments', 'monthly-income'] as const,
  monthlyExpected: ['payments', 'monthly-expected'] as const,
  debts:           ['payments', 'debts'] as const,
}

// Любой платёж меняет всё денежное разом: историю, доход, прогноз и долги.
// Все эти ключи начинаются с ['payments'] — сбрасываем префиксом, плюс балансы
// затронутых курсов (у них свой корень ['courses']) и обзор затронутых учеников.
function invalidateMoney(qc: QueryClient, courseIds: string[], studentIds: string[] = []) {
  qc.invalidateQueries({ queryKey: ['payments'] })
  for (const id of courseIds) qc.invalidateQueries({ queryKey: courseKeys.balance(id) })
  for (const id of studentIds) qc.invalidateQueries({ queryKey: studentKeys.overview(id) })
}

export function useMonthlyIncome() {
  return useQuery({ queryKey: paymentKeys.monthlyIncome, queryFn: paymentsApi.monthlyIncome })
}

export function useMonthlyExpected() {
  return useQuery({ queryKey: paymentKeys.monthlyExpected, queryFn: paymentsApi.monthlyExpected })
}

export function useRecentPayments() {
  return useQuery({ queryKey: paymentKeys.recent, queryFn: paymentsApi.listRecent })
}

export function useDebts() {
  return useQuery({ queryKey: paymentKeys.debts, queryFn: paymentsApi.debts })
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
    onSuccess:  (_data, input) => invalidateMoney(qc, [courseId], [input.student_id]),
  })
}

export function useCreateBulkPayment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: paymentsApi.createBulk,
    onSuccess:  (_data, input) => invalidateMoney(
      qc,
      input.items.map((i) => i.course_id),
      [...new Set(input.items.map((i) => i.student_id))],
    ),
  })
}

export function useUpdatePayment(courseId?: string, studentId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: PaymentUpdateInput }) =>
      paymentsApi.update(id, data),
    onSuccess: () => invalidateMoney(qc, courseId ? [courseId] : [], studentId ? [studentId] : []),
  })
}

export function useDeletePayment(courseId?: string, studentId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: paymentsApi.delete,
    onSuccess:  () => invalidateMoney(qc, courseId ? [courseId] : [], studentId ? [studentId] : []),
  })
}

export function usePaymentsPaged(params: PaymentListParams) {
  return useQuery({
    queryKey: paymentKeys.paged(params),
    queryFn:  () => paymentsApi.listPaged(params),
  })
}
