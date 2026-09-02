import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { coursesApi, CourseInput, CourseListParams } from '@/lib/api/courses'

export const courseKeys = {
  all:         ['courses'] as const,
  detail:      (id: string) => ['courses', id] as const,
  balance:     (id: string) => ['courses', id, 'balance'] as const,
  enrollments: (id: string) => ['courses', id, 'enrollments'] as const,
  byStudent:   (studentId: string) => ['courses', 'student', studentId] as const,
  archived:    ['courses', 'archived'] as const,
  subjects:    ['courses', 'subjects'] as const,
}

export function useCourses() {
  return useQuery({ queryKey: courseKeys.all, queryFn: coursesApi.list })
}

// Предметы тьютора для комбобокса. Список меняется только вместе с курсами,
// поэтому живёт под тем же ключом ['courses'] и чинится их инвалидацией.
export function useSubjects() {
  return useQuery({ queryKey: courseKeys.subjects, queryFn: coursesApi.subjects })
}

export function useCourse(id: string) {
  return useQuery({ queryKey: courseKeys.detail(id), queryFn: () => coursesApi.get(id) })
}

export function useCourseBalance(id: string) {
  return useQuery({ queryKey: courseKeys.balance(id), queryFn: () => coursesApi.getBalance(id) })
}

export function useCourseEnrollments(id: string) {
  return useQuery({
    queryKey: courseKeys.enrollments(id),
    queryFn:  () => coursesApi.getEnrollments(id),
    enabled:  !!id,
  })
}

export function useStudentCourses(studentId: string) {
  return useQuery({
    queryKey: courseKeys.byStudent(studentId),
    queryFn:  () => coursesApi.listByStudent(studentId),
    enabled:  !!studentId,
  })
}

export function useCreateCourse() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: coursesApi.create,
    onSuccess:  () => qc.invalidateQueries({ queryKey: courseKeys.all }),
  })
}

export function useUpdateCourse(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Omit<CourseInput, 'student_id'>) => coursesApi.update(id, data),
    onSuccess:  (updated) => {
      qc.invalidateQueries({ queryKey: courseKeys.all })
      qc.setQueryData(courseKeys.detail(id), updated)
    },
  })
}

export function useDeleteCourse() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: coursesApi.delete,
    onSuccess:  () => qc.invalidateQueries({ queryKey: courseKeys.all }),
  })
}

export function useAddEnrollment(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (studentId: string) => coursesApi.addEnrollment(courseId, studentId),
    onSuccess:  () => qc.invalidateQueries({ queryKey: courseKeys.enrollments(courseId) }),
  })
}

// Состав группы одним запросом: собирать её кликами по одному ученику — это
// диалог «добавить» N раз вместо одной формы.
export function useAddEnrollmentsBulk() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ courseId, studentIds }: { courseId: string; studentIds: string[] }) =>
      coursesApi.addEnrollmentsBulk(courseId, studentIds),
    onSuccess: (_, { courseId }) =>
      qc.invalidateQueries({ queryKey: courseKeys.enrollments(courseId) }),
  })
}

export function useRemoveEnrollment(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (studentId: string) => coursesApi.removeEnrollment(courseId, studentId),
    onSuccess:  () => qc.invalidateQueries({ queryKey: courseKeys.enrollments(courseId) }),
  })
}

export function useCoursesPaged(params: CourseListParams) {
  return useQuery({
    queryKey: [...courseKeys.all, 'list', params],
    queryFn:  () => coursesApi.listPaged(params),
  })
}

export function useCourseCount() {
  return useQuery({
    queryKey: [...courseKeys.all, 'count'],
    queryFn:  () => coursesApi.listPaged({ page: 1, limit: 1, search: '' }).then((r) => r.total),
  })
}

export function useArchivedCoursesPaged(params: CourseListParams) {
  return useQuery({
    queryKey: [...courseKeys.archived, 'list', params],
    queryFn:  () => coursesApi.listArchived(params),
  })
}

export function useRestoreCourse() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: coursesApi.restore,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: courseKeys.all })
      qc.invalidateQueries({ queryKey: courseKeys.archived })
    },
  })
}
