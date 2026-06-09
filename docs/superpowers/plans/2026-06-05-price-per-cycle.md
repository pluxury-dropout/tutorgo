# Price Per Cycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `price_per_lesson` with `price_per_cycle` + `lessons_per_cycle` across all layers so tutors can enter a total cycle price instead of computing a per-lesson price manually.

**Architecture:** DB migration renames/replaces the column; Go models, repo, and test fixtures are updated mechanically; frontend CourseForm gets two new fields with a live "= X ₸ за урок" hint; all display pages and payment helpers use the new fields.

**Tech Stack:** Go/pgx (backend), Next.js/React + zod + react-hook-form (frontend), goose (migrations)

---

## File Map

| File | Change |
|---|---|
| `migrations/012_price_per_cycle.sql` | New migration |
| `models/course.go` | Replace `PricePerLesson` → `PricePerCycle` + `LessonsPerCycle` |
| `repository/course.go` | Update all 5 SQL queries |
| `repository/payment.go` | Update `GetMonthlyExpected` query |
| `handlers/mocks_test.go` | Update `testCourse` + `testCreateCourseReq` fixtures |
| `handlers/course_test.go` | Update validation test request body |
| `frontend/src/types/api.ts` | Update `Course` interface |
| `frontend/src/lib/api/courses.ts` | Update `CourseInput` interface |
| `frontend/src/schemas/course.ts` | Update zod schema |
| `frontend/src/components/courses/CourseForm.tsx` | Replace price field, add live hint |
| `frontend/src/app/(dashboard)/courses/[id]/page.tsx` | Update Row display + PaymentForm prop |
| `frontend/src/app/(dashboard)/courses/page.tsx` | Update table cell |
| `frontend/src/app/(dashboard)/students/[id]/page.tsx` | Update price display |
| `frontend/src/app/(dashboard)/payments/page.tsx` | Update `coursePriceMap` |

---

### Task 1: DB Migration

**Files:**
- Create: `migrations/012_price_per_cycle.sql`

- [ ] **Step 1: Write the migration**

Create `migrations/012_price_per_cycle.sql`:

```sql
-- +goose Up
ALTER TABLE courses
  ADD COLUMN price_per_cycle   numeric(10,2),
  ADD COLUMN lessons_per_cycle int;

UPDATE courses SET price_per_cycle = price_per_lesson, lessons_per_cycle = 1;

ALTER TABLE courses
  ALTER COLUMN price_per_cycle   SET NOT NULL,
  ALTER COLUMN lessons_per_cycle SET NOT NULL;

ALTER TABLE courses DROP COLUMN price_per_lesson;

-- +goose Down
ALTER TABLE courses
  ADD COLUMN price_per_lesson numeric(10,2);

UPDATE courses SET price_per_lesson = price_per_cycle / lessons_per_cycle;

ALTER TABLE courses
  ALTER COLUMN price_per_lesson SET NOT NULL;

ALTER TABLE courses
  DROP COLUMN price_per_cycle,
  DROP COLUMN lessons_per_cycle;
```

- [ ] **Step 2: Apply the migration**

```bash
goose -dir migrations postgres "$DB_URL" up
```

Expected: `OK    012_price_per_cycle.sql`

- [ ] **Step 3: Commit**

```bash
git add migrations/012_price_per_cycle.sql
git commit -m "feat: migrate price_per_lesson to price_per_cycle + lessons_per_cycle"
```

---

### Task 2: Go Model

**Files:**
- Modify: `models/course.go`

- [ ] **Step 1: Replace `models/course.go`**

```go
package models

import "time"

type Course struct {
	ID              string     `json:"id"`
	StudentID       *string    `json:"student_id"`
	TutorID         string     `json:"tutor_id"`
	Subject         string     `json:"subject"`
	PricePerCycle   float64    `json:"price_per_cycle"`
	LessonsPerCycle int        `json:"lessons_per_cycle"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at"`
}

