# Student Cabinet Frontend v1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Кабинет ученика: invite → аккаунт → login, список уроков с позицией в оплаченном цикле, вход в звонок+доску, профиль со сменой пароля; плюс кнопка приглашения у репетитора и уборка мёртвого гостевого входа.

**Architecture:** Изолированный student-auth-стек (свой axios-инстанс `studentHttp` c ключами `tg_student_token`/`tg_student_user` и refresh на `/student/auth/refresh`) рядом с нетронутым tutor-стеком. Страницы в `app/student/` с тремя route-group: `(auth)` — логин/инвайт, `(cabinet)` — гейт + шапка, `(room)` — гейт без шапки для звонка. Звонок/доска переиспользуют `CallRoom` (role="guest"; доска приходит по LiveKit DataChannel от репетитора — board-token эндпоинт фронту не нужен). Backend меняется в одном месте: `ListLessons` ученика обогащается `cycle_position`/`cycle_size` (rank в SQL + платежи через `cyclePositionFromRank`).

**Tech Stack:** Next.js (app router), axios, zustand, @tanstack/react-query, react-hook-form + zod, LiveKit, sonner; Go/Gin/pgx на backend.

## Global Constraints

- Tutor-стек не трогаем: `lib/api/client.ts`, `stores/auth.ts`, ключи `tg_token`/`tg_user` остаются как есть (исключения — только явные правки Task 8/9).
- localStorage-ключи ученика: строго `tg_student_token`, `tg_student_user`.
- В student-клиенте НЕТ 402-ветки (подписка — забота репетитора).
- Все UI-тексты — на русском; UI-примитивы из `components/ui/*` и `components/common/SectionCard`.
- Проверка каждой frontend-задачи: `cd frontend && npx tsc --noEmit` — зелёный.
- Проверка backend-задачи: `go build ./... && go vet ./... && make test` — зелёные.
- Коммит после каждой задачи.

## Волны исполнения (для параллельных субагентов)

- **Волна 1 (последовательно):** Task 1 → Task 2 → Task 3.
- **Волна 2 (параллельно, файлы не пересекаются):** Task 4, 5, 6, 7, 8, 9.
- **Волна 3:** Task 10 (финальная сборка + smoke).

---

### Task 1: Backend — cycle_position/cycle_size в уроках ученика

**Files:**
- Modify: `repository/student.go` (метод `ListLessons`, ~строки 175–209)
- Modify: `service/student.go` (struct `studentService`, `NewStudentService`, метод `ListLessons`)
- Modify: `router/router.go:41`
- Test: `service/student_test.go`

**Interfaces:**
- Consumes: `cyclePositionFromRank(rank int, payments []models.Payment) (position, size int)` из `service/lesson.go:49` (тот же пакет); `paymentRepo.GetByCoursesBatch(ctx, courseIDs []string) (map[string][]models.Payment, error)`.
- Produces: `GET /student/lessons` теперь возвращает `cycle_position`/`cycle_size` в элементах (`omitempty` — контракт не ломается). Сигнатура `NewStudentService(repo repository.StudentRepository, paymentRepo repository.PaymentRepository)`.

- [ ] **Step 1: Написать падающий тест**

В `service/student_test.go` добавить (в конец файла). `mockPaymentRepo` уже существует в `service/payment_test.go` (тот же пакет `service_test`) — переиспользуем:

```go
func TestStudentListLessons_CyclePositions(t *testing.T) {
	repo := new(mockStudentRepo)
	payRepo := new(mockPaymentRepo)
	svc := service.NewStudentService(repo, payRepo)

	rank3, rank9 := 3, 9
	lessons := []models.CalendarLesson{
		{ID: "l1", CourseID: "c1", Rank: &rank3},
		{ID: "l2", CourseID: "c1", Rank: &rank9},
		{ID: "l3", CourseID: "c2"}, // rank нет (все уроки отменены) — цикл не считаем
	}
	repo.On("ListLessons", mock.Anything, "stu-1", false).Return(lessons, nil)
	payRepo.On("GetByCoursesBatch", mock.Anything, []string{"c1"}).Return(map[string][]models.Payment{
		"c1": {{LessonsCount: 8}, {LessonsCount: 8}},
	}, nil)

	got, err := svc.ListLessons(context.Background(), "stu-1", false)

	assert.NoError(t, err)
	assert.Equal(t, 3, *got[0].CyclePosition) // 3-й урок первого пакета из 8
	assert.Equal(t, 8, *got[0].CycleSize)
	assert.Equal(t, 1, *got[1].CyclePosition) // 9-й урок = 1-й второго пакета
	assert.Nil(t, got[2].CyclePosition)
	repo.AssertExpectations(t)
	payRepo.AssertExpectations(t)
}
```

Одновременно обновить существующие вызовы конструктора в этом файле (иначе не скомпилируется):

```bash
sed -i 's/service\.NewStudentService(repo)/service.NewStudentService(repo, new(mockPaymentRepo))/' service/student_test.go
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./service/ -run TestStudentListLessons_CyclePositions`
Expected: FAIL — компиляция упадёт на `NewStudentService` (лишний аргумент) — это и есть красный шаг.

- [ ] **Step 3: Service — paymentRepo + обогащение**

В `service/student.go` заменить struct и конструктор:

```go
type studentService struct {
	repo        repository.StudentRepository
	paymentRepo repository.PaymentRepository
}

func NewStudentService(repo repository.StudentRepository, paymentRepo repository.PaymentRepository) StudentService {
	return &studentService{repo: repo, paymentRepo: paymentRepo}
}
```

Заменить метод `ListLessons` (был простым пробросом):

