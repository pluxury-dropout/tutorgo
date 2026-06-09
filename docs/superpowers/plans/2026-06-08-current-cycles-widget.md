# Виджет «Текущие циклы» — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить виджет на `/dashboard`, показывающий текущий платёжный цикл каждого курса тьютора — предмет, ученик, прогресс `X / N`, дата последнего урока.

**Architecture:** Новый endpoint `GET /dashboard/cycles` реализуется через существующие слои Go (repo → service → handler). Сервисный метод напрямую работает с платёжными границами для группировки уроков по циклам, без дублирования логики `computeCyclePositions`. Фронтенд добавляет хук и третий виджет в существующий grid дашборда.

**Tech Stack:** Go + pgx/v5, Gin, React/Next.js, TanStack Query

---

## Затронутые файлы

| Файл | Действие |
|------|----------|
| `models/lesson.go` | добавить `CurrentCycleInfo` |
| `repository/lesson.go` | добавить `GetAllLessonsForCycles` в интерфейс + реализацию |
| `service/lesson.go` | добавить `GetCurrentCycles` в интерфейс + реализацию; добавить импорты `sort`, `time` |
| `service/lesson_test.go` | добавить mock-метод + тест `TestGetCurrentCycles` |
| `handlers/lesson.go` | добавить метод `GetCurrentCycles` |
| `router/router.go` | зарегистрировать `GET /dashboard/cycles` |
| `frontend/src/types/api.ts` | добавить `CurrentCycleInfo` |
| `frontend/src/lib/api/calendar.ts` | добавить `getCurrentCycles()` |
| `frontend/src/lib/hooks/useCalendar.ts` | добавить `useCurrentCycles()` |
| `frontend/src/app/(dashboard)/dashboard/page.tsx` | добавить виджет |

---

### Task 1: Модель `CurrentCycleInfo`

**Files:**
- Modify: `models/lesson.go`

- [ ] **Step 1: Добавить структуру в конец `models/lesson.go`**

```go
type CurrentCycleInfo struct {
	CourseID    string    `json:"course_id"`
	Subject     string    `json:"subject"`
	StudentName *string   `json:"student_name"`
	Progress    int       `json:"progress"`
	CycleSize   int       `json:"cycle_size"`
	LastAt      time.Time `json:"last_at"`
}
```

`time` уже импортирован в файле — новых импортов не нужно.

- [ ] **Step 2: Проверить компиляцию**

```bash
go build ./...
```

Ожидание: успех, без ошибок.

- [ ] **Step 3: Коммит**

```bash
git add models/lesson.go
git commit -m "feat: add CurrentCycleInfo model"
```

---

### Task 2: Repo метод `GetAllLessonsForCycles`

**Files:**
- Modify: `repository/lesson.go`

- [ ] **Step 1: Добавить метод в интерфейс `LessonRepository`** (после строки с `GetCalendar`)

```go
GetAllLessonsForCycles(ctx context.Context, tutorID string) ([]models.CalendarLesson, error)
```

- [ ] **Step 2: Добавить реализацию в конец файла** (перед последней закрывающей скобкой нет — просто в конец файла)

```go
func (r *lessonRepository) GetAllLessonsForCycles(ctx context.Context, tutorID string) ([]models.CalendarLesson, error) {
	rows, err := r.pool.Query(ctx,
		`WITH ranked AS (
		   SELECT l.id,
		          ROW_NUMBER() OVER (PARTITION BY l.course_id ORDER BY l.scheduled_at)::int AS rank
		   FROM lessons l
		   JOIN courses c ON c.id = l.course_id
		   WHERE c.tutor_id = $1
		     AND l.status != 'cancelled'
		 )
		 SELECT l.id, l.course_id, l.scheduled_at, l.status,
		        c.subject,
		        CASE WHEN c.student_id IS NOT NULL
		             THEN CASE WHEN s.last_name = '' THEN s.first_name ELSE s.first_name || ' ' || s.last_name END
		             ELSE NULL
		        END AS student_name,
		        (c.student_id IS NULL) AS is_group,
		        r.rank
		 FROM lessons l
		 JOIN courses c ON c.id = l.course_id
		 LEFT JOIN students s ON s.id = c.student_id
		 LEFT JOIN ranked r ON r.id = l.id
		 WHERE c.tutor_id = $1
		   AND l.status != 'cancelled'
		 ORDER BY l.course_id, l.scheduled_at`,
		tutorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []models.CalendarLesson
	for rows.Next() {
		var cl models.CalendarLesson
		if err := rows.Scan(
			&cl.ID, &cl.CourseID, &cl.ScheduledAt, &cl.Status,
			&cl.Subject, &cl.StudentName, &cl.IsGroup, &cl.Rank,
		); err != nil {
			return nil, err
		}
		lessons = append(lessons, cl)
	}
	return lessons, rows.Err()
}
```

