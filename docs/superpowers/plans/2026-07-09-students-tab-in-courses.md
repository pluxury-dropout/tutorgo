# Вкладка «Ученики» на странице курсов — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Список учеников становится вкладкой между «Активные» и «Архив» на странице курсов; отдельный пункт «Ученики» из навигации убирается.

**Architecture:** `StudentsList` — новый презентационный компонент (рендерит строки). Страница курсов держит оркестрацию (данные/поиск/пагинация/форма) ради динамического заголовка. Активная вкладка — в URL (`?tab=`), поиск учеников — в локальном стейте.

**Tech Stack:** Next.js (App Router, client components), React Query (`useStudentsPaged`), существующие примитивы `PageHeader`/`SectionCard`/`Pagination`/`StudentForm`.

## Global Constraints

- Нет тестового харнеса для React-компонентов. Верификация каждой задачи: `npx tsc --noEmit` (0 ошибок) + `npm run build` (успех) + ручная проверка в `npm run dev`. Не добавлять vitest/jest (YAGNI).
- Существующие визуальные токены и паттерны (`var(--...)`, инлайн-стили строк) сохранять как есть — переносить разметку дословно.
- Русскоязычные подписи UI.
- Рабочая директория команд: `frontend/`.

**Спека:** `docs/superpowers/specs/2026-07-09-students-tab-in-courses-design.md`

---

## File Structure

- **Create** `frontend/src/components/students/StudentsList.tsx` — презентационный список учеников (строки в `SectionCard`).
- **Modify** `frontend/src/app/(dashboard)/courses/page.tsx` — третья вкладка, URL-стейт вкладки, динамический заголовок, студенческие хуки/стейт/форма, переключение источника поиска.
- **Modify** `frontend/src/app/(dashboard)/students/page.tsx` — заменить на `redirect('/courses?tab=students')`.
- **Modify** `frontend/src/components/layout/Sidebar.tsx` — убрать пункт «Ученики».
- **Modify** `frontend/src/components/layout/MobileBottomNav.tsx` — убрать пункт «Ученики».

---

### Task 1: Презентационный компонент `StudentsList`

**Files:**
- Create: `frontend/src/components/students/StudentsList.tsx`

**Interfaces:**
- Produces:
  ```ts
  interface StudentsListProps {
    students: Student[]
    onEdit: (s: Student) => void
    onDelete: (s: Student) => void
  }
  export function StudentsList(props: StudentsListProps): JSX.Element
  ```
  Строку по клику ведёт на `/students/${id}` внутри компонента (через `useRouter`).

- [ ] **Step 1: Создать файл компонента**

Разметка строк переносится дословно из текущего `students/page.tsx` (строки 126-179), меняются только источники: `students` из props, `openEdit`→`onEdit`, `handleDelete`→`onDelete`.

```tsx
'use client'

import { useRouter } from 'next/navigation'
import { Pencil, Trash2, ChevronRight } from 'lucide-react'
import { SectionCard } from '@/components/common/SectionCard'
import { Button } from '@/components/ui/button'
import { Student } from '@/types/api'

interface StudentsListProps {
  students: Student[]
  onEdit: (s: Student) => void
  onDelete: (s: Student) => void
}

export function StudentsList({ students, onEdit, onDelete }: StudentsListProps) {
  const router = useRouter()

  return (
    <SectionCard>
      {students.map((student, i) => (
        <div
          key={student.id}
          style={{
            display: 'grid',
            gridTemplateColumns: 'minmax(0, 1fr) auto',
            alignItems: 'center',
            gap: 16,
            padding: '10px 18px',
            borderTop: i === 0 ? 'none' : '1px solid var(--row-border)',
            cursor: 'pointer',
          }}
          className="hover:bg-muted/30 group"
          role="button"
          tabIndex={0}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); router.push(`/students/${student.id}`) } }}
          onClick={() => router.push(`/students/${student.id}`)}
        >
          <div style={{ minWidth: 0 }}>
            <div style={{
              fontSize: 14, fontWeight: 600, color: 'var(--foreground)',
              overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
            }}>
              {student.first_name}{student.last_name ? ` ${student.last_name}` : ''}
            </div>
            {student.email && (
              <div style={{
                fontSize: 12.5, color: 'var(--muted-foreground)', marginTop: 1,
                overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
              }}>
                {student.email}
              </div>
            )}
          </div>
          <div className="flex items-center gap-3 shrink-0">
            <span style={{ fontSize: 13, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
              {student.phone || '—'}
            </span>
            <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity duration-150" />
            <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
              <Button size="icon" variant="ghost" className="h-8 w-8" onClick={() => onEdit(student)}>
                <Pencil className="h-3.5 w-3.5" />
              </Button>
              <Button size="icon" variant="ghost"
                className="h-8 w-8 text-destructive hover:text-destructive"
                onClick={() => onDelete(student)}>
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        </div>
      ))}
    </SectionCard>
  )
}
```

