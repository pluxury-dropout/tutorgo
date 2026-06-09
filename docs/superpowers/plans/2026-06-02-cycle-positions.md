# Cycle Position Badges on Lessons — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show each lesson's ordinal number within its payment cycle as a badge in the calendar and course lesson list; the last lesson of a cycle shows a red badge.

**Architecture:** Service-layer enrichment — two batch SQL queries fetch lesson ranks and payments per course, then `computeCyclePositions` (pure Go, unit-tested) annotates each lesson. Cancelled lessons are excluded from ranking. Lessons beyond total paid count receive no badge.

**Tech Stack:** Go (Gin, pgx v5), Next.js 14 (React, TailwindCSS)

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `models/lesson.go` | Modify | Add `CyclePosition *int`, `CycleSize *int` to `Lesson` and `CalendarLesson` |
| `repository/lesson.go` | Modify | Add `GetRanksForCourses` to interface + impl |
| `repository/payment.go` | Modify | Add `GetByCoursesBatch` to interface + impl |
| `service/lesson.go` | Modify | Add `paymentRepo`, `computeCyclePositions`, enrich `GetCalendar` + `GetByPeriod` |
| `service/cycle_test.go` | Create | Unit tests for `computeCyclePositions` (package service) |
| `service/payment_test.go` | Modify | Add `GetByCoursesBatch` stub to `mockPaymentRepo` |
| `service/lesson_test.go` | Modify | Add `GetRanksForCourses` stub to `mockLessonRepo`; update `newLessonSvc` |
| `router/router.go` | Modify | Pass `paymentRepo` to `NewLessonService` |
| `frontend/src/types/api.ts` | Modify | Add `cycle_position?`, `cycle_size?` to `Lesson` and `CalendarLesson` |
| `frontend/src/components/lessons/CycleBadge.tsx` | Create | Badge component |
| `frontend/src/app/(dashboard)/calendar/page.tsx` | Modify | Render badge in `eventContent` + pass fields via `extendedProps` |
| `frontend/src/app/(dashboard)/courses/[id]/page.tsx` | Modify | Render badge in lesson list row |

---

### Task 1: Add CyclePosition and CycleSize fields to models

**Files:**
- Modify: `models/lesson.go`

- [ ] **Step 1: Add the two optional fields to both structs**

In `models/lesson.go`, add to `Lesson`:
```go
type Lesson struct {
	ID              string    `json:"id"`
	CourseID        string    `json:"course_id"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	DurationMinutes int       `json:"duration_minutes"`
	Status          string    `json:"status"`
	Notes           string    `json:"notes"`
	SeriesID        *string   `json:"series_id,omitempty"`
	CyclePosition   *int      `json:"cycle_position,omitempty"`
	CycleSize       *int      `json:"cycle_size,omitempty"`
}
```

Add to `CalendarLesson`:
```go
type CalendarLesson struct {
	ID              string    `json:"id"`
	CourseID        string    `json:"course_id"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	DurationMinutes int       `json:"duration_minutes"`
	Status          string    `json:"status"`
	Notes           string    `json:"notes"`
	Subject         string    `json:"subject"`
	StudentName     *string   `json:"student_name"`
	IsGroup         bool      `json:"is_group"`
	SeriesID        *string   `json:"series_id,omitempty"`
	CyclePosition   *int      `json:"cycle_position,omitempty"`
	CycleSize       *int      `json:"cycle_size,omitempty"`
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add models/lesson.go
git commit -m "feat: add CyclePosition/CycleSize fields to Lesson and CalendarLesson models"
```

---

### Task 2: Add GetRanksForCourses to lesson repository

**Files:**
- Modify: `repository/lesson.go`

- [ ] **Step 1: Add method to the interface**

In `repository/lesson.go`, add to `LessonRepository` interface:
```go
GetRanksForCourses(ctx context.Context, courseIDs []string) (map[string]map[string]int, error)
```
Returns `map[courseID]map[lessonID]rank` — 1-based rank of each non-cancelled lesson, partitioned by course and ordered by `scheduled_at`.

- [ ] **Step 2: Implement the method**

Add below the existing methods in `repository/lesson.go`:
```go
func (r *lessonRepository) GetRanksForCourses(ctx context.Context, courseIDs []string) (map[string]map[string]int, error) {
	if len(courseIDs) == 0 {
		return map[string]map[string]int{}, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, course_id,
		        ROW_NUMBER() OVER (PARTITION BY course_id ORDER BY scheduled_at)::int AS rank
		 FROM lessons
		 WHERE course_id = ANY($1)
		   AND status != 'cancelled'`,
		courseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]map[string]int{}
	for rows.Next() {
		var lessonID, courseID string
		var rank int
		if err := rows.Scan(&lessonID, &courseID, &rank); err != nil {
			return nil, err
		}
		if result[courseID] == nil {
			result[courseID] = map[string]int{}
		}
		result[courseID][lessonID] = rank
	}
	return result, rows.Err()
}
```

- [ ] **Step 3: Verify build**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add repository/lesson.go
git commit -m "feat: add GetRanksForCourses to lesson repository"
```

