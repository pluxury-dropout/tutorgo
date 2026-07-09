'use client'

import { Suspense, useState, useEffect, useRef } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { toast } from 'sonner'
import { BookOpen, Plus, Pencil, Trash2, ChevronRight, ArchiveRestore, Users } from 'lucide-react'

import { useCoursesPaged, useCreateCourse, useUpdateCourse, useDeleteCourse, useArchivedCoursesPaged, useRestoreCourse } from '@/lib/hooks/useCourses'
import { useStudents, useStudentsPaged, useCreateStudent, useUpdateStudent, useDeleteStudent } from '@/lib/hooks/useStudents'
import { CourseForm } from '@/components/courses/CourseForm'
import { StudentForm } from '@/components/students/StudentForm'
import { StudentsList } from '@/components/students/StudentsList'
import { PageHeader, HeaderMetric } from '@/components/common/PageHeader'
import { SectionCard } from '@/components/common/SectionCard'
import { EmptyState } from '@/components/common/EmptyState'
import { Pagination } from '@/components/common/Pagination'
import { CourseTypeBadge } from '@/components/common/CourseTypeBadge'
import { CourseFormValues } from '@/schemas/course'
import { StudentFormValues } from '@/schemas/student'
import { Course, Student } from '@/types/api'
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

  function changeTab(next: 'active' | 'students' | 'archive') {
    setTab(next)
    const p = new URLSearchParams(searchParams.toString())
    if (next === 'active') p.delete('tab')
    else p.set('tab', next)
    router.replace(`/courses?${p}`)
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

  const tabParam = searchParams.get('tab')
  const [tab, setTab] = useState<'active' | 'students' | 'archive'>(
    tabParam === 'students' ? 'students' : tabParam === 'archive' ? 'archive' : 'active'
  )
  const [archivePage, setArchivePage] = useState(1)

  const [studentSearch, setStudentSearch] = useState('')
  const [studentPage, setStudentPage] = useState(1)
  useEffect(() => { setStudentPage(1) }, [studentSearch])

  const { data: studentsData, isLoading: studentsLoading } = useStudentsPaged({
    page: studentPage, limit: LIMIT, search: studentSearch,
  })
  const studentList  = studentsData?.data ?? []
  const studentTotal = studentsData?.total ?? 0
  const studentPages  = Math.ceil(studentTotal / LIMIT)

  const [studentFormOpen, setStudentFormOpen] = useState(false)
  const [editingStudent, setEditingStudent]   = useState<Student | undefined>()

  const createStudent = useCreateStudent()
  const updateStudent = useUpdateStudent(editingStudent?.id ?? '')
  const deleteStudent = useDeleteStudent()

  function openCreateStudent() { setEditingStudent(undefined); setStudentFormOpen(true) }
  function openEditStudent(s: Student) { setEditingStudent(s); setStudentFormOpen(true) }

  async function handleStudentSubmit(values: StudentFormValues) {
    if (editingStudent) {
      await updateStudent.mutateAsync(values)
      toast.success('Ученик обновлён')
    } else {
      await createStudent.mutateAsync(values)
      toast.success('Ученик добавлен')
    }
  }

  async function handleStudentDelete(s: Student) {
    if (!confirm(`Удалить ${s.first_name}${s.last_name ? ` ${s.last_name}` : ''}?`)) return
    await deleteStudent.mutateAsync(s.id)
    toast.success('Ученик удалён')
  }

  const { data: archivedData, isLoading: archivedLoading } = useArchivedCoursesPaged({
    page: archivePage, limit: LIMIT, search,
  })
  const archivedCourses = archivedData?.data ?? []
  const archivedTotal   = archivedData?.total ?? 0
  const archivedPages   = Math.ceil(archivedTotal / LIMIT)

  useEffect(() => { setArchivePage(1) }, [search])

  const restoreCourse = useRestoreCourse()

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
    if (!confirm(`Архивировать курс "${course.subject}"? Завершённые уроки останутся в календаре.`)) return
    try {
      await deleteCourse.mutateAsync(course.id)
      toast.success('Курс архивирован')
    } catch {
      toast.error('Ошибка архивирования')
    }
  }

  async function handleRestore(course: Course) {
    try {
      await restoreCourse.mutateAsync(course.id)
      toast.success('Курс восстановлен')
    } catch {
      toast.error('Ошибка восстановления')
    }
  }

  function studentName(course: Course) {
    if (!course.student_id) return null
    const s = students.find((s) => s.id === course.student_id)
    return s ? `${s.first_name}${s.last_name ? ` ${s.last_name}` : ''}` : '—'
  }

  return (
    <div style={{ maxWidth: 900 }}>
      <PageHeader
        title={tab === 'students' ? 'Ученики' : 'Курсы'}
        meta={
          tab === 'students' ? (
            <HeaderMetric color="var(--purple)">{studentTotal} учеников</HeaderMetric>
          ) : (
            <HeaderMetric color="var(--success)">
              {tab === 'active' ? `${total} курсов` : `${archivedTotal} в архиве`}
            </HeaderMetric>
          )
        }
        actions={
          tab === 'active' ? (
            <Button size="sm" onClick={openCreate}>
              <Plus className="h-4 w-4 mr-1.5" /> Добавить
            </Button>
          ) : tab === 'students' ? (
            <Button size="sm" onClick={openCreateStudent}>
              <Plus className="h-4 w-4 mr-1.5" /> Добавить
            </Button>
          ) : null
        }
      />

      {/* Tab switcher */}
      <div className="flex gap-1 mb-4" style={{ borderBottom: '1px solid var(--border)', paddingBottom: 0 }}>
        {(['active', 'students', 'archive'] as const).map((t) => (
          <button
            key={t}
            onClick={() => changeTab(t)}
            style={{
              padding: '6px 16px',
              fontSize: 13,
              fontWeight: tab === t ? 600 : 400,
              color: tab === t ? 'var(--foreground)' : 'var(--muted-foreground)',
              background: 'none',
              border: 'none',
              borderBottom: tab === t ? '2px solid var(--foreground)' : '2px solid transparent',
              cursor: 'pointer',
              marginBottom: -1,
            }}
          >
            {t === 'active' ? 'Активные' : t === 'students' ? 'Ученики' : 'Архив'}
          </button>
        ))}
      </div>

      <div className="mb-4">
        <Input
          placeholder={tab === 'students' ? 'Поиск по имени или email...' : 'Поиск по предмету...'}
          value={tab === 'students' ? studentSearch : localSearch}
          onChange={(e) => tab === 'students' ? setStudentSearch(e.target.value) : setLocalSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>

      {tab === 'students' ? (
        /* ── Students tab ── */
        studentsLoading ? (
          <div className="space-y-2">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="h-12 rounded-md bg-muted animate-pulse" />
            ))}
          </div>
        ) : studentList.length === 0 ? (
          <EmptyState
            icon={Users}
            title={studentSearch ? 'Ничего не найдено' : 'Нет учеников'}
            description={studentSearch ? 'Попробуй другой запрос' : 'Добавь первого ученика'}
            action={!studentSearch ? { label: 'Добавить ученика', onClick: openCreateStudent } : undefined}
          />
        ) : (
          <>
            <StudentsList students={studentList} onEdit={openEditStudent} onDelete={handleStudentDelete} />
            {studentPages > 1 && (
              <div className="flex items-center justify-between mt-3 px-1">
                <span className="text-xs text-muted-foreground">
                  Страница {studentPage} из {studentPages}
                </span>
                <Pagination page={studentPage} totalPages={studentPages} onPageChange={setStudentPage} />
              </div>
            )}
          </>
        )
      ) : tab === 'active' ? (
        /* ── Active tab ── */
        isLoading ? (
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
            <SectionCard>
              <div style={{
                display: 'grid',
                gridTemplateColumns: '1.5fr 130px 1fr 160px 90px 16px 72px',
                alignItems: 'baseline',
                gap: 12,
                padding: '10px 18px 8px',
                borderBottom: '1px solid var(--border)',
              }}>
                {(['Предмет', 'Тип', 'Ученик', 'Цена за цикл', 'Начало', '', ''] as const).map((label, i) => (
                  <span key={i} style={{ fontSize: 11.5, fontWeight: 500, color: 'var(--muted-foreground)', letterSpacing: '0.05em', textTransform: 'uppercase' }}>{label}</span>
                ))}
              </div>
              {courses.map((course, i) => (
                <div
                  key={course.id}
                  role="button"
                  tabIndex={0}
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '1.5fr 130px 1fr 160px 90px 16px 72px',
                    alignItems: 'center',
                    gap: 12,
                    padding: '10px 18px',
                    borderTop: i === 0 ? 'none' : '1px solid var(--row-border)',
                    cursor: 'pointer',
                  }}
                  className="hover:bg-muted/30 group"
                  onClick={() => router.push(`/courses/${course.id}`)}
                  onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); router.push(`/courses/${course.id}`) } }}
                >
                  <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--foreground)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {course.subject}
                  </span>
                  <span><CourseTypeBadge isGroup={!course.student_id} /></span>
                  <span style={{ fontSize: 13, color: 'var(--muted-foreground)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {studentName(course) ?? '—'}
                  </span>
                  <span style={{ fontSize: 13, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
                    {course.price_per_cycle.toLocaleString()} ₸ / {course.lessons_per_cycle} ур.
                  </span>
                  <span style={{ fontSize: 13, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
                    {new Date(course.started_at).toLocaleDateString('ru-RU')}
                  </span>
                  <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity duration-150" />
                  <div className="flex items-center justify-end gap-1" onClick={(e) => e.stopPropagation()}>
                    <Button size="icon" variant="ghost" className="h-8 w-8" onClick={() => openEdit(course)}>
                      <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button size="icon" variant="ghost" className="h-8 w-8 text-destructive hover:text-destructive" onClick={() => handleDelete(course)}>
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
              ))}
            </SectionCard>
            {totalPages > 1 && (
              <div className="flex items-center justify-between mt-3 px-1">
                <span className="text-xs text-muted-foreground">Страница {page} из {totalPages}</span>
                <Pagination page={page} totalPages={totalPages} onPageChange={handlePageChange} />
              </div>
            )}
          </>
        )
      ) : (
        /* ── Archive tab ── */
        archivedLoading ? (
          <div className="space-y-2">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="h-12 rounded-md bg-muted animate-pulse" />
            ))}
          </div>
        ) : archivedCourses.length === 0 ? (
          <EmptyState
            icon={BookOpen}
            title={search ? 'Ничего не найдено' : 'Архив пуст'}
            description={search ? 'Попробуй другой запрос' : 'Архивированные курсы появятся здесь'}
          />
        ) : (
          <>
            <SectionCard>
              <div style={{
                display: 'grid',
                gridTemplateColumns: '1.5fr 130px 1fr 160px 90px 16px 120px',
                alignItems: 'baseline',
                gap: 12,
                padding: '10px 18px 8px',
                borderBottom: '1px solid var(--border)',
              }}>
                {(['Предмет', 'Тип', 'Ученик', 'Цена за цикл', 'Начало', '', ''] as const).map((label, i) => (
                  <span key={i} style={{ fontSize: 11.5, fontWeight: 500, color: 'var(--muted-foreground)', letterSpacing: '0.05em', textTransform: 'uppercase' }}>{label}</span>
                ))}
              </div>
              {archivedCourses.map((course, i) => (
                <div
                  key={course.id}
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '1.5fr 130px 1fr 160px 90px 16px 120px',
                    alignItems: 'center',
                    gap: 12,
                    padding: '10px 18px',
                    borderTop: i === 0 ? 'none' : '1px solid var(--row-border)',
                    opacity: 0.7,
                  }}
                >
                  <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--foreground)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {course.subject}
                  </span>
                  <span><CourseTypeBadge isGroup={!course.student_id} /></span>
                  <span style={{ fontSize: 13, color: 'var(--muted-foreground)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {studentName(course) ?? '—'}
                  </span>
                  <span style={{ fontSize: 13, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
                    {course.price_per_cycle.toLocaleString()} ₸ / {course.lessons_per_cycle} ур.
                  </span>
                  <span style={{ fontSize: 13, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
                    {new Date(course.started_at).toLocaleDateString('ru-RU')}
                  </span>
                  <span />
                  <div className="flex items-center justify-end">
                    <Button size="sm" variant="outline" className="h-7 text-xs gap-1" onClick={() => handleRestore(course)}>
                      <ArchiveRestore className="h-3 w-3" /> Восстановить
                    </Button>
                  </div>
                </div>
              ))}
            </SectionCard>
            {archivedPages > 1 && (
              <div className="flex items-center justify-between mt-3 px-1">
                <span className="text-xs text-muted-foreground">Страница {archivePage} из {archivedPages}</span>
                <Pagination page={archivePage} totalPages={archivedPages} onPageChange={setArchivePage} />
              </div>
            )}
          </>
        )
      )}

      <CourseForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSubmit={handleSubmit}
        initial={editing}
      />

      <StudentForm
        open={studentFormOpen}
        onClose={() => setStudentFormOpen(false)}
        onSubmit={handleStudentSubmit}
        initial={editingStudent}
      />
    </div>
  )
}

export default function CoursesPage() {
  return (
    <Suspense>
      <CoursesPageInner />
    </Suspense>
  )
}