- [ ] **Step 2: Проверить типы**

Run: `cd frontend && npx tsc --noEmit`
Expected: 0 ошибок.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/students/StudentsList.tsx
git commit -m "feat(students): extract presentational StudentsList component"
```

---

### Task 2: Вкладка «Ученики» на странице курсов

**Files:**
- Modify: `frontend/src/app/(dashboard)/courses/page.tsx`

**Interfaces:**
- Consumes: `StudentsList` из Task 1; хуки `useStudentsPaged`, `useCreateStudent`, `useUpdateStudent`, `useDeleteStudent` из `@/lib/hooks/useStudents`; `StudentForm` из `@/components/students/StudentForm`; `StudentFormValues` из `@/schemas/student`; тип `Student`.

Ключевые правки. `LIMIT` (20) переиспользуем для учеников.

- [ ] **Step 1: Импорты**

Добавить в блок импортов:

```tsx
import { useStudents, useStudentsPaged, useCreateStudent, useUpdateStudent, useDeleteStudent } from '@/lib/hooks/useStudents'
import { StudentForm } from '@/components/students/StudentForm'
import { StudentsList } from '@/components/students/StudentsList'
import { StudentFormValues } from '@/schemas/student'
import { Course, Student } from '@/types/api'
import { Users } from 'lucide-react'
```

(строка 9 `import { useStudents } ...` заменяется расширенным импортом выше; строка 17 `import { Course }` → `import { Course, Student }`; в строке 6 к иконкам добавить `Users`.)

- [ ] **Step 2: Тип вкладки + инициализация из URL**

Заменить строку 73:

```tsx
const tabParam = searchParams.get('tab')
const [tab, setTab] = useState<'active' | 'students' | 'archive'>(
  tabParam === 'students' ? 'students' : tabParam === 'archive' ? 'archive' : 'active'
)
```

- [ ] **Step 3: Стейт и хуки учеников**

Добавить рядом с курсовым стейтом (после строки 74 `archivePage`):

```tsx
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
```

- [ ] **Step 4: Синхронизация вкладки в URL**

Добавить функцию (рядом с `handlePageChange`):

```tsx
function changeTab(next: 'active' | 'students' | 'archive') {
  setTab(next)
  const p = new URLSearchParams(searchParams.toString())
  if (next === 'active') p.delete('tab')
  else p.set('tab', next)
  router.replace(`/courses?${p}`)
}
```

- [ ] **Step 5: Динамический заголовок (три ветки)**

Заменить `<PageHeader ...>` (строки 136-150):

```tsx
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
```

- [ ] **Step 6: Три кнопки-вкладки**

Заменить массив вкладок и подпись в свитчере (строки 154 и 170):

```tsx
{(['active', 'students', 'archive'] as const).map((t) => (
```
и подпись:
```tsx
{t === 'active' ? 'Активные' : t === 'students' ? 'Ученики' : 'Архив'}
```
а `onClick={() => setTab(t)}` → `onClick={() => changeTab(t)}`.

- [ ] **Step 7: Переключение источника поля поиска**

Заменить поле поиска (строки 175-182), чтобы на вкладке учеников оно писало в `studentSearch`:

```tsx
<div className="mb-4">
  <Input
    placeholder={tab === 'students' ? 'Поиск по имени или email...' : 'Поиск по предмету...'}
    value={tab === 'students' ? studentSearch : localSearch}
    onChange={(e) => tab === 'students' ? setStudentSearch(e.target.value) : setLocalSearch(e.target.value)}
    className="max-w-sm"
  />
</div>
```

- [ ] **Step 8: Рендер контента вкладки учеников**

Обернуть существующие ветки: сейчас `{tab === 'active' ? (...) : (...архив...)}`. Сделать три ветки. Перед активной веткой добавить students, затем оставить active/archive как есть:

```tsx
{tab === 'students' ? (
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
  /* ...существующая активная ветка без изменений... */
```

(Существующий тернарник `tab === 'active' ? (...) : (...)` становится вложенной `: tab === 'active' ? (...) : (...archive...)`. Архивную ветку не трогаем.)

- [ ] **Step 9: Форма ученика**

Перед закрытием компонента (рядом с `<CourseForm ... />`) добавить:

```tsx
<StudentForm
  open={studentFormOpen}
  onClose={() => setStudentFormOpen(false)}
  onSubmit={handleStudentSubmit}
  initial={editingStudent}
/>
```

- [ ] **Step 10: Типы + сборка**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: 0 ошибок типов, сборка успешна.

- [ ] **Step 11: Ручная проверка**

Run: `cd frontend && npm run dev`, открыть `/courses`.
Expected:
- Три вкладки «Активные · Ученики · Архив». На «Ученики» — заголовок «Ученики», метрика «N учеников» (фиолетовая), кнопка «Добавить».
- Клик по вкладке меняет `?tab=` в адресной строке.
- Поиск на вкладке учеников фильтрует список и меняет число в заголовке; переключение на «Активные» не ломает URL курсов.
- Создание/редактирование/удаление ученика работает; клик по строке ведёт на `/students/{id}`.

- [ ] **Step 12: Commit**

```bash
git add frontend/src/app/\(dashboard\)/courses/page.tsx
git commit -m "feat(courses): add students tab between active and archive"
```

---

### Task 3: Убрать пункт «Ученики» из навигации + редирект старого роута

**Files:**
- Modify: `frontend/src/components/layout/Sidebar.tsx:331`
- Modify: `frontend/src/components/layout/MobileBottomNav.tsx:10`
- Modify: `frontend/src/app/(dashboard)/students/page.tsx`

- [ ] **Step 1: Убрать из сайдбара**

Удалить строку 331 (`{ href: '/students', label: 'Ученики', icon: Users },`). Если `Users` больше нигде в файле не используется — убрать из импорта `lucide-react` (проверить `grep Users Sidebar.tsx`).

- [ ] **Step 2: Убрать из мобильной навигации**

Удалить строку 10 (`{ href: '/students', label: 'Ученики', Icon: Users },`) в `MobileBottomNav.tsx`. Аналогично проверить импорт `Users`.

- [ ] **Step 3: Заменить старый роут на редирект**

Полностью заменить содержимое `frontend/src/app/(dashboard)/students/page.tsx`:

```tsx
import { redirect } from 'next/navigation'

export default function StudentsPage() {
  redirect('/courses?tab=students')
}
```

(Это server component — убираем `'use client'` и весь прежний код списка; он переехал в Task 1/2.)

- [ ] **Step 4: Типы + сборка**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: 0 ошибок, сборка успешна.

- [ ] **Step 5: Ручная проверка**

Run: dev-сервер.
Expected:
- В сайдбаре и в мобильной нижней навигации пункта «Ученики» нет.
- Открытие `/students` редиректит на `/courses?tab=students` и показывает вкладку «Ученики».

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/layout/Sidebar.tsx frontend/src/components/layout/MobileBottomNav.tsx frontend/src/app/\(dashboard\)/students/page.tsx
git commit -m "feat(nav): drop Students nav item, redirect /students to courses tab"
```

---

## Self-Review

**Spec coverage:**
- Вкладки Активные·Ученики·Архив → Task 2 (Steps 6, 8). ✓
- Изоляция `StudentsList` презентационный → Task 1. ✓
- Динамический заголовок → Task 2 Step 5. ✓
- Вкладка в URL, поиск учеников локально → Task 2 Steps 2, 4, 7. ✓
- Редирект `/students` → Task 3 Step 3. ✓
- Убрать пункт из сайдбара + мобильной навигации → Task 3 Steps 1-2. ✓
- Карточку `/students/[id]` не трогаем → нигде не модифицируется. ✓

**Placeholder scan:** нет TBD/TODO; весь код приведён.

**Type consistency:** `StudentsList` props (`students`/`onEdit`/`onDelete`) совпадают между Task 1 (Produces) и вызовом в Task 2 Step 8. Хуки `useStudentsPaged`/`useCreate/Update/DeleteStudent` — те же, что в удаляемом `students/page.tsx`, сигнатуры не меняются.