---

### Task 3: Add GetByCoursesBatch to payment repository

**Files:**
- Modify: `repository/payment.go`

- [ ] **Step 1: Add method to the interface**

In `repository/payment.go`, add to `PaymentRepository` interface:
```go
GetByCoursesBatch(ctx context.Context, courseIDs []string) (map[string][]models.Payment, error)
```
Returns `map[courseID][]Payment` with payments sorted by `paid_at ASC` per course.

- [ ] **Step 2: Implement the method**

Add below the existing methods in `repository/payment.go`:
```go
func (r *paymentRepository) GetByCoursesBatch(ctx context.Context, courseIDs []string) (map[string][]models.Payment, error) {
	if len(courseIDs) == 0 {
		return map[string][]models.Payment{}, nil
	}
	rows, err := r.conn.Query(ctx,
		`SELECT id, course_id, amount, lessons_count, paid_at
		 FROM payments
		 WHERE course_id = ANY($1)
		 ORDER BY course_id, paid_at ASC`,
		courseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string][]models.Payment{}
	for rows.Next() {
		var p models.Payment
		if err := rows.Scan(&p.ID, &p.CourseID, &p.Amount, &p.LessonsCount, &p.PaidAt); err != nil {
			return nil, err
		}
		result[p.CourseID] = append(result[p.CourseID], p)
	}
	return result, rows.Err()
}
```

- [ ] **Step 3: Verify build**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add repository/payment.go
git commit -m "feat: add GetByCoursesBatch to payment repository"
```

---

### Task 4: Implement and test computeCyclePositions

**Files:**
- Modify: `service/lesson.go` (add helper)
- Create: `service/cycle_test.go` (unit tests)

- [ ] **Step 1: Create the test file first (TDD)**

Create `service/cycle_test.go`:
```go
package service

import (
	"testing"
	"tutorgo/models"

	"github.com/stretchr/testify/assert"
)