- [ ] **Step 3: Проверить компиляцию**

```bash
go build ./...
```

Ожидание: успех. Если ошибка `mockLessonRepo does not implement GetAllLessonsForCycles` — значит нужно перейти к Task 3 Step 1 (добавить mock-метод) перед компиляцией тестов.

- [ ] **Step 4: Коммит**

```bash
git add repository/lesson.go
git commit -m "feat: add GetAllLessonsForCycles repo method"
```

---

### Task 3: Service метод `GetCurrentCycles` + тест

**Files:**
- Modify: `service/lesson.go`
- Modify: `service/lesson_test.go`

- [ ] **Step 1: Добавить mock-метод в `service/lesson_test.go`** (после последнего `func (m *mockLessonRepo)` метода, перед `// fixtures`)

```go
func (m *mockLessonRepo) GetAllLessonsForCycles(ctx context.Context, tutorID string) ([]models.CalendarLesson, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).([]models.CalendarLesson), args.Error(1)
}
```

- [ ] **Step 2: Написать тест** (добавить в конец `service/lesson_test.go`)

```go
func TestGetCurrentCycles_ReturnsActiveCycle(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	paymentRepo := new(mockPaymentRepo)
	svc := newLessonSvcWithPayment(lessonRepo, new(mockCourseRepo), paymentRepo)

	r1, r2, r3, r4 := 1, 2, 3, 4
	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	studentName := "Азиз"
	lessons := []models.CalendarLesson{
		{ID: "l1", CourseID: "c1", Status: "completed", Subject: "Математика", StudentName: &studentName, Rank: &r1, ScheduledAt: base},
		{ID: "l2", CourseID: "c1", Status: "completed", Subject: "Математика", StudentName: &studentName, Rank: &r2, ScheduledAt: base.AddDate(0, 0, 7)},
		{ID: "l3", CourseID: "c1", Status: "scheduled", Subject: "Математика", StudentName: &studentName, Rank: &r3, ScheduledAt: base.AddDate(0, 0, 14)},
		{ID: "l4", CourseID: "c1", Status: "scheduled", Subject: "Математика", StudentName: &studentName, Rank: &r4, ScheduledAt: base.AddDate(0, 0, 21)},
	}
	lessonRepo.On("GetAllLessonsForCycles", mock.Anything, "tutor-1").Return(lessons, nil)
	paymentRepo.On("GetByCoursesBatch", mock.Anything, mock.Anything).Return(map[string][]models.Payment{
		"c1": {{LessonsCount: 4}},
	}, nil)

	result, err := svc.GetCurrentCycles(context.Background(), "tutor-1")

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "c1", result[0].CourseID)
	assert.Equal(t, "Математика", result[0].Subject)
	assert.Equal(t, &studentName, result[0].StudentName)
	assert.Equal(t, 2, result[0].Progress)
	assert.Equal(t, 4, result[0].CycleSize)
	assert.Equal(t, base.AddDate(0, 0, 21), result[0].LastAt)
}

func TestGetCurrentCycles_SkipsCourseWithNoScheduled(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	paymentRepo := new(mockPaymentRepo)
	svc := newLessonSvcWithPayment(lessonRepo, new(mockCourseRepo), paymentRepo)

	r1, r2 := 1, 2
	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	lessons := []models.CalendarLesson{
		{ID: "l1", CourseID: "c1", Status: "completed", Subject: "Физика", Rank: &r1, ScheduledAt: base},
		{ID: "l2", CourseID: "c1", Status: "completed", Subject: "Физика", Rank: &r2, ScheduledAt: base.AddDate(0, 0, 7)},
	}
	lessonRepo.On("GetAllLessonsForCycles", mock.Anything, "tutor-1").Return(lessons, nil)
	paymentRepo.On("GetByCoursesBatch", mock.Anything, mock.Anything).Return(map[string][]models.Payment{
		"c1": {{LessonsCount: 2}},
	}, nil)

	result, err := svc.GetCurrentCycles(context.Background(), "tutor-1")

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetCurrentCycles_EmptyLessons(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	paymentRepo := new(mockPaymentRepo)
	svc := newLessonSvcWithPayment(lessonRepo, new(mockCourseRepo), paymentRepo)

	lessonRepo.On("GetAllLessonsForCycles", mock.Anything, "tutor-1").Return([]models.CalendarLesson{}, nil)

	result, err := svc.GetCurrentCycles(context.Background(), "tutor-1")

	assert.NoError(t, err)
	assert.Empty(t, result)
}
```

