# Фаза 4: `/courses` → `/groups` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Убрать «Курсы» из навигации TutorGo как отдельную концепцию — раздел переименовывается в «Группы» и показывает только групповые курсы, `/courses` становится редиректом, форма создания курса теряет ветку для индивидуального курса (он и так создаётся автоматически через `GetOrCreateIndividual`, когда тьютор ставит урок ученику).

**Architecture:** Чисто фронтенд-задача, бэкенд и миграции не трогаются (эндпоинты `/courses`, `/courses/archived`, `POST /courses` и т.д. остаются как есть — фильтрация групп идёт на клиенте по уже существующему полю `student_id`). `/courses/[id]` как экран остаётся живым для прямых ссылок; список переезжает в новый роут `/groups`, а `/courses` (список) превращается в серверный редирект через `redirect()` из `next/navigation`.

**Tech Stack:** Next.js 16 (App Router), React Query, react-hook-form + zod, TypeScript strict.

**Spec:** `docs/specs/2026-09-06-price-units-and-student-centric-money.md`, раздел 8 «Фаза 4 — `/courses` → `/groups`» (строки 919–937), критерии приёмки — строки 932–936.

## Global Constraints

- Миграций нет, бэкенд не меняется (спека, п.8, преамбула).
- `/courses/:id` остаётся живым для прямых ссылок и перехода с карточки ученика (спека, п.8, п.7.3).
- Фильтрация групп — на клиенте по `student_id === null`, отдельный параметр API не заводим, пока курсов у тьютора десятки (спека, п.8).
- В навигации не должно остаться слова «курс» (критерий приёмки №1).
- Новый тьютор проходит путь «зарегистрировался → ученик → урок → оплата», ни разу не увидев экран создания курса (критерий приёмки №2).
- Тест-раннера для фронтенда в репозитории нет (ни vitest, ни jest, ни typecheck-скрипта в `package.json`) — верификация каждой задачи: `npx tsc --noEmit` (типы) + `npm run lint` (eslint). Оба запускать из `frontend/`.

---

## Task 1: Sidebar — «Курсы» → «Группы»

**Files:**
- Modify: `frontend/src/components/layout/Sidebar.tsx:6-19` (импорт иконок), `:404` (пункт `NAV`)

**Interfaces:** Нет — изолированная правка данных навигации, никто другой файл её не потребляет.

- [ ] **Step 1: Поменять пункт навигации**

В `frontend/src/components/layout/Sidebar.tsx` убрать `BookOpen` из импорта `lucide-react` (используется только в этом пункте `NAV`, других вхождений в файле нет) и поменять строку `NAV`:

```tsx
// было (строка ~404):
{ href: '/courses',    label: 'Курсы',        icon: BookOpen },
// стало:
{ href: '/groups',     label: 'Группы',       icon: Users },
```

`Users` уже импортирован (используется для пункта «Ученики») — новый импорт не нужен.

- [ ] **Step 2: Проверить типы и линт**

```bash
cd frontend && npx tsc --noEmit && npm run lint
```
Ожидается: без ошибок (в частности, ни одного предупреждения про неиспользуемый `BookOpen`).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/layout/Sidebar.tsx
git commit -m "feat(nav): переименовать «Курсы» в «Группы» в сайдбаре"
```

---

## Task 2: GettingStarted — убрать шаг «Создать курс»

**Files:**
- Modify: `frontend/src/components/common/GettingStarted.tsx`
- Modify: `frontend/src/app/(dashboard)/dashboard/page.tsx:154-159`

**Interfaces:**
- Produces: `GettingStartedProps` без поля `hasCourses` — три шага (`hasStudents`, `hasLessons`, `hasPayments`) вместо четырёх, что дословно соответствует пути из критерия приёмки «зарегистрировался → ученик → урок → оплата».

- [ ] **Step 1: Убрать `hasCourses` из `GettingStarted.tsx`**

Убрать поле из интерфейса, из деструктуризации пропсов, из массива шагов и поправить комментарий про количество шагов:

```tsx
// было:
interface GettingStartedProps {
  hasStudents: boolean
  hasCourses: boolean
  hasLessons: boolean
  hasPayments: boolean
  onAddStudent: () => void
}