func TestComputeCyclePositions(t *testing.T) {
	t.Run("assigns positions across two payment cycles", func(t *testing.T) {
		// 3 lessons in cycle 1, 4 in cycle 2
		ranks := map[string]int{
			"l1": 1, "l2": 2, "l3": 3,
			"l4": 4, "l5": 5, "l6": 6, "l7": 7,
		}
		payments := []models.Payment{
			{LessonsCount: 3},
			{LessonsCount: 4},
		}

		result := computeCyclePositions(ranks, payments)

		assert.Equal(t, cycleInfo{Position: 1, Size: 3}, result["l1"])
		assert.Equal(t, cycleInfo{Position: 3, Size: 3}, result["l3"])
		assert.Equal(t, cycleInfo{Position: 1, Size: 4}, result["l4"])
		assert.Equal(t, cycleInfo{Position: 4, Size: 4}, result["l7"])
	})

	t.Run("lessons beyond total paid count have no entry", func(t *testing.T) {
		ranks := map[string]int{"l1": 1, "l2": 2, "l3": 3}
		payments := []models.Payment{{LessonsCount: 2}}

		result := computeCyclePositions(ranks, payments)

		assert.Contains(t, result, "l1")
		assert.Contains(t, result, "l2")
		assert.NotContains(t, result, "l3")
	})

	t.Run("no payments returns nil", func(t *testing.T) {
		ranks := map[string]int{"l1": 1}
		result := computeCyclePositions(ranks, nil)
		assert.Nil(t, result)
	})

	t.Run("empty ranks returns empty map", func(t *testing.T) {
		payments := []models.Payment{{LessonsCount: 8}}
		result := computeCyclePositions(map[string]int{}, payments)
		assert.Empty(t, result)
	})

	t.Run("single payment single lesson is both first and last", func(t *testing.T) {
		ranks := map[string]int{"l1": 1}
		payments := []models.Payment{{LessonsCount: 1}}

		result := computeCyclePositions(ranks, payments)

		assert.Equal(t, cycleInfo{Position: 1, Size: 1}, result["l1"])
	})
}
```

- [ ] **Step 2: Run tests — expect compile error (type not defined yet)**

```bash
go test ./service/ -run TestComputeCyclePositions -v
```
Expected: compile error — `cycleInfo` and `computeCyclePositions` undefined.

- [ ] **Step 3: Add cycleInfo type and computeCyclePositions to service/lesson.go**

Add at the bottom of `service/lesson.go` (before the last closing brace of the file, after existing methods):
```go
type cycleInfo struct {
	Position int
	Size     int
}

