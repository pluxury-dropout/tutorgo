'use client'

import { Suspense, useState, useEffect, useRef } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { toast } from 'sonner'
import { BookOpen, Plus, Pencil, Trash2, ChevronRight } from 'lucide-react'

import { useCoursesPaged, useCreateCourse, useUpdateCourse, useDeleteCourse } from '@/lib/hooks/useCourses'
import { useStudents } from '@/lib/hooks/useStudents'
import { CourseForm } from '@/components/courses/CourseForm'
import { PageHeader } from '@/components/common/PageHeader'
import { EmptyState } from '@/components/common/EmptyState'
import { Pagination } from '@/components/common/Pagination'
import { CourseTypeBadge } from '@/components/common/CourseTypeBadge'
import { CourseFormValues } from '@/schemas/course'
import { Course } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const LIMIT = 20

function CoursesPageInner() {
  const router       = useRouter()
  const searchParams = useSearchParams()

  const page   = Math.max(1, Number(searchParams.get('page') ?? '1'))
  const search = searchParams.get('search') ?? ''

  const [localSearch, setLocalSearch] = useState(search)
  const mounted = useRef(false)

  useEffect(() => { setLocalSearch(search) }, [search])

  useEffect(() => {
    if (!mounted.current) { mounted.current = true; return }
    if (localSearch === search) return
    const t = setTimeout(() => {
      const p = new URLSearchParams()
      if (localSearch) p.set('search', localSearch)
      p.set('page', '1')
      router.replace(`/courses?${p}`)
    }, 300)
    return () => clearTimeout(t)
  }, [localSearch]) // eslint-disable-line react-hooks/exhaustive-deps

  function handlePageChange(newPage: number) {
    const p = new URLSearchParams(searchParams.toString())
    p.set('page', String(newPage))
    router.push(`/courses?${p}`)
  }

  const { data, isLoading } = useCoursesPaged({ page, limit: LIMIT, search })
  const courses    = data?.data ?? []
  const total      = data?.total ?? 0
  const totalPages = Math.ceil(total / LIMIT)

  useEffect(() => {
    if (!isLoading && total > 0 && page > totalPages) {
      handlePageChange(totalPages)
    }
  }, [isLoading, total, page, totalPages]) // eslint-disable-line react-hooks/exhaustive-deps

  const { data: students = [] } = useStudents()

  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing]   = useState<Course | undefined>()

  const createCourse = useCreateCourse()
  const updateCourse = useUpdateCourse(editing?.id ?? '')
  const deleteCourse = useDeleteCourse()

  function openCreate() { setEditing(undefined); setFormOpen(true) }
  function openEdit(c: Course) { setEditing(c); setFormOpen(true) }

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
    return s ? `${s.first_name}${s.last_name ? ` ${s.last_name}` : ''}` : '—'
  }

  return (
    <>
      <PageHeader
        title="Курсы"
        description={`${total} курсов`}
        icon={BookOpen}
        iconBg="oklch(0.92 0.05 155)"
        iconColor="oklch(0.36 0.10 155)"
        actions={
          <Button size="sm" onClick={openCreate}>
            <Plus className="h-4 w-4 mr-1.5" /> Добавить
          </Button>
        }
      />

      <div className="mb-4">
        <Input
          placeholder="Поиск по предмету..."
          value={localSearch}
          onChange={(e) => setLocalSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>

      {isLoading ? (
        <div className="space-y-2">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="h-12 rounded-md bg-muted animate-pulse" />
          ))}
        </div>
      ) : courses.length === 0 ? (
        <EmptyState
          icon={BookOpen}
          title={search ? 'Ничего не найдено' : 'Нет курсов'}
          description={search ? 'Попробуй другой запрос' : 'Добавь первый курс'}
          action={!search ? { label: 'Добавить курс', onClick: openCreate } : undefined}
        />
      ) : (
        <>
          <div className="border rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/40">
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Предмет</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Тип</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Ученик</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Цена / урок</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Начало</th>
                  <th className="w-4" />
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody>
                {courses.map((course) => (
                  <tr
                    key={course.id}
                    className="border-b last:border-0 hover:bg-muted/30 cursor-pointer group"
                    onClick={() => router.push(`/courses/${course.id}`)}
                  >
                    <td className="px-4 py-3 font-medium">{course.subject}</td>
                    <td className="px-4 py-3"><CourseTypeBadge isGroup={!course.student_id} /></td>
                    <td className="px-4 py-3 text-muted-foreground">{studentName(course) ?? '—'}</td>
                    <td className="px-4 py-3 text-muted-foreground">{course.price_per_lesson.toLocaleString()} ₸</td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {new Date(course.started_at).toLocaleDateString('ru-RU')}
                    </td>
                    <td className="pr-1 py-3 w-4">
                      <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity duration-150" />
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-1" onClick={(e) => e.stopPropagation()}>
                        <Button size="icon" variant="ghost" className="h-8 w-8" onClick={() => openEdit(course)}>
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
          {totalPages > 1 && (
            <div className="flex items-center justify-between mt-3 px-1">
              <span className="text-xs text-muted-foreground">
                Страница {page} из {totalPages}
              </span>
              <Pagination page={page} totalPages={totalPages} onPageChange={handlePageChange} />
            </div>
          )}
        </>
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

export default function CoursesPage() {
  return (
    <Suspense>
      <CoursesPageInner />
    </Suspense>
  )
}