```go
func (s *studentService) ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error) {
	lessons, err := s.repo.ListLessons(ctx, studentID, past)
	if err != nil {
		return nil, err
	}
	// Циклы считаются от платежей — как в tutor-календаре (GetCalendar).
	seen := map[string]bool{}
	courseIDs := []string{}
	for _, l := range lessons {
		if l.Rank != nil && !seen[l.CourseID] {
			seen[l.CourseID] = true
			courseIDs = append(courseIDs, l.CourseID)
		}
	}
	if len(courseIDs) == 0 {
		return lessons, nil
	}
	paymentsMap, err := s.paymentRepo.GetByCoursesBatch(ctx, courseIDs)
	if err != nil {
		return nil, err
	}
	for i, l := range lessons {
		if l.Rank == nil {
			continue
		}
		coursePayments := paymentsMap[l.CourseID]
		if len(coursePayments) == 0 {
			continue
		}
		pos, size := cyclePositionFromRank(*l.Rank, coursePayments)
		if pos > 0 {
			p, sz := pos, size
			lessons[i].CyclePosition = &p
			lessons[i].CycleSize = &sz
		}
	}
	return lessons, nil
}
```

- [ ] **Step 4: Repo — rank в SQL**

В `repository/student.go` заменить тело `ListLessons`:

```go
func (r *studentRepository) ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error) {
	// enrollment-джойн идентичен EnrolledInLesson: индивидуальный курс (c.student_id)
	// ИЛИ групповой через course_enrollments. Фильтр активности курса намеренно
	// опущен — ученик видит все свои уроки, включая архивные (история).
	// rank считается по ВСЕМ неотменённым урокам курса (без date-фильтра),
	// иначе позиция в цикле зависела бы от выбранной вкладки.
	base := `WITH stu_courses AS MATERIALIZED (
	           SELECT c.id FROM courses c
	           WHERE c.student_id = $1
	              OR EXISTS (SELECT 1 FROM course_enrollments ce
	                         WHERE ce.course_id = c.id AND ce.student_id = $1)
	         ),
	         ranked AS (
	           SELECT l.id,
	                  ROW_NUMBER() OVER (PARTITION BY l.course_id ORDER BY l.scheduled_at)::int AS rank
	           FROM lessons l
	           WHERE l.course_id IN (SELECT id FROM stu_courses)
	             AND l.status != 'cancelled'
	         )
	         SELECT l.id, l.course_id, l.scheduled_at, l.duration_minutes, l.status,
	                l.notes, c.subject, s.first_name, (c.student_id IS NULL) AS is_group,
	                r.rank
	         FROM lessons l
	         JOIN courses c ON c.id = l.course_id
	         LEFT JOIN students s ON s.id = c.student_id
	         LEFT JOIN ranked r ON r.id = l.id
	         WHERE l.course_id IN (SELECT id FROM stu_courses)`
	var q string
	if past {
		q = base + ` AND l.scheduled_at < now() ORDER BY l.scheduled_at DESC`
	} else {
		q = base + ` AND l.scheduled_at >= now() ORDER BY l.scheduled_at ASC`
	}
	rows, err := r.conn.Query(ctx, q, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lessons := []models.CalendarLesson{}
	for rows.Next() {
		var l models.CalendarLesson
		if err := rows.Scan(&l.ID, &l.CourseID, &l.ScheduledAt, &l.DurationMinutes,
			&l.Status, &l.Notes, &l.Subject, &l.StudentName, &l.IsGroup, &l.Rank); err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}
	return lessons, rows.Err()
}
```

- [ ] **Step 5: Router wiring**

В `router/router.go:41` заменить:

```go
studentService := service.NewStudentService(studentRepo, paymentRepo)
```

(`paymentRepo` уже создаётся выше в этой же функции — `repository.NewPaymentRepository(pool)`.)

- [ ] **Step 6: Прогнать проверки**

Run: `go build ./... && go vet ./... && make test`
Expected: всё зелёное, включая `TestStudentListLessons_CyclePositions`.

- [ ] **Step 7: Commit**

```bash
git add repository/student.go service/student.go router/router.go service/student_test.go
git commit -m "feat(student): cycle position/size in student lessons list"
```

---

### Task 2: Frontend-ядро — studentClient, store, API-модуль

**Files:**
- Create: `frontend/src/lib/api/studentClient.ts`
- Create: `frontend/src/lib/api/student.ts`
- Create: `frontend/src/stores/studentAuth.ts`

**Interfaces:**
- Consumes: `ApiError`, `CalendarLesson` из `@/types/api`; `RoomTokenResponse` из `./calls`.
- Produces (используют Task 3–7): `studentHttp` (axios instance), `forceStudentLogout()`, `useStudentAuthStore` (`{ token, user, setAuth(token, user), clearAuth() }`), `studentApi` c методами `acceptInvite`, `login`, `logout`, `me`, `lessons`, `changePassword`, `roomToken`; типы `StudentProfile`, `LessonsFilter`.

- [ ] **Step 1: Создать `frontend/src/lib/api/studentClient.ts`**