type CourseBalance struct {
	LessonsPaid      int `json:"lessons_paid"`
	LessonsCompleted int `json:"lessons_completed"`
	LessonsRemaining int `json:"lessons_remaining"`
}

type CreateCourseRequest struct {
	StudentID       *string    `json:"student_id"        validate:"omitempty,uuid"`
	Subject         string     `json:"subject"           validate:"required,min=2"`
	PricePerCycle   float64    `json:"price_per_cycle"   validate:"required,gt=0"`
	LessonsPerCycle int        `json:"lessons_per_cycle" validate:"required,min=1"`
	StartedAt       time.Time  `json:"started_at"        validate:"required"`
	EndedAt         *time.Time `json:"ended_at"`
}

type UpdateCourseRequest struct {
	Subject         string     `json:"subject"           validate:"required,min=2"`
	PricePerCycle   float64    `json:"price_per_cycle"   validate:"required,gt=0"`
	LessonsPerCycle int        `json:"lessons_per_cycle" validate:"required,min=1"`
	StartedAt       time.Time  `json:"started_at"        validate:"required"`
	EndedAt         *time.Time `json:"ended_at"`
}
```

- [ ] **Step 2: Verify build fails with clear errors** (repo still uses old field names)

```bash
go build ./... 2>&1 | head -20
```

Expected: compile errors referencing `PricePerLesson` in `repository/course.go` and `handlers/mocks_test.go`.

---

### Task 3: Course Repository

**Files:**
- Modify: `repository/course.go`

- [ ] **Step 1: Replace `repository/course.go`**

```go
package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CourseRepository interface {
	Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error)
	GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Course, error)
	GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error)
	Delete(ctx context.Context, id string, tutorID string) error
}

type courseRepository struct {
	conn *pgxpool.Pool
}

func NewCourseRepository(conn *pgxpool.Pool) CourseRepository {
	return &courseRepository{conn: conn}
}

func (r *courseRepository) Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at`,
		req.StudentID, tutorID, req.Subject, req.PricePerCycle, req.LessonsPerCycle, req.StartedAt, req.EndedAt,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt)
	return course, err
}

func (r *courseRepository) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM courses
		 WHERE tutor_id = $1
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')`,
		tutorID, p.Search,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at
		 FROM courses
		 WHERE tutor_id = $1
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')
		 ORDER BY started_at DESC
		 LIMIT $3 OFFSET $4`,
		tutorID, p.Search, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	courses := []models.Course{}
	for rows.Next() {
		var course models.Course
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt); err != nil {
			return nil, 0, err
		}
		courses = append(courses, course)
	}
	return courses, total, rows.Err()
}

func (r *courseRepository) GetByID(ctx context.Context, id string, tutorID string) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at
		 FROM courses WHERE id = $1 AND tutor_id = $2`, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt)
	return course, err
}

func (r *courseRepository) GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at
		 FROM courses
		 WHERE tutor_id = $2 AND student_id = $1
		 UNION
		 SELECT c.id, c.student_id, c.tutor_id, c.subject, c.price_per_cycle, c.lessons_per_cycle, c.started_at, c.ended_at
		 FROM courses c
		 JOIN course_enrollments ce ON ce.course_id = c.id
		 WHERE c.tutor_id = $2 AND ce.student_id = $1
		 ORDER BY started_at DESC`,
		studentID, tutorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var courses []models.Course
	for rows.Next() {
		var course models.Course
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt); err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	return courses, rows.Err()
}

func (r *courseRepository) Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`UPDATE courses SET subject=$1, price_per_cycle=$2, lessons_per_cycle=$3, started_at=$4, ended_at=$5
		 WHERE id=$6 AND tutor_id=$7
		 RETURNING id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at`,
		req.Subject, req.PricePerCycle, req.LessonsPerCycle, req.StartedAt, req.EndedAt, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt)
	return course, err
}

func (r *courseRepository) Delete(ctx context.Context, id string, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM courses WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}
```

---

### Task 4: Payment Repository

**Files:**
- Modify: `repository/payment.go` (only `GetMonthlyExpected` function)

