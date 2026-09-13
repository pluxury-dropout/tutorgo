import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { studentsApi, StudentInput, StudentListParams } from '@/lib/api/students'
import { courseKeys } from '@/lib/hooks/useCourses'
import { ApiError, OnboardingStudentInput, Student } from '@/types/api'

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
    // Удаляется только ученик без истории (иначе 409, см. useRemoveStudent) —
    // вместе с курсами и будущими уроками; без этих сбросов они висели бы в
    // календаре до протухания кэша.
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: studentKeys.all })
      qc.invalidateQueries({ queryKey: courseKeys.all })
      qc.invalidateQueries({ queryKey: ['calendar'] })
    },
  })
}

export function useArchiveStudent() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: studentsApi.archive,
    // Архивация уводит в архив курсы ученика и удаляет их будущие уроки.
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: studentKeys.all })
      qc.invalidateQueries({ queryKey: courseKeys.all })
      qc.invalidateQueries({ queryKey: ['calendar'] })
    },
  })
}

export function useRestoreStudent() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: studentsApi.restore,
    onSuccess:  () => qc.invalidateQueries({ queryKey: studentKeys.all }),
  })
}

/** «Удалить» из интерфейса. Ученик без истории удаляется; с платежами или
 *  проведёнными уроками сервер отвечает 409, и тогда предлагаем архив — удаление
 *  стёрло бы их из истории (спека, п. 5a.2). Одна функция на список и карточку,
 *  чтобы тексты диалогов не разъехались. Любая другая ошибка — toast, возврат null.
 */
export function useRemoveStudent() {
  const del     = useDeleteStudent()
  const archive = useArchiveStudent()

  return async (s: Student): Promise<'deleted' | 'archived' | null> => {
    const name = `${s.first_name}${s.last_name ? ` ${s.last_name}` : ''}`
    if (!confirm(`Удалить ${name}?`)) return null
    try {
      await del.mutateAsync(s.id)
      return 'deleted'
    } catch (e) {
      if ((e as ApiError).status !== 409) {
        toast.error('Не удалось удалить ученика')
        return null
      }
    }
    if (!confirm(`${name}: есть платежи или проведённые уроки — удаление стёрло бы их из истории. Перенести в архив?`)) {
      return null
    }
    try {
      await archive.mutateAsync(s.id)
      return 'archived'
    } catch {
      toast.error('Не удалось перенести ученика в архив')
      return null
    }
  }
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