// computeCyclePositions returns cycle position info for each lesson in ranks.
// ranks: lessonID → 1-based rank (non-cancelled, per course).
// payments: sorted by paid_at ASC for the same course.
// Lessons whose rank exceeds total paid count are omitted from the result.
func computeCyclePositions(ranks map[string]int, payments []models.Payment) map[string]cycleInfo {
	if len(payments) == 0 {
		return nil
	}

	// Build cumulative upper bounds per payment cycle.
	bounds := make([]int, len(payments))
	cum := 0
	for i, p := range payments {
		cum += p.LessonsCount
		bounds[i] = cum
	}

	result := make(map[string]cycleInfo)
	for lessonID, rank := range ranks {
		for i, bound := range bounds {
			prev := 0
			if i > 0 {
				prev = bounds[i-1]
			}
			if rank > prev && rank <= bound {
				result[lessonID] = cycleInfo{
					Position: rank - prev,
					Size:     payments[i].LessonsCount,
				}
				break
			}
		}
	}
	return result
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./service/ -run TestComputeCyclePositions -v
```
Expected:
```
--- PASS: TestComputeCyclePositions/assigns_positions_across_two_payment_cycles
--- PASS: TestComputeCyclePositions/lessons_beyond_total_paid_count_have_no_entry
--- PASS: TestComputeCyclePositions/no_payments_returns_nil
--- PASS: TestComputeCyclePositions/empty_ranks_returns_empty_map
--- PASS: TestComputeCyclePositions/single_payment_single_lesson_is_both_first_and_last
PASS
```

- [ ] **Step 5: Commit**

```bash
git add service/lesson.go service/cycle_test.go
git commit -m "feat: add computeCyclePositions with unit tests"
```

---

### Task 5: Inject paymentRepo and enrich GetCalendar + GetByPeriod

**Files:**
- Modify: `service/lesson.go`

- [ ] **Step 1: Add paymentRepo field and update constructor**

Replace the struct and constructor in `service/lesson.go`:

```go
type lessonService struct {
	repo        repository.LessonRepository
	courseRepo  repository.CourseRepository
	paymentRepo repository.PaymentRepository
}

func NewLessonService(repo repository.LessonRepository, courseRepo repository.CourseRepository, paymentRepo repository.PaymentRepository) LessonService {
	return &lessonService{repo: repo, courseRepo: courseRepo, paymentRepo: paymentRepo}
}
```

- [ ] **Step 2: Add enrichWithCyclePositions helper**

Add this private helper below `NewLessonService` in `service/lesson.go`:
```go
// enrichWithCyclePositions annotates lessons with cycle position info.
// courseIDsFn returns the unique course IDs from whatever slice is passed.
func (s *lessonService) enrichCalendarLessons(ctx context.Context, lessons []models.CalendarLesson) error {
	if len(lessons) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	courseIDs := make([]string, 0)
	for _, l := range lessons {
		if !seen[l.CourseID] {
			seen[l.CourseID] = true
			courseIDs = append(courseIDs, l.CourseID)
		}
	}
	ranks, err := s.repo.GetRanksForCourses(ctx, courseIDs)
	if err != nil {
		return err
	}
	paymentsMap, err := s.paymentRepo.GetByCoursesBatch(ctx, courseIDs)
	if err != nil {
		return err
	}
	for i, l := range lessons {
		coursePayments := paymentsMap[l.CourseID]
		if len(coursePayments) == 0 {
			continue
		}
		infos := computeCyclePositions(ranks[l.CourseID], coursePayments)
		if info, ok := infos[l.ID]; ok {
			pos, size := info.Position, info.Size
			lessons[i].CyclePosition = &pos
			lessons[i].CycleSize = &size
		}
	}
	return nil
}

func (s *lessonService) enrichLessons(ctx context.Context, courseID string, lessons []models.Lesson) error {
	if len(lessons) == 0 {
		return nil
	}
	ranks, err := s.repo.GetRanksForCourses(ctx, []string{courseID})
	if err != nil {
		return err
	}
	paymentsMap, err := s.paymentRepo.GetByCoursesBatch(ctx, []string{courseID})
	if err != nil {
		return err
	}
	coursePayments := paymentsMap[courseID]
	if len(coursePayments) == 0 {
		return nil
	}
	infos := computeCyclePositions(ranks[courseID], coursePayments)
	for i, l := range lessons {
		if info, ok := infos[l.ID]; ok {
			pos, size := info.Position, info.Size
			lessons[i].CyclePosition = &pos
			lessons[i].CycleSize = &size
		}
	}
	return nil
}
```

- [ ] **Step 3: Update GetCalendar to call enrichCalendarLessons**

Replace the existing `GetCalendar` method:
```go
func (s *lessonService) GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error) {
	lessons, err := s.repo.GetCalendar(ctx, tutorID, from, to)
	if err != nil {
		return nil, err
	}
	if err := s.enrichCalendarLessons(ctx, lessons); err != nil {
		return nil, err
	}
	return lessons, nil
}
```

- [ ] **Step 4: Update GetByPeriod to call enrichLessons**

Replace the existing `GetByPeriod` method:
```go
func (s *lessonService) GetByPeriod(ctx context.Context, courseID string, tutorID string, from string, to string) ([]models.Lesson, error) {
	_, err := s.courseRepo.GetByID(ctx, courseID, tutorID)
	if err != nil {
		return nil, fmt.Errorf("course: %w", ErrNotFound)
	}
	lessons, err := s.repo.GetByPeriod(ctx, courseID, tutorID, from, to)
	if err != nil {
		return nil, err
	}
	if err := s.enrichLessons(ctx, courseID, lessons); err != nil {
		return nil, err
	}
	return lessons, nil
}
```

- [ ] **Step 5: Verify build (will fail — router not updated yet, that's fine)**

```bash
go build ./service/...
```
Expected: compiles cleanly.

- [ ] **Step 6: Commit**

```bash
git add service/lesson.go
git commit -m "feat: inject paymentRepo, enrich GetCalendar and GetByPeriod with cycle positions"
```

---

### Task 6: Update router wiring

**Files:**
- Modify: `router/router.go`

- [ ] **Step 1: Pass paymentRepo to NewLessonService**

In `router/router.go`, find line:
```go
lessonService := service.NewLessonService(lessonRepo, courseRepo)
```
Replace with:
```go
lessonService := service.NewLessonService(lessonRepo, courseRepo, paymentRepo)
```

- [ ] **Step 2: Build and run tests**

```bash
go build ./...
go test ./...
```
Expected: build passes. Tests may show compile errors in `lesson_test.go` — fix in Task 7.

- [ ] **Step 3: Commit after Task 7 fixes**

Hold this commit until Task 7 is done.

---

### Task 7: Update test mocks

**Files:**
- Modify: `service/lesson_test.go`
- Modify: `service/payment_test.go`

- [ ] **Step 1: Add GetRanksForCourses stub to mockLessonRepo in lesson_test.go**

Add to `mockLessonRepo` in `service/lesson_test.go`:
```go
func (m *mockLessonRepo) GetRanksForCourses(ctx context.Context, courseIDs []string) (map[string]map[string]int, error) {
	args := m.Called(ctx, courseIDs)
	return args.Get(0).(map[string]map[string]int), args.Error(1)
}
```

- [ ] **Step 2: Add GetByCoursesBatch stub to mockPaymentRepo in payment_test.go**

Add to `mockPaymentRepo` in `service/payment_test.go`:
```go
func (m *mockPaymentRepo) GetByCoursesBatch(ctx context.Context, courseIDs []string) (map[string][]models.Payment, error) {
	args := m.Called(ctx, courseIDs)
	return args.Get(0).(map[string][]models.Payment), args.Error(1)
}
```

- [ ] **Step 3: Update newLessonSvc to pass a mockPaymentRepo**

In `service/lesson_test.go`, update:
```go
func newLessonSvc(lessonRepo *mockLessonRepo, courseRepo *mockCourseRepo) service.LessonService {
	return service.NewLessonService(lessonRepo, courseRepo, new(mockPaymentRepo))
}
```

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```
Expected: all existing tests pass.

