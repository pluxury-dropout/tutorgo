'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { BookOpen, Plus, Pencil, Trash2 } from 'lucide-react'

import { useCourses, useCreateCourse, useUpdateCourse, useDeleteCourse } from '@/lib/hooks/useCourses'
import { useStudents } from '@/lib/hooks/useStudents'
import { CourseForm } from '@/components/courses/CourseForm'
import { PageHeader } from '@/components/common/PageHeader'
import { EmptyState } from '@/components/common/EmptyState'
import { CourseFormValues } from '@/schemas/course'
import { Course } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'

export default function CoursesPage() {
  const router = useRouter()
  const { data: courses = [], isLoading } = useCourses()
  const { data: students = [] } = useStudents()

  const [search, setSearch]     = useState('')
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing]   = useState<Course | undefined>()

  const createCourse = useCreateCourse()
  const updateCourse = useUpdateCourse(editing?.id ?? '')
  const deleteCourse = useDeleteCourse()

  const filtered = courses.filter((c) =>
    c.subject.toLowerCase().includes(search.toLowerCase())
  )

  function openCreate() {
    setEditing(undefined)
    setFormOpen(true)
  }

  function openEdit(course: Course) {
    setEditing(course)
    setFormOpen(true)
  }

  async function handleSubmit(values: CourseFormValues) {
    const { type, student_id, started_at, ended_at, ...rest } = values
    const payload = {
      ...rest,
      started_at: `${started_at}T00:00:00Z`,
      ended_at:   ended_at ? `${ended_at}T00:00:00Z` : undefined,
    }
    if (editing) {
      await updateCourse.mutateAsync(payload)
      toast.success('Курс обновлён')
    } else {
      await createCourse.mutateAsync({
        ...payload,
        student_id: type === 'individual' && student_id ? student_id : undefined,
      })
      toast.success('Курс добавлен')
    }
  }

  async function handleDelete(course: Course) {
    if (!confirm(`Удалить курс "${course.subject}"?`)) return
    try {
      await deleteCourse.mutateAsync(course.id)
      toast.success('Курс удалён')
    } catch {
      toast.error('Нельзя удалить курс с уроками')
    }
  }

  function studentName(course: Course) {
    if (!course.student_id) return null
    const s = students.find((s) => s.id === course.student_id)
    return s ? `${s.first_name} ${s.last_name}` : '—'
  }

  return (
    <>
      <PageHeader
        title="Курсы"
        description={`${courses.length} курсов`}
        actions={
          <Button size="sm" onClick={openCreate}>
            <Plus className="h-4 w-4 mr-1.5" /> Добавить
          </Button>
        }
      />

      <div className="mb-4">
        <Input
          placeholder="Поиск по предмету..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>

      {isLoading ? (
        <div className="space-y-2">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="h-12 rounded-md bg-muted animate-pulse" />
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={BookOpen}
          title={search ? 'Ничего не найдено' : 'Нет курсов'}
          description={search ? 'Попробуй другой запрос' : 'Добавь первый курс'}
          action={!search ? { label: 'Добавить курс', onClick: openCreate } : undefined}
        />
      ) : (
        <div className="border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40">
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">Предмет</th>
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">Тип</th>
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">Ученик</th>
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">Цена / урок</th>
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">Начало</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody>
              {filtered.map((course) => (
                <tr
                  key={course.id}
                  className="border-b last:border-0 hover:bg-muted/30 cursor-pointer"
                  onClick={() => router.push(`/courses/${course.id}`)}
                >
                  <td className="px-4 py-3 font-medium">{course.subject}</td>
                  <td className="px-4 py-3">
                    <Badge variant={course.student_id ? 'default' : 'secondary'}>
                      {course.student_id ? 'Индивидуальный' : 'Групповой'}
                    </Badge>
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {studentName(course) ?? '—'}
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {course.price_per_lesson.toLocaleString()} ₸
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {new Date(course.started_at).toLocaleDateString('ru-RU')}
                  </td>
                  <td className="px-4 py-3">
                    <div
                      className="flex items-center justify-end gap-1"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <Button size="icon" variant="ghost" className="h-8 w-8"
                        onClick={() => openEdit(course)}>
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button size="icon" variant="ghost"
                        className="h-8 w-8 text-destructive hover:text-destructive"
                        onClick={() => handleDelete(course)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <CourseForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSubmit={handleSubmit}
        initial={editing}
      />
    </>
  )
}