```ts
import axios, { AxiosError } from 'axios'
import { ApiError } from '@/types/api'

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'

// Изолированный клиент кабинета ученика: свой токен (tg_student_token) и свой
// refresh (/student/auth/refresh). Логика — упрощённая копия tutor-клиента
// (client.ts) без 402-ветки; tutor-стек не трогаем, чтобы оба логина жили в
// одном браузере независимо.
export const studentHttp = axios.create({
  baseURL: BASE_URL,
  headers: { 'Content-Type': 'application/json' },
})

function getTokenExp(token: string): number | null {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    return typeof payload.exp === 'number' ? payload.exp : null
  } catch {
    return null
  }
}

let isRefreshing = false
let refreshPromise: Promise<void> | null = null
// После жёсткого провала refresh не долбим /student/auth/refresh на каждый
// запрос (иначе rate limiter выдаст 429). Сбрасывается перезагрузкой страницы.
let refreshFailed = false

export function forceStudentLogout(): void {
  localStorage.removeItem('tg_student_token')
  localStorage.removeItem('tg_student_user')
  if (typeof window !== 'undefined') window.location.href = '/student/login'
}

async function refreshToken(): Promise<string> {
  const { data } = await axios.post<{ access_token: string }>(
    `${BASE_URL}/student/auth/refresh`,
    {},
    { withCredentials: true },
  )
  localStorage.setItem('tg_student_token', data.access_token)
  return data.access_token
}

async function proactiveRefresh(): Promise<void> {
  if (isRefreshing || refreshFailed) return
  isRefreshing = true
  const p = refreshToken().then(
    () => undefined,
    (err: AxiosError) => {
      refreshFailed = true
      if (err.response?.status === 401) forceStudentLogout()
    },
  )
  refreshPromise = p
  try {
    await p
  } finally {
    isRefreshing = false
    refreshPromise = null
  }
}

studentHttp.interceptors.request.use(async (config) => {
  const token =
    typeof window !== 'undefined' ? localStorage.getItem('tg_student_token') : null
  if (token) {
    const exp = getTokenExp(token)
    // refresh, если осталось меньше 7 дней
    if (exp && exp - Date.now() / 1000 < 7 * 24 * 60 * 60) {
      await proactiveRefresh()
      const fresh = localStorage.getItem('tg_student_token')
      config.headers.Authorization = `Bearer ${fresh ?? token}`
    } else {
      config.headers.Authorization = `Bearer ${token}`
    }
  }
  return config
})

studentHttp.interceptors.response.use(
  (r) => r,
  async (error: AxiosError<{ error: string } | Record<string, string>>) => {
    // /student/password возвращает 401 при неверном старом пароле — это НЕ
    // протухший токен, refresh-retry зациклился бы. Отдаём 401 форме как есть.
    const skipRefresh =
      error.config?.url?.startsWith('/student/auth/') ||
      error.config?.url === '/student/password'

    if (error.response?.status === 401 && !skipRefresh && !isRefreshing) {
      isRefreshing = true
      const p = refreshToken()
      refreshPromise = p.then(() => undefined)
      try {
        const freshToken = await p
        isRefreshing = false
        refreshPromise = null
        if (error.config) {
          error.config.headers = error.config.headers ?? {}
          error.config.headers['Authorization'] = `Bearer ${freshToken}`
          return studentHttp.request(error.config)
        }
      } catch {
        isRefreshing = false
        refreshPromise = null
        forceStudentLogout()
      }
    }

    const status = error.response?.status ?? 0
    const data = error.response?.data

    let normalized: ApiError
    if (data && typeof data === 'object' && 'error' in data) {
      normalized = { message: data.error as string, status }
    } else if (data && typeof data === 'object') {
      normalized = {
        message: 'Validation error',
        fieldErrors: data as Record<string, string>,
        status,
      }
    } else {
      normalized = { message: 'Unknown error', status }
    }

    return Promise.reject(normalized)
  },
)
```

- [ ] **Step 2: Создать `frontend/src/lib/api/student.ts`**

```ts
import { studentHttp } from './studentClient'
import type { CalendarLesson } from '@/types/api'
import type { RoomTokenResponse } from './calls'

export interface StudentProfile {
  first_name: string
  last_name: string
  phone: string
  username: string
}

export type LessonsFilter = 'upcoming' | 'past'

// withCredentials на auth-путях: backend ставит/читает refresh-cookie ученика.
export const studentApi = {
  acceptInvite: (data: { token: string; username: string; password: string }) =>
    studentHttp
      .post<{ access_token: string }>('/student/auth/accept-invite', data, { withCredentials: true })
      .then((r) => r.data),

  login: (data: { identifier: string; password: string }) =>
    studentHttp
      .post<{ access_token: string }>('/student/auth/login', data, { withCredentials: true })
      .then((r) => r.data),

  logout: () =>
    studentHttp.post('/student/auth/logout', {}, { withCredentials: true }).catch(() => {}),

  me: () => studentHttp.get<StudentProfile>('/student/me').then((r) => r.data),

  lessons: (filter: LessonsFilter) =>
    studentHttp
      .get<CalendarLesson[]>('/student/lessons', { params: { filter } })
      .then((r) => r.data),

  changePassword: (data: { old_password: string; new_password: string }) =>
    studentHttp
      .post<{ access_token: string }>('/student/password', data, { withCredentials: true })
      .then((r) => r.data),

  roomToken: (lessonId: string) =>
    studentHttp
      .post<RoomTokenResponse>(`/student/lessons/${lessonId}/room-token`)
      .then((r) => r.data),
}
```

- [ ] **Step 3: Создать `frontend/src/stores/studentAuth.ts`**

```ts
import { create } from 'zustand'
import type { StudentProfile } from '@/lib/api/student'

interface StudentAuthState {
  token: string | null
  user: StudentProfile | null
  setAuth: (token: string, user: StudentProfile) => void
  clearAuth: () => void
}

const hydrate = () => {
  if (typeof window === 'undefined') return { token: null, user: null }
  const token = localStorage.getItem('tg_student_token')
  const raw = localStorage.getItem('tg_student_user')
  const user = raw ? (JSON.parse(raw) as StudentProfile) : null
  return { token, user }
}

export const useStudentAuthStore = create<StudentAuthState>((set) => ({
  ...hydrate(),
  setAuth: (token, user) => {
    localStorage.setItem('tg_student_token', token)
    localStorage.setItem('tg_student_user', JSON.stringify(user))
    set({ token, user })
  },
  clearAuth: () => {
    localStorage.removeItem('tg_student_token')
    localStorage.removeItem('tg_student_user')
    set({ token: null, user: null })
  },
}))
```