- [ ] **Step 3: Запустить тесты — убедиться что падают**

```bash
go test ./service/ -run TestGetCurrentCycles -v
```

Ожидание: FAIL — `GetCurrentCycles undefined`

- [ ] **Step 4: Добавить `GetCurrentCycles` в интерфейс `LessonService`** (в `service/lesson.go`, после строки с `GetCalendar`)

```go
GetCurrentCycles(ctx context.Context, tutorID string) ([]models.CurrentCycleInfo, error)
```

- [ ] **Step 5: Добавить импорты в `service/lesson.go`** (добавить `"sort"` и `"time"` в блок импортов)

```go
import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
	"tutorgo/models"
	"tutorgo/repository"
)
```

- [ ] **Step 6: Добавить реализацию в `service/lesson.go`** (перед функцией `ExistsPublic`)

```go
func (s *lessonService) GetCurrentCycles(ctx context.Context, tutorID string) ([]models.CurrentCycleInfo, error) {
	lessons, err := s.repo.GetAllLessonsForCycles(ctx, tutorID)
	if err != nil {
		return nil, err
	}
	if len(lessons) == 0 {
		return nil, nil
	}

	type lessonMeta struct {
		id          string
		scheduledAt time.Time
		status      string
		rank        int
	}
	type courseMeta struct {
		subject     string
		studentName *string
		lessons     []lessonMeta
	}

	coursesByID := map[string]*courseMeta{}
	for _, l := range lessons {
		if l.Rank == nil {
			continue
		}
		if _, ok := coursesByID[l.CourseID]; !ok {
			coursesByID[l.CourseID] = &courseMeta{subject: l.Subject, studentName: l.StudentName}
		}
		coursesByID[l.CourseID].lessons = append(coursesByID[l.CourseID].lessons, lessonMeta{
			id: l.ID, scheduledAt: l.ScheduledAt, status: l.Status, rank: *l.Rank,
		})
	}

	courseIDs := make([]string, 0, len(coursesByID))
	for id := range coursesByID {
		courseIDs = append(courseIDs, id)
	}

	paymentsMap, err := s.paymentRepo.GetByCoursesBatch(ctx, courseIDs)
	if err != nil {
		return nil, err
	}

	var result []models.CurrentCycleInfo
	for courseID, meta := range coursesByID {
		payments := paymentsMap[courseID]
		if len(payments) == 0 {
			continue
		}

		bounds := make([]int, len(payments))
		cum := 0
		for i, p := range payments {
			cum += p.LessonsCount
			bounds[i] = cum
		}

		buckets := make([][]lessonMeta, len(payments))
		for _, lm := range meta.lessons {
			for i, bound := range bounds {
				prev := 0
				if i > 0 {
					prev = bounds[i-1]
				}
				if lm.rank > prev && lm.rank <= bound {
					buckets[i] = append(buckets[i], lm)
					break
				}
			}
		}

		currentIdx := -1
		for i := len(buckets) - 1; i >= 0; i-- {
			for _, lm := range buckets[i] {
				if lm.status == "scheduled" {
					currentIdx = i
					break
				}
			}
			if currentIdx >= 0 {
				break
			}
		}
		if currentIdx < 0 {
			continue
		}

		bucket := buckets[currentIdx]
		cycleSize := payments[currentIdx].LessonsCount
		var lastAt time.Time
		progress := 0
		for _, lm := range bucket {
			if lm.status == "completed" || lm.status == "missed" {
				progress++
			}
			if lm.scheduledAt.After(lastAt) {
				lastAt = lm.scheduledAt
			}
		}

		result = append(result, models.CurrentCycleInfo{
			CourseID:    courseID,
			Subject:     meta.subject,
			StudentName: meta.studentName,
			Progress:    progress,
			CycleSize:   cycleSize,
			LastAt:      lastAt,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].LastAt.After(result[j].LastAt)
	})
	return result, nil
}
```