- [ ] **Step 1: Update `GetMonthlyExpected` query**

Find the `GetMonthlyExpected` function in `repository/payment.go` and replace its SQL query:

```go
func (r *paymentRepository) GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error) {
	var total float64
	err := r.conn.QueryRow(ctx,
		`SELECT COALESCE(SUM((c.price_per_cycle::float / c.lessons_per_cycle) * lc.cnt), 0)
		 FROM courses c
		 JOIN (
		     SELECT course_id, COUNT(*) AS cnt
		     FROM lessons
		     WHERE status IN ('scheduled', 'completed', 'missed')
		       AND scheduled_at >= date_trunc('month', NOW())
		       AND scheduled_at <  date_trunc('month', NOW()) + interval '1 month'
		     GROUP BY course_id
		 ) lc ON lc.course_id = c.id
		 WHERE c.tutor_id = $1`,
		tutorID,
	).Scan(&total)
	return total, err
}
```

---

### Task 5: Go Test Fixtures + Build Verification

**Files:**
- Modify: `handlers/mocks_test.go`
- Modify: `handlers/course_test.go`

- [ ] **Step 1: Update `testCourse` and `testCreateCourseReq` in `handlers/mocks_test.go`**

Find the `testCourse` and `testCreateCourseReq` var block and replace:

```go
var (
	testEndedAt         = func() *time.Time { t := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC); return &t }()
	testCreateCourseReq = models.CreateCourseRequest{
		StudentID:       testStudentIDPtr,
		Subject:         "Mathematics",
		PricePerCycle:   20000,
		LessonsPerCycle: 4,
		StartedAt:       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		EndedAt:         testEndedAt,
	}
)
```

Find the `testCourse` literal in `mocks_test.go` and replace:

```go
testCourse = models.Course{
    ID:              testCourseID,
    TutorID:         testTutorID,
    StudentID:       testStudentIDPtr,
    Subject:         "Mathematics",
    PricePerCycle:   20000,
    LessonsPerCycle: 4,
    StartedAt:       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
    EndedAt:         func() *time.Time { t := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC); return &t }(),
}
```

- [ ] **Step 2: Update validation test body in `handlers/course_test.go`**

Find `TestCourseCreate_ValidationError` and update the request body (the test sends a body missing `subject` — the key being tested is unchanged; just rename the price field in the body so it matches the new API):

```go
func TestCourseCreate_ValidationError(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	// subject is required
	w := makeRequest(t, r, http.MethodPost, "/courses", map[string]any{
		"student_id":        testStudentID,
		"price_per_cycle":   20000,
		"lessons_per_cycle": 4,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Create")
}
```

- [ ] **Step 3: Build and run tests**

```bash
go build ./... && go test ./...
```

Expected: `ok` for all packages, zero failures.

- [ ] **Step 4: Commit**

```bash
git add models/course.go repository/course.go repository/payment.go handlers/mocks_test.go handlers/course_test.go
git commit -m "feat: replace price_per_lesson with price_per_cycle + lessons_per_cycle in Go layers"
```

---

### Task 6: Frontend Types + API Client

**Files:**
- Modify: `frontend/src/types/api.ts`
- Modify: `frontend/src/lib/api/courses.ts`

- [ ] **Step 1: Update `Course` interface in `frontend/src/types/api.ts`**

Replace the `Course` interface:

```ts
export interface Course {
  id: string
  student_id: string | null
  tutor_id: string
  subject: string
  price_per_cycle: number
  lessons_per_cycle: number
  started_at: string
  ended_at: string | null
}
```

- [ ] **Step 2: Update `CourseInput` in `frontend/src/lib/api/courses.ts`**

Replace the `CourseInput` interface:

```ts
export interface CourseInput {
  student_id?: string
  subject: string
  price_per_cycle: number
  lessons_per_cycle: number
  started_at: string
  ended_at?: string
}
```

---

### Task 7: Zod Schema