- [ ] **Step 4: Проверка**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/api/studentClient.ts frontend/src/lib/api/student.ts frontend/src/stores/studentAuth.ts
git commit -m "feat(student-cabinet): isolated student auth stack (client, store, api)"
```

---

### Task 3: Route-группы и гейт кабинета

**Files:**
- Create: `frontend/src/components/student/StudentGate.tsx`
- Create: `frontend/src/app/student/(auth)/layout.tsx`
- Create: `frontend/src/app/student/(cabinet)/layout.tsx`
- Create: `frontend/src/app/student/(room)/layout.tsx`

**Interfaces:**
- Consumes: `useStudentAuthStore`, `studentApi` (Task 2).
- Produces: URL-пространство `/student/*`; `StudentGate` (client-гейт: нет `tg_student_token` → redirect на `/student/login?next=<путь>`); `(cabinet)` — шапка + контейнер `max-w-3xl`, `(room)` — голый гейт для fullscreen-звонка.

Next.js: группы `(auth)`/`(cabinet)`/`(room)` не попадают в URL — всё живёт под `/student/...`. Статичный `/student/lessons` (cabinet) и вложенный `/student/lessons/[id]/call` (room) не конфликтуют — resolved-пути различны.

- [ ] **Step 1: Создать `frontend/src/components/student/StudentGate.tsx`**

```tsx
'use client'

import { useEffect, useState } from 'react'
import { usePathname, useRouter } from 'next/navigation'

// Клиентский гейт кабинета ученика: без tg_student_token уводит на логин,
// сохранив целевой путь в ?next=. Протухший токен чинит interceptor.
export function StudentGate({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const pathname = usePathname()
  const [ready, setReady] = useState(false)

  useEffect(() => {
    const token = localStorage.getItem('tg_student_token')
    if (!token) {
      router.replace(`/student/login?next=${encodeURIComponent(pathname)}`)
      return
    }
    setReady(true)
  }, [router, pathname])

  if (!ready) return null
  return <>{children}</>
}
```

- [ ] **Step 2: Создать `frontend/src/app/student/(auth)/layout.tsx`**

```tsx
export default function StudentAuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen flex items-center justify-center bg-muted/40 px-4">
      <div className="w-full max-w-md">{children}</div>
    </div>
  )
}
```

- [ ] **Step 3: Создать `frontend/src/app/student/(cabinet)/layout.tsx`**

```tsx
'use client'

import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { GraduationCap, LogOut } from 'lucide-react'

import { StudentGate } from '@/components/student/StudentGate'
import { useStudentAuthStore } from '@/stores/studentAuth'
import { studentApi } from '@/lib/api/student'
import { Button } from '@/components/ui/button'

export default function StudentCabinetLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const user = useStudentAuthStore((s) => s.user)
  const clearAuth = useStudentAuthStore((s) => s.clearAuth)

  async function handleLogout() {
    await studentApi.logout()
    clearAuth()
    router.replace('/student/login')
  }

  return (
    <StudentGate>
      <div className="min-h-screen bg-background">
        <header style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="mx-auto max-w-3xl px-4 h-14 flex items-center justify-between">
            <Link href="/student/lessons" className="flex items-center gap-2 font-semibold">
              <GraduationCap className="h-5 w-5 text-primary" />
              TutorHub
            </Link>
            <div className="flex items-center gap-3">
              <Link
                href="/student/profile"
                className="text-sm text-muted-foreground hover:text-foreground"
              >
                {user ? `${user.first_name} ${user.last_name}`.trim() : 'Профиль'}
              </Link>
              <Button size="icon" variant="ghost" className="h-8 w-8" onClick={handleLogout} title="Выйти">
                <LogOut className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </header>
        <main className="mx-auto max-w-3xl px-4 py-6">{children}</main>
      </div>
    </StudentGate>
  )
}
```

- [ ] **Step 4: Создать `frontend/src/app/student/(room)/layout.tsx`**

```tsx
import { StudentGate } from '@/components/student/StudentGate'

export default function StudentRoomLayout({ children }: { children: React.ReactNode }) {
  return <StudentGate>{children}</StudentGate>
}
```

- [ ] **Step 5: Проверка**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок. (Пустые группы без page.tsx — нормально, страницы добавят Task 4–7.)

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/student/StudentGate.tsx "frontend/src/app/student/(auth)/layout.tsx" "frontend/src/app/student/(cabinet)/layout.tsx" "frontend/src/app/student/(room)/layout.tsx"
git commit -m "feat(student-cabinet): /student route groups, gate and cabinet shell"
```

---

### Task 4: Страницы входа и принятия приглашения

**Files:**
- Create: `frontend/src/schemas/studentAuth.ts`
- Create: `frontend/src/app/student/(auth)/login/page.tsx`
- Create: `frontend/src/app/student/(auth)/invite/[token]/page.tsx`

**Interfaces:**
- Consumes: `studentApi.login/acceptInvite/me`, `useStudentAuthStore.setAuth` (Task 2); layout `(auth)` (Task 3).
- Produces: `/student/login` (поле `identifier` — телефон или логин; уважает `?next=`), `/student/invite/[token]`.

- [ ] **Step 1: Создать `frontend/src/schemas/studentAuth.ts`**

```ts
import { z } from 'zod'

export const studentLoginSchema = z.object({
  identifier: z.string().min(1, 'Введите телефон или логин'),
  password: z.string().min(6, 'Минимум 6 символов'),
})

export const acceptInviteSchema = z
  .object({
    username: z
      .string()
      .min(3, 'Минимум 3 символа')
      .max(32, 'Максимум 32 символа')
      .regex(/^[a-zA-Z0-9]+$/, 'Только латинские буквы и цифры'),
    password: z.string().min(6, 'Минимум 6 символов'),
    confirm: z.string(),
  })
  .refine((d) => d.password === d.confirm, {
    path: ['confirm'],
    message: 'Пароли не совпадают',
  })

export type StudentLoginInput = z.infer<typeof studentLoginSchema>
export type AcceptInviteInput = z.infer<typeof acceptInviteSchema>
```

- [ ] **Step 2: Создать `frontend/src/app/student/(auth)/login/page.tsx`**

```tsx
'use client'

import { Suspense, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'

import { studentLoginSchema, StudentLoginInput } from '@/schemas/studentAuth'
import { studentApi } from '@/lib/api/student'
import { useStudentAuthStore } from '@/stores/studentAuth'
import { ApiError } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

function LoginForm() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const setAuth = useStudentAuthStore((s) => s.setAuth)
  const [loading, setLoading] = useState(false)

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<StudentLoginInput>({ resolver: zodResolver(studentLoginSchema) })

  async function onSubmit(values: StudentLoginInput) {
    setLoading(true)
    try {
      const { access_token } = await studentApi.login(values)
      // Токен кладём до me(): request-interceptor подхватит его для запроса профиля
      localStorage.setItem('tg_student_token', access_token)
      const user = await studentApi.me()
      setAuth(access_token, user)
      const next = searchParams.get('next')
      // только внутренние пути кабинета — без open redirect
      router.replace(next && next.startsWith('/student') ? next : '/student/lessons')
    } catch (err) {
      toast.error((err as ApiError).status === 401 ? 'Неверный логин или пароль' : ((err as ApiError).message ?? 'Ошибка входа'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="rounded-xl border bg-card p-8 shadow-sm">
      <h1 className="text-2xl font-semibold tracking-tight mb-1">Вход для ученика</h1>
      <p className="text-sm text-muted-foreground mb-6">
        Аккаунт создаётся по приглашению репетитора
      </p>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="identifier">Телефон или логин</Label>
          <Input
            id="identifier"
            placeholder="+77001234567 или логин"
            autoComplete="username"
            {...register('identifier')}
          />
          {errors.identifier && (
            <p className="text-xs text-destructive">{errors.identifier.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="password">Пароль</Label>
          <Input
            id="password"
            type="password"
            placeholder="••••••"
            autoComplete="current-password"
            {...register('password')}
          />
          {errors.password && (
            <p className="text-xs text-destructive">{errors.password.message}</p>
          )}
        </div>

        <Button type="submit" className="w-full" disabled={loading}>
          {loading ? 'Вход...' : 'Войти'}
        </Button>
      </form>
    </div>
  )
}

export default function StudentLoginPage() {
  return (
    <Suspense>
      <LoginForm />
    </Suspense>
  )
}
```

- [ ] **Step 3: Создать `frontend/src/app/student/(auth)/invite/[token]/page.tsx`**

```tsx
'use client'

import { useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'

import { acceptInviteSchema, AcceptInviteInput } from '@/schemas/studentAuth'
import { studentApi } from '@/lib/api/student'
import { useStudentAuthStore } from '@/stores/studentAuth'
import { ApiError } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export default function AcceptInvitePage() {
  const { token } = useParams<{ token: string }>()
  const router = useRouter()
  const setAuth = useStudentAuthStore((s) => s.setAuth)
  const [loading, setLoading] = useState(false)

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors },
  } = useForm<AcceptInviteInput>({ resolver: zodResolver(acceptInviteSchema) })

  async function onSubmit(values: AcceptInviteInput) {
    setLoading(true)
    try {
      const { access_token } = await studentApi.acceptInvite({
        token,
        username: values.username,
        password: values.password,
      })
      localStorage.setItem('tg_student_token', access_token)
      const user = await studentApi.me()
      setAuth(access_token, user)
      router.replace('/student/lessons')
    } catch (err) {
      const e = err as ApiError
      if (e.status === 409) {
        setError('username', { message: 'Такой логин или телефон уже занят' })
      } else if (e.status === 404 || e.status === 400) {
        toast.error('Ссылка недействительна или истекла. Попросите репетитора прислать новую')
      } else {
        toast.error(e.message ?? 'Не удалось создать аккаунт')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="rounded-xl border bg-card p-8 shadow-sm">
      <h1 className="text-2xl font-semibold tracking-tight mb-1">Создание аккаунта</h1>
      <p className="text-sm text-muted-foreground mb-6">
        Придумайте логин и пароль для входа в кабинет ученика
      </p>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="username">Логин</Label>
          <Input id="username" placeholder="латинские буквы и цифры" autoComplete="username" {...register('username')} />
          {errors.username && (
            <p className="text-xs text-destructive">{errors.username.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="password">Пароль</Label>
          <Input id="password" type="password" placeholder="••••••" autoComplete="new-password" {...register('password')} />
          {errors.password && (
            <p className="text-xs text-destructive">{errors.password.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="confirm">Повторите пароль</Label>
          <Input id="confirm" type="password" placeholder="••••••" autoComplete="new-password" {...register('confirm')} />
          {errors.confirm && (
            <p className="text-xs text-destructive">{errors.confirm.message}</p>
          )}
        </div>

        <Button type="submit" className="w-full" disabled={loading}>
          {loading ? 'Создание...' : 'Создать аккаунт'}
        </Button>
      </form>
    </div>
  )
}
```

- [ ] **Step 4: Проверка**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/schemas/studentAuth.ts "frontend/src/app/student/(auth)/login" "frontend/src/app/student/(auth)/invite"
git commit -m "feat(student-cabinet): login and accept-invite pages"
```

---

### Task 5: Главная — уроки с циклами

**Files:**
- Create: `frontend/src/app/student/(cabinet)/lessons/page.tsx`

**Interfaces:**
- Consumes: `studentApi.lessons(filter)` → `CalendarLesson[]` (Task 2); layout `(cabinet)` (Task 3); `SectionCard/SectionRow`, `Badge`, `Button`.
- Produces: `/student/lessons` c вкладками `?tab=upcoming|past`; кнопка «Войти в урок» → `/student/lessons/[id]/call`.

- [ ] **Step 1: Создать `frontend/src/app/student/(cabinet)/lessons/page.tsx`**

```tsx
'use client'

import { Suspense } from 'react'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'

import { studentApi, LessonsFilter } from '@/lib/api/student'
import { SectionCard, SectionRow } from '@/components/common/SectionCard'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { CalendarLesson } from '@/types/api'

const dateFmt = new Intl.DateTimeFormat('ru-RU', {
  weekday: 'short',
  day: 'numeric',
  month: 'long',
  hour: '2-digit',
  minute: '2-digit',
})

const STATUS_LABELS: Record<string, string> = {
  scheduled: 'Запланирован',
  completed: 'Проведён',
  cancelled: 'Отменён',
}

function LessonRow({ lesson, isFirst, upcoming }: { lesson: CalendarLesson; isFirst: boolean; upcoming: boolean }) {
  return (
    <SectionRow isFirst={isFirst}>
      <div className="flex items-center justify-between gap-4">
        <div style={{ minWidth: 0 }}>
          <div className="flex items-center gap-2 flex-wrap">
            <span style={{ fontSize: 14, fontWeight: 600 }}>{lesson.subject}</span>
            {lesson.is_group && <Badge variant="secondary">Группа</Badge>}
            {lesson.cycle_position != null && lesson.cycle_size != null && (
              <Badge variant="outline">
                Урок {lesson.cycle_position} из {lesson.cycle_size}
              </Badge>
            )}
          </div>
          <div style={{ fontSize: 12.5, color: 'var(--muted-foreground)', marginTop: 2 }}>
            {dateFmt.format(new Date(lesson.scheduled_at))} · {lesson.duration_minutes} мин
            {!upcoming && ` · ${STATUS_LABELS[lesson.status] ?? lesson.status}`}
          </div>
        </div>
        {upcoming && lesson.status === 'scheduled' && (
          <Button asChild size="sm">
            <Link href={`/student/lessons/${lesson.id}/call`}>Войти в урок</Link>
          </Button>
        )}
      </div>
    </SectionRow>
  )
}

function LessonsInner() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const tab: LessonsFilter = searchParams.get('tab') === 'past' ? 'past' : 'upcoming'

  const { data: lessons, isLoading, error, refetch } = useQuery({
    queryKey: ['student-lessons', tab],
    queryFn: () => studentApi.lessons(tab),
  })

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Button
          variant={tab === 'upcoming' ? 'default' : 'outline'}
          size="sm"
          onClick={() => router.replace('/student/lessons')}
        >
          Ближайшие
        </Button>
        <Button
          variant={tab === 'past' ? 'default' : 'outline'}
          size="sm"
          onClick={() => router.replace('/student/lessons?tab=past')}
        >
          Прошедшие
        </Button>
      </div>

      <SectionCard title={tab === 'upcoming' ? 'Ближайшие уроки' : 'История уроков'}>
        {isLoading && (
          <SectionRow isFirst>
            <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Загрузка...</span>
          </SectionRow>
        )}
        {error != null && !isLoading && (
          <SectionRow isFirst>
            <div className="flex items-center justify-between gap-4">
              <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>
                Не удалось загрузить уроки
              </span>
              <Button size="sm" variant="outline" onClick={() => refetch()}>
                Повторить
              </Button>
            </div>
          </SectionRow>
        )}
        {lessons && lessons.length === 0 && (
          <SectionRow isFirst>
            <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>
              {tab === 'upcoming' ? 'Ближайших уроков нет' : 'Прошедших уроков нет'}
            </span>
          </SectionRow>
        )}
        {lessons?.map((l, i) => (
          <LessonRow key={l.id} lesson={l} isFirst={i === 0} upcoming={tab === 'upcoming'} />
        ))}
      </SectionCard>
    </div>
  )
}

export default function StudentLessonsPage() {
  return (
    <Suspense>
      <LessonsInner />
    </Suspense>
  )
}
```

- [ ] **Step 2: Проверка**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок.

- [ ] **Step 3: Commit**

```bash
git add "frontend/src/app/student/(cabinet)/lessons/page.tsx"
git commit -m "feat(student-cabinet): lessons page with cycle badges"
```

---

### Task 6: Профиль и смена пароля

**Files:**
- Create: `frontend/src/app/student/(cabinet)/profile/page.tsx`

**Interfaces:**
- Consumes: `studentApi.me/changePassword`, `useStudentAuthStore` (Task 2); layout `(cabinet)` (Task 3).
- Produces: `/student/profile`.

- [ ] **Step 1: Создать `frontend/src/app/student/(cabinet)/profile/page.tsx`**

Zod-схема — локально в файле (страница — единственный потребитель):

```tsx
'use client'

import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'

import { studentApi } from '@/lib/api/student'
import { useStudentAuthStore } from '@/stores/studentAuth'
import { ApiError } from '@/types/api'
import { SectionCard, SectionRow } from '@/components/common/SectionCard'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const changePasswordSchema = z
  .object({
    old_password: z.string().min(6, 'Минимум 6 символов'),
    new_password: z.string().min(6, 'Минимум 6 символов'),
    confirm: z.string(),
  })
  .refine((d) => d.new_password === d.confirm, {
    path: ['confirm'],
    message: 'Пароли не совпадают',
  })

type ChangePasswordInput = z.infer<typeof changePasswordSchema>

export default function StudentProfilePage() {
  const setAuth = useStudentAuthStore((s) => s.setAuth)

  const { data: profile, isLoading } = useQuery({
    queryKey: ['student-me'],
    queryFn: studentApi.me,
  })

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<ChangePasswordInput>({ resolver: zodResolver(changePasswordSchema) })

  async function onSubmit(values: ChangePasswordInput) {
    try {
      const { access_token } = await studentApi.changePassword({
        old_password: values.old_password,
        new_password: values.new_password,
      })
      // Backend отозвал ВСЕ refresh-сессии и выдал новую текущему устройству.
      // Без сохранения свежего токена следующий запрос уйдёт с мёртвой сессией.
      localStorage.setItem('tg_student_token', access_token)
      if (profile) setAuth(access_token, profile)
      toast.success('Пароль изменён. Другие устройства разлогинены')
      reset()
    } catch (err) {
      const e = err as ApiError
      if (e.status === 401) {
        setError('old_password', { message: 'Неверный текущий пароль' })
      } else {
        toast.error(e.message ?? 'Не удалось сменить пароль')
      }
    }
  }

  return (
    <div className="space-y-4">
      <SectionCard title="Профиль">
        {isLoading && (
          <SectionRow isFirst>
            <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Загрузка...</span>
          </SectionRow>
        )}
        {profile && (
          <>
            <SectionRow isFirst>
              <ProfileField label="Имя" value={`${profile.first_name} ${profile.last_name}`.trim()} />
            </SectionRow>
            <SectionRow>
              <ProfileField label="Телефон" value={profile.phone || '—'} />
            </SectionRow>
            <SectionRow>
              <ProfileField label="Логин" value={profile.username || '—'} />
            </SectionRow>
          </>
        )}
      </SectionCard>

      <SectionCard title="Смена пароля" bodyPadding={18}>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4 max-w-sm">
          <div className="space-y-1.5">
            <Label htmlFor="old_password">Текущий пароль</Label>
            <Input id="old_password" type="password" autoComplete="current-password" {...register('old_password')} />
            {errors.old_password && (
              <p className="text-xs text-destructive">{errors.old_password.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="new_password">Новый пароль</Label>
            <Input id="new_password" type="password" autoComplete="new-password" {...register('new_password')} />
            {errors.new_password && (
              <p className="text-xs text-destructive">{errors.new_password.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="confirm">Повторите новый пароль</Label>
            <Input id="confirm" type="password" autoComplete="new-password" {...register('confirm')} />
            {errors.confirm && (
              <p className="text-xs text-destructive">{errors.confirm.message}</p>
            )}
          </div>

          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Сохранение...' : 'Сменить пароль'}
          </Button>
        </form>
      </SectionCard>
    </div>
  )
}

function ProfileField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>{label}</span>
      <span style={{ fontSize: 13.5, fontWeight: 500 }}>{value}</span>
    </div>
  )
}
```

- [ ] **Step 2: Проверка**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок.

- [ ] **Step 3: Commit**

```bash
git add "frontend/src/app/student/(cabinet)/profile/page.tsx"
git commit -m "feat(student-cabinet): profile page with password change"
```

---

### Task 7: Страница звонка (звонок + доска)

**Files:**
- Create: `frontend/src/app/student/(room)/lessons/[id]/call/page.tsx`

**Interfaces:**
- Consumes: `callsApi.getRoomStatus` (публичный, через существующий Next-proxy `/api/room-status/`), `studentApi.roomToken` (Task 2), `CallRoom` (`serverUrl`, `token`, `role: 'guest'`, `enableMedia`, `onDisconnected`), layout `(room)` (Task 3).
- Produces: `/student/lessons/[id]/call`. Доска отдельного кода не требует: guest-путь `CallRoom` получает board-токен по LiveKit DataChannel от репетитора.

- [ ] **Step 1: Создать `frontend/src/app/student/(room)/lessons/[id]/call/page.tsx`**

```tsx
'use client'

import { useEffect, useRef, useState } from 'react'
import Link from 'next/link'
import { useParams } from 'next/navigation'
import '@livekit/components-styles'

import { callsApi, type RoomTokenResponse } from '@/lib/api/calls'
import { studentApi } from '@/lib/api/student'
import { CallRoom } from '@/components/call/CallRoom'
import { Button } from '@/components/ui/button'
import { GraduationCap } from 'lucide-react'
import type { ApiError } from '@/types/api'

type Stage = 'waiting' | 'in-room' | 'ended' | 'forbidden'

export default function StudentCallPage() {
  const { id } = useParams<{ id: string }>()

  const [stage, setStage] = useState<Stage>('waiting')
  const [room, setRoom] = useState<RoomTokenResponse | null>(null)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  function clearPolling() {
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
      intervalRef.current = null
    }
  }

  async function tryJoin(): Promise<boolean> {
    try {
      const { status } = await callsApi.getRoomStatus(id)
      if (status === 'ended') {
        clearPolling()
        setStage('ended')
        return true
      }
      if (status !== 'active') return false
      const data = await studentApi.roomToken(id)
      clearPolling()
      setRoom(data)
      setStage('in-room')
      return true
    } catch (err) {
      // 403 — выписали из курса; дальше поллить бессмысленно
      if ((err as ApiError).status === 403) {
        clearPolling()
        setStage('forbidden')
        return true
      }
      return false
    }
  }

  useEffect(() => {
    tryJoin().then((done) => {
      if (!done) intervalRef.current = setInterval(tryJoin, 5000)
    })
    return clearPolling
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  async function handleDisconnected() {
    setRoom(null)
    try {
      const { status } = await callsApi.getRoomStatus(id)
      if (status === 'ended') {
        setStage('ended')
        return
      }
    } catch {}
    setStage('waiting')
    intervalRef.current = setInterval(tryJoin, 5000)
  }

  if (stage === 'in-room' && room) {
    return (
      <div style={{ height: '100dvh' }}>
        <CallRoom
          serverUrl={room.server_url}
          token={room.token}
          role="guest"
          enableMedia
          onDisconnected={handleDisconnected}
        />
      </div>
    )
  }

  const message =
    stage === 'ended'
      ? { title: 'Урок завершён', text: 'Спасибо за занятие!' }
      : stage === 'forbidden'
        ? { title: 'Нет доступа к уроку', text: 'Обратитесь к репетитору' }
        : { title: 'Урок ещё не начался', text: 'Ожидаем, когда репетитор начнёт урок...' }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="flex items-center gap-2 justify-center">
          <GraduationCap className="h-6 w-6 text-primary" />
          <span className="font-semibold text-lg">TutorHub</span>
        </div>
        <div className="rounded-lg border p-6 text-center space-y-3">
          <p className="font-semibold">{message.title}</p>
          <p className="text-sm text-muted-foreground">{message.text}</p>
          <Button asChild variant="outline" size="sm">
            <Link href="/student/lessons">К моим урокам</Link>
          </Button>
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Проверка**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок.

- [ ] **Step 3: Commit**

```bash
git add "frontend/src/app/student/(room)/lessons/[id]/call/page.tsx"
git commit -m "feat(student-cabinet): lesson call page with room-status polling"
```

---

### Task 8: Tutor-UI — кнопка «Пригласить в кабинет»

**Files:**
- Modify: `frontend/src/lib/api/students.ts`
- Modify: `frontend/src/components/students/StudentsList.tsx`

**Interfaces:**
- Consumes: `POST /students/:id/invite` → `{ invite_token: string; expires_at: string }` (backend готов); `Dialog` из `components/ui/dialog`.
- Produces: `studentsApi.invite(id)`; кнопка-иконка UserPlus в строке ученика → диалог со ссылкой `${origin}/student/invite/{token}`.

- [ ] **Step 1: Добавить метод в `frontend/src/lib/api/students.ts`**

В объект `studentsApi` добавить:

```ts
  invite: (id: string) =>
    api.post<{ invite_token: string; expires_at: string }>(`/students/${id}/invite`)
      .then((r) => r.data),
```

- [ ] **Step 2: Кнопка + диалог в `StudentsList.tsx`**

Диалог живёт внутри `StudentsList` — родителей не трогаем. Полная замена файла:

```tsx
'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Pencil, Trash2, ChevronRight, UserPlus, Copy } from 'lucide-react'
import { toast } from 'sonner'

import { SectionCard } from '@/components/common/SectionCard'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { studentsApi } from '@/lib/api/students'
import { Student } from '@/types/api'

interface StudentsListProps {
  students: Student[]
  onEdit: (s: Student) => void
  onDelete: (s: Student) => void
}

export function StudentsList({ students, onEdit, onDelete }: StudentsListProps) {
  const router = useRouter()
  const [inviteFor, setInviteFor] = useState<Student | null>(null)
  const [invite, setInvite] = useState<{ invite_token: string; expires_at: string } | null>(null)

  async function handleInvite(s: Student) {
    setInviteFor(s)
    setInvite(null)
    try {
      setInvite(await studentsApi.invite(s.id))
    } catch {
      toast.error('Не удалось создать приглашение')
      setInviteFor(null)
    }
  }

  const inviteUrl = invite
    ? `${window.location.origin}/student/invite/${invite.invite_token}`
    : ''

  function copyInvite() {
    try {
      navigator.clipboard.writeText(inviteUrl)
      toast.success('Ссылка скопирована')
    } catch {}
  }

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
              <Button size="icon" variant="ghost" className="h-8 w-8"
                title="Пригласить в кабинет ученика"
                onClick={() => handleInvite(student)}>
                <UserPlus className="h-3.5 w-3.5" />
              </Button>
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

      <Dialog open={inviteFor !== null} onOpenChange={(open) => { if (!open) setInviteFor(null) }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Приглашение в кабинет</DialogTitle>
            <DialogDescription>
              {inviteFor
                ? `Отправьте ссылку ученику: ${inviteFor.first_name}${inviteFor.last_name ? ` ${inviteFor.last_name}` : ''}. По ней он создаст аккаунт и получит доступ к своим урокам.`
                : ''}
            </DialogDescription>
          </DialogHeader>
          {invite ? (
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <Input readOnly value={inviteUrl} onFocus={(e) => e.target.select()} />
                <Button size="icon" variant="outline" className="shrink-0" onClick={copyInvite} title="Скопировать">
                  <Copy className="h-4 w-4" />
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                Ссылка действует до {new Date(invite.expires_at).toLocaleDateString('ru-RU')}.
                Повторное приглашение заменит эту ссылку.
              </p>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Создание ссылки...</p>
          )}
        </DialogContent>
      </Dialog>
    </SectionCard>
  )
}
```

- [ ] **Step 3: Проверка**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api/students.ts frontend/src/components/students/StudentsList.tsx
git commit -m "feat(students): invite-to-cabinet button with link dialog"
```

---

### Task 9: Уборка мёртвого гостевого входа

**Files:**
- Rewrite: `frontend/src/app/join/[lessonId]/page.tsx`
- Modify: `frontend/src/lib/api/calls.ts` (удалить `getGuestToken`)
- Delete: `frontend/src/app/api/guest-token/` (вся папка)
- Modify: `frontend/src/app/(call)/lessons/[id]/call/page.tsx:63`

**Interfaces:**
- Consumes: ничего нового.
- Produces: `/join/[lessonId]` → redirect на `/student/login?next=/student/lessons`; `inviteUrl` репетитора ведёт на `/student/lessons/{id}/call`. Quick-rooms (`/join/room/[id]`, `getQuickGuestToken`) не трогаем.

- [ ] **Step 1: Заменить `frontend/src/app/join/[lessonId]/page.tsx` целиком**

```tsx
import { redirect } from 'next/navigation'

// Анонимный гостевой вход в запланированные уроки удалён (уроки приватные,
// backend-эндпоинт guest-token больше не существует). Старые ссылки ведём на
// логин кабинета ученика.
export default function JoinLessonRedirect() {
  redirect('/student/login?next=/student/lessons')
}
```

- [ ] **Step 2: Удалить `getGuestToken` из `frontend/src/lib/api/calls.ts`**

Удалить блок:

```ts
  getGuestToken: (lessonId: string) =>
    fetch(`/api/guest-token/${lessonId}`)
      .then((r) => { if (!r.ok) throw new Error(); return r.json() as Promise<RoomTokenResponse> }),
```

Убедиться, что импортов не осталось: `grep -rn "getGuestToken" frontend/src` → пусто.

- [ ] **Step 3: Удалить proxy-роут**

```bash
rm -r frontend/src/app/api/guest-token
```

- [ ] **Step 4: Поменять inviteUrl репетитора**

В `frontend/src/app/(call)/lessons/[id]/call/page.tsx` строка 63:

```tsx
  const inviteUrl = `${window.location.origin}/student/lessons/${id}/call`
```

- [ ] **Step 5: Проверка**

Run: `cd frontend && npx tsc --noEmit && grep -rn "getGuestToken\|/api/guest-token" src || true`
Expected: tsc зелёный; grep ничего не находит.

- [ ] **Step 6: Commit**

```bash
git add -A frontend/src/app/join frontend/src/lib/api/calls.ts frontend/src/app/api "frontend/src/app/(call)/lessons/[id]/call/page.tsx"
git commit -m "chore: replace dead guest lesson entry with student login redirect"
```

---

### Task 10: Финальная сборка и smoke

**Files:** — (только проверки и мелкие фиксы, если сборка их потребует)

- [ ] **Step 1: Полная сборка**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: сборка зелёная; в выводе маршрутов появились `/student/login`, `/student/invite/[token]`, `/student/lessons`, `/student/lessons/[id]/call`, `/student/profile`.

- [ ] **Step 2: Backend-проверки**

Run: `go build ./... && go vet ./... && make test`
Expected: зелёные.

- [ ] **Step 3: Smoke-чеклист (вывести пользователю для ручной проверки)**

1. Репетитор: список учеников → UserPlus → ссылка скопирована.
2. Инкогнито: открыть ссылку → создать аккаунт → попадаем в `/student/lessons`.
3. Уроки: вкладки, бейджи «Урок N из M» (курс с платежами), «Войти в урок».
4. Репетитор начинает урок → ученик через кнопку попадает в звонок; репетитор открывает доску → доска появляется у ученика.
5. Профиль: смена пароля (неверный старый → ошибка под полем; верный → тост, сессия жива).
6. Logout → `/student/lessons` редиректит на логин с `next`; login по телефону и по username.
7. Старая ссылка `/join/<uuid>` ведёт на студенческий логин.

- [ ] **Step 4: Commit (если были фиксы сборки)**

```bash
git add -A && git commit -m "fix(student-cabinet): build fixes"
```
