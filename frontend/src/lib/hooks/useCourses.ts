import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { coursesApi, CourseInput } from '@/lib/api/courses'

export const courseKeys = {
  all:         ['courses'] as const,
  detail:      (id: string) => ['courses', id] as const,
  balance:     (id: string) => ['courses', id, 'balance'] as const,
  enrollments: (id: string) => ['courses', id, 'enrollments'] as const,
  byStudent:   (studentId: string) => ['courses', 'student', studentId] as const,
}

export function useCourses() {
  return useQuery({ queryKey: courseKeys.all, queryFn: coursesApi.list })
}

export function useCourse(id: string) {
  return useQuery({ queryKey: courseKeys.detail(id), queryFn: () => coursesApi.list().then(l => l.find(c => c.id === id)!) })
}

export function useCourseBalance(id: string) {
  return useQuery({ queryKey: courseKeys.balance(id), queryFn: () => coursesApi.getBalance(id) })
}

export function useCourseEnrollments(id: string) {
  return useQuery({ queryKey: courseKeys.enrollments(id), queryFn: () => coursesApi.getEnrollments(id) })
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

export function useRemoveEnrollment(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (studentId: string) => coursesApi.removeEnrollment(courseId, studentId),
    onSuccess:  () => qc.invalidateQueries({ queryKey: courseKeys.enrollments(courseId) }),
  })
}
