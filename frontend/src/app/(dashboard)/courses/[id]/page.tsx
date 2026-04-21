'use client'

import { useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { ArrowLeft, Pencil, Trash2, UserPlus, X } from 'lucide-react'

import {
  useCourse,
  useCourseBalance,
  useCourseEnrollments,
  useUpdateCourse,
  useDeleteCourse,
  useAddEnrollment,
  useRemoveEnrollment,
} from '@/lib/hooks/useCourses'
import { useStudents } from '@/lib/hooks/useStudents'
import { CourseForm } from '@/components/courses/CourseForm'
import { PageHeader } from '@/components/common/PageHeader'
import { CourseFormValues } from '@/schemas/course'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

export default function CourseDetailPage() {
  const { id } = useParams<{ id: string }>()
  const router  = useRouter()

  const { data: course, isLoading } = useCourse(id)
  const { data: balance }           = useCourseBalance(id)
  const { data: enrollments = [] }  = useCourseEnrollments(id)
  const { data: students = [] }     = useStudents()

  const [formOpen, setFormOpen]           = useState(false)
  const [selectedStudent, setSelected]    = useState('')

  const updateCourse    = useUpdateCourse(id)
  const deleteCourse    = useDeleteCourse()
  const addEnrollment   = useAddEnrollment(id)
  const removeEnrollment = useRemoveEnrollment(id)

  async function handleUpdate(values: CourseFormValues) {
    const { type: _type, student_id: _sid, started_at, ended_at, ...rest } = values
    await updateCourse.mutateAsync({
      ...rest,
      started_at: `${started_at}T00:00:00Z`,
      ended_at:   ended_at ? `${ended_at}T00:00:00Z` : undefined,
    })
    toast.success('Курс обновлён')
  }

  async function handleDelete() {
    if (!course || !confirm(`Удалить курс "${course.subject}"?`)) return
    try {
      await deleteCourse.mutateAsync(course.id)
      router.push('/courses')
    } catch {
      toast.error('Нельзя удалить курс с уроками')
    }
  }

  async function handleAddEnrollment() {
    if (!selectedStudent) return
    try {
      await addEnrollment.mutateAsync(selectedStudent)
      setSelected('')
      toast.success('Ученик добавлен')
    } catch {
      toast.error('Ошибка добавления')
    }
  }

  async function handleRemoveEnrollment(studentId: string) {
    await removeEnrollment.mutateAsync(studentId)
    toast.success('Ученик удалён')
  }

  if (isLoading) {
    return <div className="space-y-3">{[...Array(4)].map((_, i) => (
      <div key={i} className="h-10 rounded-md bg-muted animate-pulse" />
    ))}</div>
  }

  if (!course) return <p className="text-muted-foreground">Курс не найден</p>

  const isGroup = !course.student_id
  const enrolledIds = new Set(enrollments.map((e) => e.student_id))
  const availableStudents = students.filter((s) => !enrolledIds.has(s.id))

  function Row({ label, value }: { label: string; value: React.ReactNode }) {
    return (
      <div className="flex justify-between py-2 border-b last:border-0 text-sm">
        <span className="text-muted-foreground">{label}</span>
        <span className="font-medium">{value}</span>
      </div>
    )
  }

  return (
    <>
      <div className="mb-4">
        <Button variant="ghost" size="sm" onClick={() => router.push('/courses')}>
          <ArrowLeft className="h-4 w-4 mr-1.5" /> Курсы
        </Button>
      </div>

      <PageHeader
        title={course.subject}
        description={isGroup ? 'Групповой курс' : 'Индивидуальный курс'}
        actions={
          <div className="flex gap-2">
            <Button size="sm" variant="outline" onClick={() => setFormOpen(true)}>
              <Pencil className="h-4 w-4 mr-1.5" /> Редактировать
            </Button>
            <Button size="sm" variant="destructive" onClick={handleDelete}>
              <Trash2 className="h-4 w-4 mr-1.5" /> Удалить
            </Button>
          </div>
        }
      />

      <div className="grid gap-4 md:grid-cols-2 mt-4">
        {/* Info card */}
        <div className="border rounded-lg p-4">
          <h2 className="text-sm font-semibold mb-2">Информация</h2>
          <Row label="Предмет" value={course.subject} />
          <Row
            label="Тип"
            value={
              <Badge variant={isGroup ? 'secondary' : 'default'}>
                {isGroup ? 'Групповой' : 'Индивидуальный'}
              </Badge>
            }
          />
          <Row label="Цена за урок" value={`${course.price_per_lesson.toLocaleString()} ₸`} />
          <Row label="Начало" value={new Date(course.started_at).toLocaleDateString('ru-RU')} />
          {course.ended_at && (
            <Row label="Окончание" value={new Date(course.ended_at).toLocaleDateString('ru-RU')} />
          )}
        </div>

        {/* Balance widget */}
        <div className="border rounded-lg p-4">
          <h2 className="text-sm font-semibold mb-3">Баланс уроков</h2>
          {balance ? (
            <div className="grid grid-cols-3 gap-3 text-center">
              <div>
                <p className="text-2xl font-bold">{balance.lessons_paid}</p>
                <p className="text-xs text-muted-foreground mt-1">Оплачено</p>
              </div>
              <div>
                <p className="text-2xl font-bold">{balance.lessons_completed}</p>
                <p className="text-xs text-muted-foreground mt-1">Проведено</p>
              </div>
              <div>
                <p className="text-2xl font-bold text-primary">{balance.lessons_remaining}</p>
                <p className="text-xs text-muted-foreground mt-1">Осталось</p>
              </div>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Загрузка...</p>
          )}
        </div>
      </div>

      {/* Enrollments — group courses only */}
      {isGroup && (
        <div className="border rounded-lg p-4 mt-4">
          <h2 className="text-sm font-semibold mb-3">Ученики группы</h2>

          {/* Add student */}
          {availableStudents.length > 0 && (
            <div className="flex gap-2 mb-4">
              <select
                value={selectedStudent}
                onChange={(e) => setSelected(e.target.value)}
                className="flex h-9 flex-1 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                <option value="">Выберите ученика...</option>
                {availableStudents.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.first_name} {s.last_name}
                  </option>
                ))}
              </select>
              <Button size="sm" onClick={handleAddEnrollment} disabled={!selectedStudent}>
                <UserPlus className="h-4 w-4 mr-1.5" /> Добавить
              </Button>
            </div>
          )}

          {/* Enrolled list */}
          {enrollments.length === 0 ? (
            <p className="text-sm text-muted-foreground">Нет записанных учеников</p>
          ) : (
            <ul className="space-y-1">
              {enrollments.map((e) => (
                <li key={e.student_id} className="flex items-center justify-between py-1.5 border-b last:border-0 text-sm">
                  <span>{e.student_first_name} {e.student_last_name}</span>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="h-7 w-7 text-destructive hover:text-destructive"
                    onClick={() => handleRemoveEnrollment(e.student_id)}
                  >
                    <X className="h-3.5 w-3.5" />
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      <CourseForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSubmit={handleUpdate}
        initial={course}
      />
    </>
  )
}