- [ ] **Step 7: Запустить тесты — убедиться что проходят**

```bash
go test ./service/ -run TestGetCurrentCycles -v
```

Ожидание: PASS все три теста.

- [ ] **Step 8: Запустить все тесты**

```bash
go test ./...
```

Ожидание: все тесты PASS.

- [ ] **Step 9: Коммит**

```bash
git add service/lesson.go service/lesson_test.go
git commit -m "feat: add GetCurrentCycles service method"
```

---

### Task 4: Handler + Router

**Files:**
- Modify: `handlers/lesson.go`
- Modify: `router/router.go`

- [ ] **Step 1: Добавить handler в конец `handlers/lesson.go`**

```go
func (h *LessonHandler) GetCurrentCycles(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	cycles, err := h.service.GetCurrentCycles(c.Request.Context(), tutorID)
	if err != nil {
		h.log.Error("Failed to get current cycles", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, cycles)
}
```

- [ ] **Step 2: Зарегистрировать маршрут в `router/router.go`** (после строки `auth.GET("/calendar", lessonHandler.GetCalendar)`)

```go
auth.GET("/dashboard/cycles", lessonHandler.GetCurrentCycles)
```

- [ ] **Step 3: Проверить компиляцию и запустить**

```bash
go build ./... && go test ./...
```

Ожидание: успех.

- [ ] **Step 4: Проверить endpoint вручную** (сервер должен быть запущен: `air` или `go run .`)

```bash
# получить токен через POST /auth/login, подставить в заголовок
curl -H "Authorization: Bearer <TOKEN>" http://localhost:8080/dashboard/cycles
```

Ожидание: JSON-массив `[]` или список циклов.

- [ ] **Step 5: Коммит**

```bash
git add handlers/lesson.go router/router.go
git commit -m "feat: add GET /dashboard/cycles endpoint"
```

---

### Task 5: Frontend — тип, API, хук

**Files:**
- Modify: `frontend/src/types/api.ts`
- Modify: `frontend/src/lib/api/calendar.ts`
- Modify: `frontend/src/lib/hooks/useCalendar.ts`

- [ ] **Step 1: Добавить тип в `frontend/src/types/api.ts`** (в конец файла)

```ts
export interface CurrentCycleInfo {
  course_id: string
  subject: string
  student_name: string | null
  progress: number
  cycle_size: number
  last_at: string
}
```

- [ ] **Step 2: Добавить API-метод в `frontend/src/lib/api/calendar.ts`**

Заменить весь файл:

```ts
import { api } from './client'
import { CalendarLesson, CurrentCycleInfo } from '@/types/api'

export const calendarApi = {
  list: (from: string, to: string) =>
    api
      .get<CalendarLesson[]>('/calendar', { params: { from, to } })
      .then((r) => r.data ?? []),

  getCurrentCycles: () =>
    api
      .get<CurrentCycleInfo[]>('/dashboard/cycles')
      .then((r) => r.data ?? []),
}
```

- [ ] **Step 3: Добавить хук в `frontend/src/lib/hooks/useCalendar.ts`** (в конец файла)

