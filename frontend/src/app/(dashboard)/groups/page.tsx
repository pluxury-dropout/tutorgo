'use client'

import { Suspense, useState, useEffect, useRef } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { toast } from 'sonner'
import { Users, Plus, Pencil, Trash2, ChevronRight, ArchiveRestore } from 'lucide-react'

import { useCoursesPaged, useCreateCourse, useUpdateCourse, useDeleteCourse, useArchivedCoursesPaged, useRestoreCourse, useAddEnrollmentsBulk } from '@/lib/hooks/useCourses'
import { CourseForm } from '@/components/courses/CourseForm'
import { PageHeader, HeaderMetric } from '@/components/common/PageHeader'
import { SectionCard } from '@/components/common/SectionCard'
import { EmptyState } from '@/components/common/EmptyState'
import { ErrorState } from '@/components/common/ErrorState'
import { Pagination } from '@/components/common/Pagination'
import { CourseFormValues } from '@/schemas/course'
import { Course } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const LIMIT = 20

function GroupsPageInner() {
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
      router.replace(`/groups?${p}`)
    }, 300)
    return () => clearTimeout(t)
  }, [localSearch]) // eslint-disable-line react-hooks/exhaustive-deps

  function handlePageChange(newPage: number) {
    const p = new URLSearchParams(searchParams.toString())
    p.set('page', String(newPage))
    router.push(`/groups?${p}`)
  }

  function changeTab(next: 'active' | 'archive') {
    setTab(next)
    const p = new URLSearchParams(searchParams.toString())
    if (next === 'active') p.delete('tab')
    else p.set('tab', next)
    router.replace(`/groups?${p}`)
  }

  const { data, isLoading, isError: coursesError, refetch: refetchCourses } =
    useCoursesPaged({ page, limit: LIMIT, search })
  // ponytail: фильтр групп — на клиенте, поверх пагинации по ВСЕМ курсам
  // тьютора (индивидуальные + групповые). Спека 2026-09-06 п.8 явно
  // допускает это, пока курсов у тьютора десятки: total/totalPages ниже
  // считаются по несмешанному списку, так что при большом перекосе
  // individual/group пагинация может обсчитаться. Апгрейд — параметр
  // ?type=group на GET /courses, если список разрастётся.
  const courses    = (data?.data ?? []).filter((c) => !c.student_id)
  const total      = data?.total ?? 0
  const totalPages = Math.ceil(total / LIMIT)

  useEffect(() => {
    if (!isLoading && total > 0 && page > totalPages) {
      handlePageChange(totalPages)
    }
  }, [isLoading, total, page, totalPages]) // eslint-disable-line react-hooks/exhaustive-deps

  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing]   = useState<Course | undefined>()

  const createCourse   = useCreateCourse()
  const addEnrollments = useAddEnrollmentsBulk()
  const updateCourse = useUpdateCourse(editing?.id ?? '')
  const deleteCourse = useDeleteCourse()

  const tabParam = searchParams.get('tab')
  const [tab, setTab] = useState<'active' | 'archive'>(
    tabParam === 'archive' ? 'archive' : 'active'
  )
  const [archivePage, setArchivePage] = useState(1)

  const {
    data: archivedData, isLoading: archivedLoading,
    isError: archivedError, refetch: refetchArchived,
  } = useArchivedCoursesPaged({
    page: archivePage, limit: LIMIT, search,
  })
  const archivedCourses = (archivedData?.data ?? []).filter((c) => !c.student_id)
  const archivedTotal   = archivedData?.total ?? 0
  const archivedPages   = Math.ceil(archivedTotal / LIMIT)

  useEffect(() => { setArchivePage(1) }, [search])

  const restoreCourse = useRestoreCourse()

  function openCreate() { setEditing(undefined); setFormOpen(true) }
  function openEdit(c: Course) { setEditing(c); setFormOpen(true) }

  async function handleSubmit(values: CourseFormValues) {
    const { student_ids, started_at, ended_at, ...rest } = values
    const payload = {
      ...rest,
      started_at: `${started_at}T00:00:00Z`,
      ended_at:   ended_at ? `${ended_at}T00:00:00Z` : undefined,
    }
    if (editing) {
      await updateCourse.mutateAsync(payload)
      toast.success('Группа обновлена')
      return
    }

    const course = await createCourse.mutateAsync(payload)
    if (student_ids && student_ids.length > 0) {
      try {
        await addEnrollments.mutateAsync({ courseId: course.id, studentIds: student_ids })
      } catch {
        toast.error('Группа создана, но учеников записать не удалось')
      }
    }
    toast.success('Группа создана')
  }

  async function handleDelete(course: Course) {
    if (!confirm(`Архивировать группу "${course.subject}"? Завершённые уроки останутся в календаре, будущие будут удалены.`)) return
    try {
      await deleteCourse.mutateAsync(course.id)
      toast.success('Группа архивирована')
    } catch {
      toast.error('Ошибка архивирования')
    }
  }

  async function handleRestore(course: Course) {
    try {
      await restoreCourse.mutateAsync(course.id)
      toast.success('Группа восстановлена')
    } catch {
      toast.error('Ошибка восстановления')
    }
  }

  return (
    <div style={{ maxWidth: 900 }}>
      <PageHeader
        title="Группы"
        meta={
          <HeaderMetric color="var(--success)">
            {tab === 'active' ? `${courses.length} групп` : `${archivedCourses.length} в архиве`}
          </HeaderMetric>
        }
        actions={
          tab === 'active' ? (
            <Button size="sm" onClick={openCreate}>
              <Plus className="h-4 w-4 mr-1.5" /> Добавить
            </Button>
          ) : null
        }
      />

      <div className="flex gap-1 mb-4" style={{ borderBottom: '1px solid var(--border)', paddingBottom: 0 }}>
        {(['active', 'archive'] as const).map((t) => (
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
            {t === 'active' ? 'Активные' : 'Архив'}
          </button>
        ))}
      </div>

      <div className="mb-4">
        <Input
          placeholder="Поиск по предмету..."
          value={localSearch}
          onChange={(e) => setLocalSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>

      {tab === 'active' ? (
        isLoading ? (
          <div className="space-y-2">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="h-12 rounded-md bg-muted animate-pulse" />
            ))}
          </div>
        ) : coursesError ? (
          <ErrorState what="группы" onRetry={() => refetchCourses()} />
        ) : courses.length === 0 ? (
          <EmptyState
            icon={Users}
            title={search ? 'Ничего не найдено' : 'Групп пока нет'}
            description={search
              ? 'Попробуй другой запрос — поиск идёт по названию предмета'
              : 'Группа — предмет и цена за одного участника. Индивидуальные занятия заводить не нужно: они появляются сами, как только ставишь ученику урок'}
            action={!search ? { label: 'Добавить группу', onClick: openCreate } : undefined}
          />
        ) : (
          <>
            <SectionCard>
              <div style={{
                display: 'grid',
                gridTemplateColumns: '1.5fr 1fr 90px 16px 72px',
                alignItems: 'baseline',
                gap: 12,
                padding: '10px 18px 8px',
                borderBottom: '1px solid var(--border)',
              }}>
                {(['Предмет', 'Цена за цикл', 'Начало', '', ''] as const).map((label, i) => (
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
                    gridTemplateColumns: '1.5fr 1fr 90px 16px 72px',
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
        archivedLoading ? (
          <div className="space-y-2">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="h-12 rounded-md bg-muted animate-pulse" />
            ))}
          </div>
        ) : archivedError ? (
          <ErrorState what="архив" onRetry={() => refetchArchived()} />
        ) : archivedCourses.length === 0 ? (
          <EmptyState
            icon={Users}
            title={search ? 'Ничего не найдено' : 'Архив пуст'}
            description={search
              ? 'Попробуй другой запрос — поиск идёт по названию предмета'
              : 'Сюда переезжают завершённые группы: они исчезают из расписания и списков, но история уроков и оплат сохраняется'}
          />
        ) : (
          <>
            <SectionCard>
              <div style={{
                display: 'grid',
                gridTemplateColumns: '1.5fr 1fr 90px 16px 120px',
                alignItems: 'baseline',
                gap: 12,
                padding: '10px 18px 8px',
                borderBottom: '1px solid var(--border)',
              }}>
                {(['Предмет', 'Цена за цикл', 'Начало', '', ''] as const).map((label, i) => (
                  <span key={i} style={{ fontSize: 11.5, fontWeight: 500, color: 'var(--muted-foreground)', letterSpacing: '0.05em', textTransform: 'uppercase' }}>{label}</span>
                ))}
              </div>
              {archivedCourses.map((course, i) => (
                <div
                  key={course.id}
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '1.5fr 1fr 90px 16px 120px',
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
    </div>
  )
}

export default function GroupsPage() {
  return (
    <Suspense>
      <GroupsPageInner />
    </Suspense>
  )
}