/**
 * Чеклист первых шагов для нового преподавателя. Состояние шагов — производное
 * от данных, которые главная и так грузит: отдельного флага «онбординг пройден»
 * нет и не нужно. Когда все четыре шага выполнены, блок пропадает навсегда.
 */
export function GettingStarted({ hasStudents, hasCourses, hasLessons, hasPayments, onAddStudent }: GettingStartedProps) {
  const steps = [
    { done: hasStudents, label: 'Добавить ученика',           hint: 'Карточка с контактами — к ней привяжутся курсы и оплаты', href: '/students', cta: 'Добавить', onClick: onAddStudent },
    { done: hasCourses,  label: 'Создать курс',               hint: 'Предмет и цена за урок: из курса растут расписание и деньги', href: '/courses',              cta: 'Создать'  },
    { done: hasLessons,  label: 'Поставить урок в расписание', hint: 'Кликни по свободному слоту в календаре',                   href: '/calendar',             cta: 'В календарь' },
    { done: hasPayments, label: 'Отметить оплату',            hint: 'Приложение само посчитает, на сколько уроков хватит баланса', href: '/payments',             cta: 'К оплатам' },
  ]

// стало:
interface GettingStartedProps {
  hasStudents: boolean
  hasLessons: boolean
  hasPayments: boolean
  onAddStudent: () => void
}

/**
 * Чеклист первых шагов для нового преподавателя. Состояние шагов — производное
 * от данных, которые главная и так грузит: отдельного флага «онбординг пройден»
 * нет и не нужно. Когда все три шага выполнены, блок пропадает навсегда.
 */
export function GettingStarted({ hasStudents, hasLessons, hasPayments, onAddStudent }: GettingStartedProps) {
  const steps = [
    { done: hasStudents, label: 'Добавить ученика',           hint: 'Карточка с контактами — к ней привяжутся курсы и оплаты', href: '/students', cta: 'Добавить', onClick: onAddStudent },
    { done: hasLessons,  label: 'Поставить урок в расписание', hint: 'Кликни по свободному слоту в календаре',                   href: '/calendar',             cta: 'В календарь' },
    { done: hasPayments, label: 'Отметить оплату',            hint: 'Приложение само посчитает, на сколько уроков хватит баланса', href: '/payments',             cta: 'К оплатам' },
  ]
```

- [ ] **Step 2: Убрать проп `hasCourses` у вызова в `dashboard/page.tsx`**

```tsx
// было (строка ~154):
{/* Онбординг — сам исчезает, когда все четыре шага сделаны */}
{!loading && (
  <GettingStarted
    hasStudents={studentCount > 0}
    hasCourses={courseCount > 0}
    hasLessons={monthLessons.length > 0}
    hasPayments={recentPayments.length > 0}
    onAddStudent={() => setOnboardOpen(true)}
  />

// стало:
{/* Онбординг — сам исчезает, когда все три шага сделаны */}
{!loading && (
  <GettingStarted
    hasStudents={studentCount > 0}
    hasLessons={monthLessons.length > 0}
    hasPayments={recentPayments.length > 0}
    onAddStudent={() => setOnboardOpen(true)}
  />
```

`courseCount`/`useCourseCount()` не трогать — переменная используется отдельно, для KPI-плашки «N курсов» на дашборде (строка ~134), которая в рамках фазы 4 не меняется (спека называет только сайдбар, `/groups`, `/courses`, `CourseForm`, `schemas/course.ts` и `GettingStarted.tsx:24` — плашка дашборда в этот список не входит).

- [ ] **Step 3: Проверить типы и линт**

```bash
cd frontend && npx tsc --noEmit && npm run lint
```
Ожидается: без ошибок.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/common/GettingStarted.tsx "frontend/src/app/(dashboard)/dashboard/page.tsx"
git commit -m "feat(onboarding): убрать шаг «Создать курс» — путь student→lesson→payment"
```

---

## Task 3: `/groups` — новый список групп, `/courses` — редирект

**Files:**
- Create: `frontend/src/app/(dashboard)/groups/page.tsx`
- Modify: `frontend/src/app/(dashboard)/courses/page.tsx` (полная замена содержимого на редирect)

**Interfaces:**
- Consumes (без изменений в этой задаче): `useCoursesPaged`, `useArchivedCoursesPaged`, `useCreateCourse`, `useUpdateCourse`, `useDeleteCourse`, `useRestoreCourse`, `useAddEnrollmentsBulk` из `@/lib/hooks/useCourses`; `CourseForm` (ещё поддерживает проп `mode: 'individual' | 'group'`, старую схему с `type` — обе трогает Task 4); `Course`, `CourseFormValues` — типы без изменений.
- Produces: маршрут `/groups`, на который переходит сайдбар (Task 1) и на который редиректит `/courses`.

- [ ] **Step 1: Создать `frontend/src/app/(dashboard)/groups/page.tsx`**

Это адаптация текущего `courses/page.tsx`: тот же паттерн пагинации/поиска/вкладок «Активные/Архив», но список отфильтрован до групповых курсов (`student_id == null`), без кнопки и веток создания индивидуального курса, без колонок «Тип»/«Ученик» (они несут смысл только когда в списке смешаны оба типа — здесь всегда группа).

```tsx
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
    const { type, student_id, student_ids, started_at, ended_at, ...rest } = values
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

    const course = await createCourse.mutateAsync({
      ...payload,
      student_id: type === 'individual' && student_id ? student_id : undefined,
    })
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
        mode="group"
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
```

- [ ] **Step 2: Заменить `frontend/src/app/(dashboard)/courses/page.tsx` на редирект**

```tsx
import { redirect } from 'next/navigation'

export default function CoursesPage() {
  redirect('/groups')
}
```

Это серверный компонент (без `'use client'`): `redirect()` из `next/navigation` в App Router отдаёт настоящий HTTP-редирект, без лишнего клиентского JS и мигания страницы.

- [ ] **Step 3: Проверить типы и линт**

```bash
cd frontend && npx tsc --noEmit && npm run lint
```
Ожидается: без ошибок.

- [ ] **Step 4: Ручной smoke — проверить редирект через HTTP**

Браузера в окружении нет (см. CLAUDE.md), но `redirect()` — это настоящий HTTP-статус, curl его видит:

```bash
cd frontend && npm run dev &
sleep 3
curl -sI http://localhost:3000/courses | head -5
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:3000/groups
kill %1
```

Ожидается: первая команда показывает `HTTP/1.1 307 Temporary Redirect` (или 308) и `location: /groups`; вторая печатает `200`.

- [ ] **Step 5: Commit**

```bash
git add "frontend/src/app/(dashboard)/groups/page.tsx" "frontend/src/app/(dashboard)/courses/page.tsx"
git commit -m "feat(groups): /groups — список только групповых курсов, /courses — редирект"
```

---

## Task 4: CourseForm и схема — убрать ветку индивидуального курса

**Files:**
- Modify: `frontend/src/schemas/course.ts`
- Modify: `frontend/src/components/courses/CourseForm.tsx`
- Modify: `frontend/src/app/(dashboard)/groups/page.tsx` (создан в Task 3 — упростить `handleSubmit`, убрать проп `mode`)
- Modify: `frontend/src/app/(dashboard)/courses/[id]/page.tsx:115-116` (деструктуризация `values` больше не содержит `type`)

**Interfaces:**
- Consumes: файл, созданный в Task 3 (`groups/page.tsx`), и существующий `courses/[id]/page.tsx`.
- Produces: `CourseFormValues` без поля `type`; `CourseFormProps` без поля `mode` — конечный контракт для всех потребителей `CourseForm`.

Важно: этот порядок (сначала роутинг в Task 3, потом упрощение формы) — не то же самое, что удалить branch раньше. Редактирование существующего курса (`courses/[id]/page.tsx`) и до, и после этой задачи не показывает пикер ученика вовсе — обе ветки (individual и group) в `CourseForm` рендерятся только при `!initial`, так что убрать individual-ветку **не меняет** экран редактирования, только форму создания на `/groups`.

- [ ] **Step 1: Убрать `type` и `superRefine` из `frontend/src/schemas/course.ts`**

```ts
import { z } from 'zod'

export const courseSchema = z.object({
  student_id:        z.string().optional(),
  student_ids:       z.array(z.string()).optional(),
  subject:           z.string().min(2, 'Минимум 2 символа'),
  price_per_cycle:   z
    .number({ error: 'Введите число' })
    .min(0, 'Не может быть отрицательной'),
  lessons_per_cycle: z
    .number({ error: 'Введите число' })
    .int('Только целое число')
    .min(1, 'Минимум 1 урок'),
  started_at: z.string().min(1, 'Выберите дату начала'),
  ended_at:   z.string().optional(),
})

export type CourseFormValues = z.infer<typeof courseSchema>
```

- [ ] **Step 2: Упростить `frontend/src/components/courses/CourseForm.tsx`**

Убрать `mode`-проп, `courseType`/`individual`-стейт и ветку индивидуального создания целиком:

```tsx
'use client'

import { useEffect, useState } from 'react'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'
import { X } from 'lucide-react'

import { courseSchema, CourseFormValues } from '@/schemas/course'
import { Course, Student, ApiError } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { StudentCombobox, studentName } from '@/components/students/StudentCombobox'
import { SubjectCombobox } from '@/components/courses/SubjectCombobox'

interface CourseFormProps {
  open: boolean
  onClose: () => void
  onSubmit: (data: CourseFormValues) => Promise<void>
  initial?: Course
}

export function CourseForm({ open, onClose, onSubmit, initial }: CourseFormProps) {
  const [picked, setPicked] = useState<Student[]>([])

  const {
    register,
    handleSubmit,
    reset,
    watch,
    control,
    formState: { errors, isSubmitting },
  } = useForm<CourseFormValues>({
    resolver: zodResolver(courseSchema),
    defaultValues: { subject: '', lessons_per_cycle: 1, started_at: '', ended_at: '' },
  })

  const pricePerCycle   = watch('price_per_cycle')
  const lessonsPerCycle = watch('lessons_per_cycle')
  const pricePerLesson  = lessonsPerCycle > 0 ? pricePerCycle / lessonsPerCycle : 0

  useEffect(() => {
    setPicked([])
    if (initial) {
      reset({
        student_id:        initial.student_id ?? undefined,
        subject:           initial.subject,
        price_per_cycle:   initial.price_per_cycle,
        lessons_per_cycle: initial.lessons_per_cycle,
        started_at:        initial.started_at.slice(0, 10),
        ended_at:          initial.ended_at?.slice(0, 10) ?? '',
      })
    } else {
      reset({ subject: '', lessons_per_cycle: 1, started_at: '', ended_at: '' })
    }
  }, [initial, open, reset])

  function addStudent(student: Student | null) {
    if (!student || picked.some((s) => s.id === student.id)) return
    setPicked([...picked, student])
  }

  async function submit(values: CourseFormValues) {
    try {
      await onSubmit({ ...values, student_ids: picked.map((s) => s.id) })
      onClose()
    } catch (err) {
      const e = err as ApiError
      toast.error(e.message ?? 'Ошибка сохранения')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{initial ? 'Редактировать курс' : 'Новая группа'}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit(submit)} className="space-y-4 pt-2">
          {!initial && (
            <div className="space-y-1.5">
              <Label>Ученики</Label>
              <StudentCombobox value={null} onChange={addStudent} placeholder="Добавить ученика" />
              {picked.length > 0 && (
                <div className="flex flex-wrap gap-1.5 pt-1">
                  {picked.map((s) => (
                    <button
                      key={s.id}
                      type="button"
                      onClick={() => setPicked(picked.filter((p) => p.id !== s.id))}
                      className="flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs hover:bg-muted/70"
                    >
                      {studentName(s)}
                      <X className="size-3" />
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          <div className="space-y-1.5">
            <Label>Предмет</Label>
            <Controller
              name="subject"
              control={control}
              render={({ field }) => (
                <SubjectCombobox value={field.value ?? ''} onChange={field.onChange} />
              )}
            />
            {errors.subject && (
              <p className="text-xs text-destructive">{errors.subject.message}</p>
            )}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="price_per_cycle">Цена за цикл (₸)</Label>
              <Input
                id="price_per_cycle"
                type="number"
                min={1}
                step="any"
                {...register('price_per_cycle', { valueAsNumber: true })}
              />
              {errors.price_per_cycle && (
                <p className="text-xs text-destructive">{errors.price_per_cycle.message}</p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="lessons_per_cycle">Уроков в цикле</Label>
              <Input
                id="lessons_per_cycle"
                type="number"
                min={1}
                step={1}
                {...register('lessons_per_cycle', { valueAsNumber: true })}
              />
              {errors.lessons_per_cycle && (
                <p className="text-xs text-destructive">{errors.lessons_per_cycle.message}</p>
              )}
            </div>
          </div>
          {pricePerLesson > 0 && (
            <p className="text-xs text-muted-foreground">
              = {Math.round(pricePerLesson).toLocaleString()} ₸ за урок
            </p>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="started_at">Дата начала</Label>
              <Input id="started_at" type="date" {...register('started_at')} />
              {errors.started_at && (
                <p className="text-xs text-destructive">{errors.started_at.message}</p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="ended_at">
                Дата окончания{' '}
                <span className="text-muted-foreground font-normal">(необязательно)</span>
              </Label>
              <Input id="ended_at" type="date" {...register('ended_at')} />
            </div>
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="outline" onClick={onClose}>
              Отмена
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 3: Обновить `handleSubmit` и вызов `CourseForm` в `frontend/src/app/(dashboard)/groups/page.tsx`**

```tsx
// было:
  async function handleSubmit(values: CourseFormValues) {
    const { type, student_id, student_ids, started_at, ended_at, ...rest } = values
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

    const course = await createCourse.mutateAsync({
      ...payload,
      student_id: type === 'individual' && student_id ? student_id : undefined,
    })
    if (student_ids && student_ids.length > 0) {
      try {
        await addEnrollments.mutateAsync({ courseId: course.id, studentIds: student_ids })
      } catch {
        toast.error('Группа создана, но учеников записать не удалось')
      }
    }
    toast.success('Группа создана')
  }

// стало:
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
```

И убрать проп `mode` у рендера формы в том же файле:

```tsx
// было:
      <CourseForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSubmit={handleSubmit}
        initial={editing}
        mode="group"
      />

// стало:
      <CourseForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSubmit={handleSubmit}
        initial={editing}
      />
```

- [ ] **Step 4: Поправить деструктуризацию в `frontend/src/app/(dashboard)/courses/[id]/page.tsx`**

```tsx
// было (строка ~115-116):
  async function handleUpdateCourse(values: CourseFormValues) {
    const { type: _type, student_id: _sid, started_at, ended_at, ...rest } = values

// стало:
  async function handleUpdateCourse(values: CourseFormValues) {
    const { student_id: _sid, started_at, ended_at, ...rest } = values
```

Вызов `<CourseForm .../>` в этом файле проп `mode` никогда не передавал — менять там больше нечего.

- [ ] **Step 5: Проверить типы и линт**

```bash
cd frontend && npx tsc --noEmit && npm run lint
```
Ожидается: без ошибок — это единственная проверка, которая ловит рассинхрон полей формы/схемы (тестов на фронте нет).

- [ ] **Step 6: Ручной smoke — полная сборка**

```bash
cd frontend && npm run build
```
Ожидается: сборка проходит (Next.js прогоняет полный тайпчек по всем роутам разом, включая `/groups` и `/courses`) — это финальная проверка того, что старый `/courses/[id]/page.tsx` и новый `/groups/page.tsx` согласованы с урезанной формой.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/schemas/course.ts frontend/src/components/courses/CourseForm.tsx \
        "frontend/src/app/(dashboard)/groups/page.tsx" "frontend/src/app/(dashboard)/courses/[id]/page.tsx"
git commit -m "refactor(courses): убрать ветку индивидуального курса из CourseForm и схемы"
```

---

## Что сознательно не входит в эту фазу (и почему)

- Плашка «N курсов» и ссылка «Курсы →» на дашборде (`dashboard/page.tsx:134,222`), ссылка «Перейти к курсам» в `payments/page.tsx:183`, кнопка «← Курсы» на `courses/[id]/page.tsx:281-283` — используют слово «курс», но это не навигация (критерий приёмки №1 — конкретно про сайдбар), и спека их не называет среди файлов фазы 4. Технически рабочие ссылки на `/courses` продолжат работать через редирект из Task 3.
- Пересчёт `total`/`totalPages` на `/groups` под реальное число групп (а не всех курсов тьютора) — спека прямо разрешает текущее приближение, пока курсов десятки (см. `ponytail:`-комментарий в Task 3).