```ts
export function useCurrentCycles() {
  return useQuery({
    queryKey: ['dashboard', 'cycles'],
    queryFn:  () => calendarApi.getCurrentCycles(),
  })
}
```

- [ ] **Step 4: Проверить TypeScript**

```bash
cd frontend && npx tsc --noEmit
```

Ожидание: без ошибок.

- [ ] **Step 5: Коммит**

```bash
git add frontend/src/types/api.ts frontend/src/lib/api/calendar.ts frontend/src/lib/hooks/useCalendar.ts
git commit -m "feat: add CurrentCycleInfo type, API method and hook"
```

---

### Task 6: Frontend — виджет на дашборде

**Files:**
- Modify: `frontend/src/app/(dashboard)/dashboard/page.tsx`

- [ ] **Step 1: Добавить импорт хука** в блок импортов вверху файла

```ts
import { useCurrentCycles } from '@/lib/hooks/useCalendar'
```

- [ ] **Step 2: Добавить вызов хука** в `DashboardPage` сразу после строки с `useMonthlyExpected`

```ts
const { data: currentCycles = [] } = useCurrentCycles()
```

- [ ] **Step 3: Добавить виджет** в JSX — третьей секцией внутри grid `grid-cols-1 md:grid-cols-2`, после секции `Последние платежи`

```tsx
{/* Текущие циклы */}
<section style={{ display: 'flex', flexDirection: 'column' }}>
  <header style={WIDGET_HEAD}>
    <h2 style={WIDGET_TITLE}>Текущие циклы</h2>
    <Link href="/courses" style={WIDGET_LINK}>Курсы →</Link>
  </header>
  {currentCycles.length === 0
    ? <p style={EMPTY}>Нет активных циклов</p>
    : currentCycles.map((cycle, i) => {
        const complete = cycle.progress === cycle.cycle_size
        return (
          <div
            key={cycle.course_id}
            style={{
              display: 'grid',
              gridTemplateColumns: '1fr auto 52px',
              alignItems: 'center',
              gap: 12,
              padding: '8px 0',
              borderTop: i === 0 ? 'none' : '1px solid var(--border)',
            }}
          >
            <div>
              <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--foreground)' }}>
                {cycle.subject}
              </span>
              {cycle.student_name && (
                <div style={{ fontSize: 12.5, color: 'var(--muted-foreground)', marginTop: 1 }}>
                  {cycle.student_name}
                </div>
              )}
            </div>
            <span style={{
              fontSize: 12,
              fontWeight: 600,
              padding: '2px 7px',
              borderRadius: 6,
              border: '1px solid',
              whiteSpace: 'nowrap',
              color:       complete ? 'var(--success)' : 'var(--foreground)',
              borderColor: complete ? 'color-mix(in srgb, var(--success) 40%, transparent)' : 'var(--border)',
              background:  complete ? 'color-mix(in srgb, var(--success) 10%, transparent)' : 'var(--muted)',
            }}>
              {cycle.progress} / {cycle.cycle_size}
            </span>
            <span style={{
              fontSize: 12.5,
              color: 'var(--muted-foreground)',
              textAlign: 'right',
              fontVariantNumeric: 'tabular-nums',
            }}>
              {fmtDate(cycle.last_at)}
            </span>
          </div>
        )
      })
  }
</section>
```

- [ ] **Step 4: Проверить TypeScript**

```bash
cd frontend && npx tsc --noEmit
```

Ожидание: без ошибок.

- [ ] **Step 5: Запустить фронтенд и проверить вручную**

```bash
cd frontend && npm run dev
```

Открыть `http://localhost:3000/dashboard`. Убедиться:
- Виджет «Текущие циклы» отображается в grid
- Строки показывают предмет, имя ученика (для индивидуальных), badge `X / N`
- Когда `progress === cycle_size` — badge зелёный
- Групповые курсы не показывают имя под предметом
- Пустое состояние «Нет активных циклов» если циклов нет

- [ ] **Step 6: Коммит**

```bash
git add frontend/src/app/\(dashboard\)/dashboard/page.tsx
git commit -m "feat: add current cycles widget to dashboard"
```