- [ ] **Step 5: Commit**

```bash
git add router/router.go service/lesson_test.go service/payment_test.go
git commit -m "feat: wire paymentRepo into lessonService, update test mocks"
```

---

### Task 8: Frontend — update TypeScript types

**Files:**
- Modify: `frontend/src/types/api.ts`

- [ ] **Step 1: Add optional cycle fields to Lesson and CalendarLesson**

In `frontend/src/types/api.ts`, update `Lesson`:
```ts
export interface Lesson {
  id: string
  course_id: string
  scheduled_at: string
  duration_minutes: number
  status: LessonStatus
  notes: string
  series_id?: string
  cycle_position?: number
  cycle_size?: number
}
```

Update `CalendarLesson`:
```ts
export interface CalendarLesson {
  id: string
  course_id: string
  scheduled_at: string
  duration_minutes: number
  status: LessonStatus
  notes: string
  subject: string
  student_name: string | null
  is_group: boolean
  cycle_position?: number
  cycle_size?: number
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/types/api.ts
git commit -m "feat: add cycle_position and cycle_size to Lesson and CalendarLesson types"
```

---

### Task 9: Create CycleBadge component

**Files:**
- Create: `frontend/src/components/lessons/CycleBadge.tsx`

- [ ] **Step 1: Create the component**

Create `frontend/src/components/lessons/CycleBadge.tsx`:
```tsx
interface CycleBadgeProps {
  position: number
  size: number
}

export function CycleBadge({ position, size }: CycleBadgeProps) {
  if (position === size) {
    return (
      <span className="inline-flex items-center justify-center w-4 h-4 rounded-full bg-red-500 text-white text-[9px] font-bold leading-none shrink-0">
        {position}
      </span>
    )
  }
  return (
    <span className="text-[9px] font-semibold opacity-60 leading-none shrink-0">
      {position}
    </span>
  )
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/lessons/CycleBadge.tsx
git commit -m "feat: add CycleBadge component"
```

---

