import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { studentsApi, StudentInput } from '@/lib/api/students'

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

export function useDeleteStudent() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: studentsApi.delete,
    onSuccess:  () => qc.invalidateQueries({ queryKey: studentKeys.all }),
  })
}
