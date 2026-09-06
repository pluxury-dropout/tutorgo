import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { studentsApi, StudentInput, StudentListParams } from '@/lib/api/students'
import { courseKeys } from '@/lib/hooks/useCourses'
import { OnboardingStudentInput } from '@/types/api'

export const studentKeys = {
  all:    ['students'] as const,
  detail: (id: string) => ['students', id] as const,
}

export function useStudents() {
  return useQuery({
    queryKey: studentKeys.all,
    queryFn:  studentsApi.list,
  })
}

export function useStudent(id: string) {
  return useQuery({
    queryKey: studentKeys.detail(id),
    queryFn:  () => studentsApi.get(id),
  })
}

export function useCreateStudent() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: studentsApi.create,
    onSuccess:  () => qc.invalidateQueries({ queryKey: studentKeys.all }),
  })
}

export function useUpdateStudent(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: StudentInput) => studentsApi.update(id, data),
    onSuccess:  (updated) => {
      qc.invalidateQueries({ queryKey: studentKeys.all })
      qc.setQueryData(studentKeys.detail(id), updated)
    },
  })
}

// Быстрый онбординг заводит ученика, курс и серию уроков одним сабмитом —
// инвалидируем все три дерева, иначе созданное не появится без перезагрузки.
export function useOnboardStudent() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: OnboardingStudentInput) => studentsApi.onboard(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: studentKeys.all })
      qc.invalidateQueries({ queryKey: courseKeys.all })
      qc.invalidateQueries({ queryKey: ['calendar'] })
    },
  })
}

export function useDeleteStudent() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: studentsApi.delete,
    // Ученик уходит вместе с курсами и уроками (ON DELETE CASCADE) — без этих
    // двух сбросов его уроки висели в календаре до протухания кэша.
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: studentKeys.all })
      qc.invalidateQueries({ queryKey: courseKeys.all })
      qc.invalidateQueries({ queryKey: ['calendar'] })
    },
  })
}

export function useStudentsPaged(params: StudentListParams) {
  return useQuery({
    queryKey: [...studentKeys.all, 'list', params],
    queryFn:  () => studentsApi.listPaged(params),
  })
}

export function useStudentCount() {
  return useQuery({
    queryKey: [...studentKeys.all, 'count'],
    queryFn:  () => studentsApi.listPaged({ page: 1, limit: 1, search: '' }).then((r) => r.total),
  })
}