**Files:**
- Modify: `frontend/src/schemas/course.ts`

- [ ] **Step 1: Replace `frontend/src/schemas/course.ts`**

```ts
import { z } from 'zod'

export const courseSchema = z
  .object({
    type:              z.enum(['individual', 'group']),
    student_id:        z.string().optional(),
    subject:           z.string().min(2, 'Минимум 2 символа'),
    price_per_cycle:   z
      .number({ error: 'Введите число' })
      .positive('Должно быть больше 0'),
    lessons_per_cycle: z
      .number({ error: 'Введите число' })
      .int('Только целое число')
      .min(1, 'Минимум 1 урок'),
    started_at: z.string().min(1, 'Выберите дату начала'),
    ended_at:   z.string().optional(),
  })
  .superRefine((data, ctx) => {
    if (data.type === 'individual' && !data.student_id) {
      ctx.addIssue({
        code:    z.ZodIssueCode.custom,
        message: 'Выберите ученика',
        path:    ['student_id'],
      })
    }
  })

export type CourseFormValues = z.infer<typeof courseSchema>
```

---

### Task 8: CourseForm

**Files:**
- Modify: `frontend/src/components/courses/CourseForm.tsx`

- [ ] **Step 1: Replace `CourseForm.tsx`**

```tsx
'use client'

import { useEffect } from 'react'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'

import { courseSchema, CourseFormValues } from '@/schemas/course'
import { Course, ApiError } from '@/types/api'
import { useStudents } from '@/lib/hooks/useStudents'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

interface CourseFormProps {
  open: boolean
  onClose: () => void
  onSubmit: (data: CourseFormValues) => Promise<void>
  initial?: Course
}

export function CourseForm({ open, onClose, onSubmit, initial }: CourseFormProps) {
  const { data: students = [] } = useStudents()

  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
    control,
    formState: { errors, isSubmitting },
  } = useForm<CourseFormValues>({
    resolver: zodResolver(courseSchema),
    defaultValues: { type: 'individual', subject: '', price_per_cycle: 0, lessons_per_cycle: 1, started_at: '', ended_at: '' },
  })

  const courseType      = watch('type')
  const pricePerCycle   = watch('price_per_cycle')
  const lessonsPerCycle = watch('lessons_per_cycle')
  const pricePerLesson  = lessonsPerCycle > 0 ? pricePerCycle / lessonsPerCycle : 0

  useEffect(() => {
    if (initial) {
      reset({
        type:              initial.student_id ? 'individual' : 'group',
        student_id:        initial.student_id ?? undefined,
        subject:           initial.subject,
        price_per_cycle:   initial.price_per_cycle,
        lessons_per_cycle: initial.lessons_per_cycle,
        started_at:        initial.started_at.slice(0, 10),
        ended_at:          initial.ended_at?.slice(0, 10) ?? '',
      })
    } else {
      reset({ type: 'individual', subject: '', price_per_cycle: 0, lessons_per_cycle: 1, started_at: '', ended_at: '' })
    }
  }, [initial, open, reset])

  async function submit(values: CourseFormValues) {
    try {
      await onSubmit(values)
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
          <DialogTitle>{initial ? 'Редактировать курс' : 'Новый курс'}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit(submit)} className="space-y-4 pt-2">
          {!initial && (
            <div className="space-y-1.5">
              <Label>Тип курса</Label>
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant={courseType === 'individual' ? 'default' : 'outline'}
                  onClick={() => setValue('type', 'individual')}
                >
                  Индивидуальный
                </Button>
                <Button
                  type="button"
                  variant={courseType === 'group' ? 'default' : 'outline'}
                  onClick={() => {
                    setValue('type', 'group')
                    setValue('student_id', undefined)
                  }}
                >
                  Групповой
                </Button>
              </div>
            </div>
          )}

          {/* Student select — individual courses only */}
          {courseType === 'individual' && !initial && (
            <div className="space-y-1.5">
              <Label htmlFor="student_id">Ученик</Label>
              <Controller
                name="student_id"
                control={control}
                render={({ field }) => (
                  <Select value={field.value ?? ''} onValueChange={field.onChange}>
                    <SelectTrigger id="student_id">
                      <SelectValue placeholder="Выберите ученика" />
                    </SelectTrigger>
                    <SelectContent>
                      {students.map((s) => (
                        <SelectItem key={s.id} value={s.id}>
                          {s.first_name}{s.last_name ? ` ${s.last_name}` : ''}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
              {errors.student_id && (
                <p className="text-xs text-destructive">{errors.student_id.message}</p>
              )}
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="subject">Предмет</Label>
            <Input id="subject" placeholder="Математика" {...register('subject')} />
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

---

### Task 9: Display Pages

**Files:**
- Modify: `frontend/src/app/(dashboard)/courses/[id]/page.tsx`
- Modify: `frontend/src/app/(dashboard)/courses/page.tsx`
- Modify: `frontend/src/app/(dashboard)/students/[id]/page.tsx`
- Modify: `frontend/src/app/(dashboard)/payments/page.tsx`

- [ ] **Step 1: Update course detail page — info rows and PaymentForm prop**

In `frontend/src/app/(dashboard)/courses/[id]/page.tsx`:

Find and replace this line:
```tsx
<Row label="Цена за урок" value={`${course.price_per_lesson.toLocaleString()} ₸`} />
```
With:
```tsx
<Row label="Цена за цикл" value={`${course.price_per_cycle.toLocaleString()} ₸`} />
<Row label="Уроков в цикле" value={String(course.lessons_per_cycle)} />
```

Find and replace the PaymentForm prop:
```tsx
pricePerLesson={course?.price_per_lesson ?? 0}
```
With:
```tsx
pricePerLesson={course ? course.price_per_cycle / course.lessons_per_cycle : 0}
```

- [ ] **Step 2: Update courses list page — table cell**

In `frontend/src/app/(dashboard)/courses/page.tsx`:

Find and replace:
```tsx
<td className="px-4 py-3 text-muted-foreground">{course.price_per_lesson.toLocaleString()} ₸</td>
```
With:
```tsx
<td className="px-4 py-3 text-muted-foreground">{course.price_per_cycle.toLocaleString()} ₸ / {course.lessons_per_cycle} ур.</td>
```

- [ ] **Step 3: Update students detail page — course price display**

In `frontend/src/app/(dashboard)/students/[id]/page.tsx`:

Find and replace:
```tsx
<span>{course.price_per_lesson.toLocaleString()} ₸/ур.</span>
```
With:
```tsx
<span>{Math.round(course.price_per_cycle / course.lessons_per_cycle).toLocaleString()} ₸/ур.</span>
```

- [ ] **Step 4: Update payments page — coursePriceMap**

In `frontend/src/app/(dashboard)/payments/page.tsx`:

Find and replace:
```tsx
const coursePriceMap = Object.fromEntries(courses.map((c) => [c.id, c.price_per_lesson]))
```
With:
```tsx
const coursePriceMap = Object.fromEntries(courses.map((c) => [c.id, c.price_per_cycle / c.lessons_per_cycle]))
```

---

### Task 10: TypeScript Build Verification

- [ ] **Step 1: Run TypeScript build**

```bash
cd frontend && npm run build
```

Expected: `✓ Compiled successfully` with no type errors.

- [ ] **Step 2: Commit all frontend changes**

```bash
git add frontend/src/types/api.ts \
        frontend/src/lib/api/courses.ts \
        frontend/src/schemas/course.ts \
        frontend/src/components/courses/CourseForm.tsx \
        "frontend/src/app/(dashboard)/courses/page.tsx" \
        "frontend/src/app/(dashboard)/courses/[id]/page.tsx" \
        "frontend/src/app/(dashboard)/students/[id]/page.tsx" \
        "frontend/src/app/(dashboard)/payments/page.tsx"
git commit -m "feat: update frontend to price_per_cycle + lessons_per_cycle with live hint"
```