### Task 10: Calendar page — render CycleBadge in eventContent

**Files:**
- Modify: `frontend/src/app/(dashboard)/calendar/page.tsx`

- [ ] **Step 1: Add cycle fields to extendedProps in lessonEvents mapping**

In `calendar/page.tsx`, find the `lessonEvents` mapping (around line 78). Update `extendedProps`:
```tsx
extendedProps: {
  type:            'lesson',
  courseId:        l.course_id,
  status:          l.status,
  notes:           l.notes,
  isGroup:         l.is_group,
  scheduledAt:     l.scheduled_at,
  durationMinutes: l.duration_minutes,
  cyclePosition:   l.cycle_position ?? null,
  cycleSize:       l.cycle_size ?? null,
},
```

- [ ] **Step 2: Import CycleBadge**

Add to imports at the top of `calendar/page.tsx`:
```tsx
import { CycleBadge } from '@/components/lessons/CycleBadge'
```

- [ ] **Step 3: Render badge in eventContent for lessons**

In `eventContent`, find the lesson branch (the `<>` fragment with `fc-event-time` and `fc-event-title`). Replace it with:
```tsx
const cyclePosition = arg.event.extendedProps.cyclePosition as number | null
const cycleSize     = arg.event.extendedProps.cycleSize as number | null
const cancelled     = arg.event.extendedProps.status === 'cancelled'
return (
  <div className="relative h-full w-full overflow-hidden">
    <div className="fc-event-time">{arg.timeText}</div>
    <div
      className="fc-event-title"
      style={cancelled ? { textDecoration: 'line-through' } : undefined}
    >
      {arg.event.title}
    </div>
    {cyclePosition != null && cycleSize != null && (
      <div className="absolute bottom-0.5 right-0.5">
        <CycleBadge position={cyclePosition} size={cycleSize} />
      </div>
    )}
  </div>
)
```

Note: remove the `const cancelled` line that existed before this block to avoid duplicate declaration.

- [ ] **Step 4: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit
```
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/app/\(dashboard\)/calendar/page.tsx
git commit -m "feat: render cycle position badge on calendar lessons"
```

---

### Task 11: Course page — render CycleBadge in lesson list

**Files:**
- Modify: `frontend/src/app/(dashboard)/courses/[id]/page.tsx`

- [ ] **Step 1: Import CycleBadge**

Add to imports at the top of `courses/[id]/page.tsx`:
```tsx
import { CycleBadge } from '@/components/lessons/CycleBadge'
```

- [ ] **Step 2: Render badge in lesson row**

Find the lesson list row (around line 479–483):
```tsx
<span className="text-muted-foreground shrink-0">{lesson.duration_minutes} мин</span>
<span className={`text-xs px-2 py-0.5 rounded-full shrink-0 ${STATUS_COLORS[lesson.status] ?? ''}`}>
  {STATUS_LABELS[lesson.status] ?? lesson.status}
</span>
```

After the status badge span, add:
```tsx
{lesson.cycle_position != null && lesson.cycle_size != null && (
  <CycleBadge position={lesson.cycle_position} size={lesson.cycle_size} />
)}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/\(dashboard\)/courses/\[id\]/page.tsx
git commit -m "feat: render cycle position badge in course lesson list"
```

---

### Task 12: Final verification

- [ ] **Step 1: Run all backend tests**

```bash
go test ./...
```
Expected: all pass.

- [ ] **Step 2: Build frontend**

```bash
cd frontend && npm run build
```
Expected: build succeeds with no TypeScript errors.

- [ ] **Step 3: Manual smoke test**

Start the backend (`air` or `go run .`) and frontend (`npm run dev`).

1. Open the calendar page — verify lessons show a small number in the bottom-right corner.
2. A lesson that is the last in a payment cycle should show a red circle badge.
3. Lessons with no payment assigned should show no badge.
4. Open a course page — verify the same badges appear in the lesson list.
5. Create a cancelled lesson — verify it does not shift the position numbers of subsequent lessons.
