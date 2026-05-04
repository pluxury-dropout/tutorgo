# Pagination Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add server-side search to students/courses API, add tutor-wide paginated payments endpoint, and wire up numbered pagination UI with URL state across Students, Courses, Payments pages and a count-only dashboard optimization.

**Architecture:** Backend adds `Search string` to the `Pagination` model (auto-bound from query), updates SQL with `ILIKE` filter for students and courses, and adds `GetAllByTutorPaged` to payments. Frontend adds a reusable `Pagination` component, new `*Paged` hooks that store state in URL via `useSearchParams`, and removes client-side filtering.

**Tech Stack:** Go (Gin, pgx), Next.js App Router (`useSearchParams`, `useRouter`), TanStack Query v5, Tailwind CSS, Lucide icons.

---

## File Map

**Backend — modify:**
- `models/pagination.go` — add `Search string` field
- `repository/student.go` — add search `ILIKE` to both SQL queries in `GetAll`
- `repository/course.go` — add search `ILIKE` to both SQL queries in `GetAll`
- `repository/payment.go` — add `GetAllByTutorPaged` method
- `service/payment.go` — add `GetAllByTutorPaged` to interface and impl
- `handlers/payment.go` — make `course_id` optional in `GetAll`; route to correct service method

**Backend — modify tests:**
- `handlers/student_test.go` — add `TestStudentGetAll_WithSearch`
- `handlers/course_test.go` — add `TestCourseGetAll_WithSearch`
- `handlers/payment_test.go` — update `TestPaymentGetAll_MissingCourseID`, add `TestPaymentGetAll_AllTutor`
- `handlers/mocks_test.go` — add `GetAllByTutorPaged` to `mockPaymentService`

**Frontend — modify:**
- `frontend/src/types/api.ts` — add `PagedResponse<T>` interface
- `frontend/src/lib/api/students.ts` — add `listPaged`, fix `list` to use `limit: 100`
- `frontend/src/lib/api/courses.ts` — add `listPaged`, fix `list` to use `limit: 100`
- `frontend/src/lib/api/payments.ts` — add `listPaged`
- `frontend/src/lib/hooks/useStudents.ts` — add `useStudentsPaged`, `useStudentCount`
- `frontend/src/lib/hooks/useCourses.ts` — add `useCoursesPaged`, `useCourseCount`
- `frontend/src/lib/hooks/usePayments.ts` — add `usePaymentsPaged`
- `frontend/src/app/(dashboard)/students/page.tsx` — URL state, Pagination component
- `frontend/src/app/(dashboard)/courses/page.tsx` — URL state, Pagination component
- `frontend/src/app/(dashboard)/payments/page.tsx` — paginated API, simplified stats
- `frontend/src/app/(dashboard)/dashboard/page.tsx` — `useStudentCount`, `useCourseCount`

**Frontend — create:**
- `frontend/src/components/common/Pagination.tsx` — numbered pagination component

---

## Task 1: Add `Search` to `Pagination`, update students backend

**Files:**
- Modify: `models/pagination.go`
- Modify: `repository/student.go`
- Modify: `handlers/student_test.go`

- [ ] **Step 1: Add `Search` field to `Pagination` struct**

In `models/pagination.go`, add the field:

```go
type Pagination struct {
	Page   int    `form:"page"`
	Limit  int    `form:"limit"`
	Search string `form:"search"`
}
```

`Normalize()` and `Offset()` need no changes — `Search` defaults to `""`.

- [ ] **Step 2: Update student repository `GetAll` SQL**

In `repository/student.go`, replace the `GetAll` method body:

```go
func (r *studentRepository) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Student, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM students
		 WHERE tutor_id = $1
		   AND ($2 = '' OR first_name ILIKE '%' || $2 || '%'
		                 OR last_name  ILIKE '%' || $2 || '%'
		                 OR email      ILIKE '%' || $2 || '%')`,
		tutorID, p.Search,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, tutor_id, first_name, last_name, phone, email, notes, active
		 FROM students
		 WHERE tutor_id = $1
		   AND ($2 = '' OR first_name ILIKE '%' || $2 || '%'
		                 OR last_name  ILIKE '%' || $2 || '%'
		                 OR email      ILIKE '%' || $2 || '%')
		 ORDER BY first_name, last_name
		 LIMIT $3 OFFSET $4`,
		tutorID, p.Search, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	students := []models.Student{}
	for rows.Next() {
		var student models.Student
		if err := rows.Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active); err != nil {
			return nil, 0, err
		}
		students = append(students, student)
	}
	return students, total, rows.Err()
}
```

- [ ] **Step 3: Run existing student tests to confirm nothing broke**

```bash
go test ./... -run TestStudent
```

Expected: all existing student tests PASS (Search `""` is backward-compatible).

- [ ] **Step 4: Add handler test for search**

In `handlers/student_test.go`, add after `TestStudentGetAll_ServiceError`:

```go
func TestStudentGetAll_WithSearch(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	p := models.Pagination{Page: 1, Limit: 20, Search: "Aiya"}
	expected := []models.Student{testStudent}
	svc.On("GetAll", mock.Anything, testTutorID, p).Return(expected, 1, nil)

	w := makeRequest(t, r, http.MethodGet, "/students?page=1&limit=20&search=Aiya", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.PagedResponse[models.Student]
	decodeJSON(t, w, &got)
	assert.Len(t, got.Data, 1)
	assert.Equal(t, 1, got.Total)
	svc.AssertExpectations(t)
}
```

- [ ] **Step 5: Run new test to verify it passes**

```bash
go test ./handlers/ -run TestStudentGetAll_WithSearch -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add models/pagination.go repository/student.go handlers/student_test.go
git commit -m "feat: add search param to students endpoint"
```

---

## Task 2: Add search to courses backend

**Files:**
- Modify: `repository/course.go`
- Modify: `handlers/course_test.go`

- [ ] **Step 1: Check existing course handler test for `GetAll`**

Open `handlers/course_test.go` and find `TestCourseGetAll_Success` — note the URL and `Pagination` struct used (should be `{Page:1, Limit:20}`). Existing test needs no change — `Search:""` is the zero value.

- [ ] **Step 2: Update course repository `GetAll` SQL**

In `repository/course.go`, replace the `GetAll` method body:

```go
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
		`SELECT id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at
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

	var courses []models.Course
	for rows.Next() {
		var course models.Course
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt); err != nil {
			return nil, 0, err
		}
		courses = append(courses, course)
	}
	return courses, total, rows.Err()
}
```

- [ ] **Step 3: Run existing course tests**

```bash
go test ./... -run TestCourse
```

Expected: all existing course tests PASS.

- [ ] **Step 4: Add handler test for search**

In `handlers/course_test.go`, add after the existing `GetAll` tests:

```go
func TestCourseGetAll_WithSearch(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	p := models.Pagination{Page: 1, Limit: 20, Search: "Math"}
	expected := []models.Course{testCourse}
	svc.On("GetAll", mock.Anything, testTutorID, p).Return(expected, 1, nil)

	w := makeRequest(t, r, http.MethodGet, "/courses?page=1&limit=20&search=Math", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.PagedResponse[models.Course]
	decodeJSON(t, w, &got)
	assert.Len(t, got.Data, 1)
	assert.Equal(t, 1, got.Total)
	svc.AssertExpectations(t)
}
```

- [ ] **Step 5: Run new test**

```bash
go test ./handlers/ -run TestCourseGetAll_WithSearch -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add repository/course.go handlers/course_test.go
git commit -m "feat: add search param to courses endpoint"
```

---

## Task 3: Add tutor-wide paginated payments endpoint

**Files:**
- Modify: `repository/payment.go`
- Modify: `service/payment.go`
- Modify: `handlers/payment.go`
- Modify: `handlers/mocks_test.go`
- Modify: `handlers/payment_test.go`

- [ ] **Step 1: Add `GetAllByTutorPaged` to payment repository interface and impl**

In `repository/payment.go`, add to the `PaymentRepository` interface:

```go
GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error)
```

Then add the implementation after `GetAllByTutor`:

```go
func (r *paymentRepository) GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments p
		 JOIN courses c ON c.id = p.course_id
		 WHERE c.tutor_id = $1`, tutorID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT p.id, p.course_id, p.amount, p.lessons_count, p.paid_at
		 FROM payments p
		 JOIN courses c ON c.id = p.course_id
		 WHERE c.tutor_id = $1
		 ORDER BY p.paid_at DESC
		 LIMIT $2 OFFSET $3`,
		tutorID, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	payments := []models.Payment{}
	for rows.Next() {
		var payment models.Payment
		if err := rows.Scan(&payment.ID, &payment.CourseID, &payment.Amount, &payment.LessonsCount, &payment.PaidAt); err != nil {
			return nil, 0, err
		}
		payments = append(payments, payment)
	}
	return payments, total, rows.Err()
}
```

- [ ] **Step 2: Add `GetAllByTutorPaged` to payment service interface and impl**

In `service/payment.go`, add to the `PaymentService` interface:

```go
GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error)
```

Add implementation in `paymentService`:

```go
func (s *paymentService) GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error) {
	return s.repo.GetAllByTutorPaged(ctx, tutorID, p)
}
```

- [ ] **Step 3: Update `GetAll` handler to make `course_id` optional**

In `handlers/payment.go`, replace the `GetAll` method:

```go
func (h *PaymentHandler) GetAll(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var p models.Pagination
	_ = c.ShouldBindQuery(&p)
	p.Normalize()

	courseID := c.Query("course_id")
	if courseID != "" {
		payments, total, err := h.service.GetByCourse(c.Request.Context(), courseID, tutorID, p)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, models.PagedResponse[models.Payment]{
			Data: payments, Total: total, Page: p.Page, Limit: p.Limit,
		})
		return
	}

	payments, total, err := h.service.GetAllByTutorPaged(c.Request.Context(), tutorID, p)
	if err != nil {
		h.log.Error("Failed to get all payments", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.PagedResponse[models.Payment]{
		Data: payments, Total: total, Page: p.Page, Limit: p.Limit,
	})
}
```

- [ ] **Step 4: Add `GetAllByTutorPaged` to payment mock in `handlers/mocks_test.go`**

In `handlers/mocks_test.go`, inside `mockPaymentService`, add:

```go
func (m *mockPaymentService) GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error) {
	args := m.Called(ctx, tutorID, p)
	return args.Get(0).([]models.Payment), args.Int(1), args.Error(2)
}
```

- [ ] **Step 5: Update and add payment handler tests**

In `handlers/payment_test.go`, update `TestPaymentGetAll_MissingCourseID` (it previously expected 400; now no `course_id` returns all tutor payments):

```go
func TestPaymentGetAll_AllTutor(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	p := models.Pagination{Page: 1, Limit: 20}
	svc.On("GetAllByTutorPaged", mock.Anything, testTutorID, p).Return([]models.Payment{testPayment}, 1, nil)

	w := makeRequest(t, r, http.MethodGet, "/payments?page=1&limit=20", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.PagedResponse[models.Payment]
	decodeJSON(t, w, &got)
	assert.Len(t, got.Data, 1)
	assert.Equal(t, 1, got.Total)
	svc.AssertExpectations(t)
}
```

Remove or rename `TestPaymentGetAll_MissingCourseID` — the 400 behavior no longer applies.

- [ ] **Step 6: Run all payment tests**

```bash
go test ./... -run TestPayment
```

Expected: all PASS.

- [ ] **Step 7: Run full test suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add repository/payment.go service/payment.go handlers/payment.go handlers/mocks_test.go handlers/payment_test.go
git commit -m "feat: add tutor-wide paginated payments endpoint"
```

---

## Task 4: Add `PagedResponse<T>` type and `Pagination.tsx` component

**Files:**
- Modify: `frontend/src/types/api.ts`
- Create: `frontend/src/components/common/Pagination.tsx`

- [ ] **Step 1: Add `PagedResponse<T>` to `types/api.ts`**

At the end of `frontend/src/types/api.ts`, add:

```ts
export interface PagedResponse<T> {
  data:  T[]
  total: number
  page:  number
  limit: number
}
```

- [ ] **Step 2: Create `Pagination.tsx` component**

Create `frontend/src/components/common/Pagination.tsx`:

```tsx
'use client'

import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface PaginationProps {
  page:         number
  totalPages:   number
  onPageChange: (page: number) => void
}

function pageNumbers(page: number, total: number): (number | '…')[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  if (page <= 4)            return [1, 2, 3, 4, 5, '…', total]
  if (page >= total - 3)    return [1, '…', total - 4, total - 3, total - 2, total - 1, total]
  return [1, '…', page - 1, page, page + 1, '…', total]
}

export function Pagination({ page, totalPages, onPageChange }: PaginationProps) {
  if (totalPages <= 1) return null
  const pages = pageNumbers(page, totalPages)
  return (
    <div className="flex items-center gap-1">
      <Button
        size="icon" variant="ghost" className="h-8 w-8"
        disabled={page <= 1}
        onClick={() => onPageChange(page - 1)}
      >
        <ChevronLeft className="h-4 w-4" />
      </Button>
      {pages.map((p, i) =>
        p === '…' ? (
          <span key={`e${i}`} className="px-2 text-sm text-muted-foreground">…</span>
        ) : (
          <Button
            key={p}
            size="icon"
            variant={p === page ? 'default' : 'ghost'}
            className="h-8 w-8 text-sm"
            onClick={() => onPageChange(p as number)}
          >
            {p}
          </Button>
        )
      )}
      <Button
        size="icon" variant="ghost" className="h-8 w-8"
        disabled={page >= totalPages}
        onClick={() => onPageChange(page + 1)}
      >
        <ChevronRight className="h-4 w-4" />
      </Button>
    </div>
  )
}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/types/api.ts frontend/src/components/common/Pagination.tsx
git commit -m "feat: add PagedResponse type and Pagination component"
```

---

## Task 5: Update students API functions and hooks

**Files:**
- Modify: `frontend/src/lib/api/students.ts`
- Modify: `frontend/src/lib/hooks/useStudents.ts`

- [ ] **Step 1: Update `students.ts` API**

Replace `frontend/src/lib/api/students.ts`:

```ts
import { api } from './client'
import { Student, PagedResponse } from '@/types/api'

export interface StudentInput {
  first_name: string
  last_name?: string
  email?: string
  phone?: string
}

export interface StudentListParams {
  page:   number
  limit:  number
  search: string
}

export const studentsApi = {
  list: () =>
    api.get<PagedResponse<Student>>('/students', { params: { page: 1, limit: 100 } })
      .then((r) => r.data.data),
  listPaged: (p: StudentListParams) =>
    api.get<PagedResponse<Student>>('/students', { params: p }).then((r) => r.data),
  get: (id: string) => api.get<Student>(`/students/${id}`).then((r) => r.data),
  create: (data: StudentInput) =>
    api.post<Student>('/students', data).then((r) => r.data),
  update: (id: string, data: StudentInput) =>
    api.put<Student>(`/students/${id}`, data).then((r) => r.data),
  delete: (id: string) => api.delete(`/students/${id}`).then(() => id),
}
```

Note: `list()` now passes `limit: 100` so dropdowns and name lookups get all students (up to 100). The backend caps at 100.

- [ ] **Step 2: Add `useStudentsPaged` and `useStudentCount` to `useStudents.ts`**

Add to the end of `frontend/src/lib/hooks/useStudents.ts`:

```ts
import { StudentListParams } from '@/lib/api/students'

export function useStudentsPaged(params: StudentListParams) {
  return useQuery({
    queryKey: [...studentKeys.all, 'list', params],
    queryFn:  () => studentsApi.listPaged(params),
  })
}

export function useStudentCount() {
  return useQuery({
    queryKey: [...studentKeys.all, 'count'],
    queryFn:  () => studentsApi.listPaged({ page: 1, limit: 1, search: '' }).then((r) => r.total),
  })
}
```

Update the import at the top of `frontend/src/lib/hooks/useStudents.ts` to:

```ts
import { studentsApi, StudentInput, StudentListParams } from '@/lib/api/students'
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api/students.ts frontend/src/lib/hooks/useStudents.ts
git commit -m "feat: add listPaged and useStudentsPaged/useStudentCount"
```

---

## Task 6: Update courses API functions and hooks

**Files:**
- Modify: `frontend/src/lib/api/courses.ts`
- Modify: `frontend/src/lib/hooks/useCourses.ts`

- [ ] **Step 1: Update `courses.ts` API**

In `frontend/src/lib/api/courses.ts`, add `CourseListParams` interface and `listPaged` function. Also update `list` to use `limit: 100`:

```ts
import { api } from './client'
import { Course, CourseBalance, Enrollment, PagedResponse } from '@/types/api'

export interface CourseInput {
  student_id?: string
  subject: string
  price_per_lesson: number
  started_at: string
  ended_at?: string
}

export interface CourseListParams {
  page:   number
  limit:  number
  search: string
}

export const coursesApi = {
  list: () =>
    api.get<PagedResponse<Course>>('/courses', { params: { page: 1, limit: 100 } })
      .then((r) => r.data.data ?? []),
  listPaged: (p: CourseListParams) =>
    api.get<PagedResponse<Course>>('/courses', { params: p }).then((r) => r.data),
  get: (id: string) => api.get<Course>(`/courses/${id}`).then((r) => r.data),
  create: (data: CourseInput) =>
    api.post<Course>('/courses', data).then((r) => r.data),
  update: (id: string, data: Omit<CourseInput, 'student_id'>) =>
    api.put<Course>(`/courses/${id}`, data).then((r) => r.data),
  delete: (id: string) => api.delete(`/courses/${id}`).then(() => id),
  getBalance: (id: string) =>
    api.get<CourseBalance>(`/payments/balance?course_id=${id}`).then((r) => r.data),
  getEnrollments: (id: string) =>
    api.get<Enrollment[]>(`/courses/${id}/enrollments`).then((r) => r.data ?? []),
  addEnrollment: (courseId: string, studentId: string) =>
    api.post<Enrollment>(`/courses/${courseId}/enrollments`, { student_id: studentId })
      .then((r) => r.data),
  removeEnrollment: (courseId: string, studentId: string) =>
    api.delete(`/courses/${courseId}/enrollments/${studentId}`).then(() => studentId),
  listByStudent: (studentId: string) =>
    api.get<Course[]>(`/students/${studentId}/courses`).then((r) => r.data ?? []),
}
```

- [ ] **Step 2: Add `useCoursesPaged` and `useCourseCount` to `useCourses.ts`**

Add imports at top of `frontend/src/lib/hooks/useCourses.ts`:

```ts
import { coursesApi, CourseInput, CourseListParams } from '@/lib/api/courses'
```

Add to end of file:

```ts
export function useCoursesPaged(params: CourseListParams) {
  return useQuery({
    queryKey: [...courseKeys.all, 'list', params],
    queryFn:  () => coursesApi.listPaged(params),
  })
}

export function useCourseCount() {
  return useQuery({
    queryKey: [...courseKeys.all, 'count'],
    queryFn:  () => coursesApi.listPaged({ page: 1, limit: 1, search: '' }).then((r) => r.total),
  })
}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api/courses.ts frontend/src/lib/hooks/useCourses.ts
git commit -m "feat: add listPaged and useCoursesPaged/useCourseCount"
```

---

## Task 7: Update payments API and hook

**Files:**
- Modify: `frontend/src/lib/api/payments.ts`
- Modify: `frontend/src/lib/hooks/usePayments.ts`

- [ ] **Step 1: Add `listPaged` to `payments.ts`**

In `frontend/src/lib/api/payments.ts`, add `PaymentListParams` and `listPaged`:

```ts
import { api } from './client'
import { Payment, PaymentBalance, PagedResponse } from '@/types/api'

export interface PaymentInput {
  course_id: string
  amount: number
  lessons_count: number
  paid_at?: string
}

export interface PaymentListParams {
  page:  number
  limit: number
}

export const paymentsApi = {
  list: (courseId: string) =>
    api.get<PagedResponse<Payment>>('/payments', { params: { course_id: courseId } })
      .then((r) => r.data.data ?? []),
  listPaged: (p: PaymentListParams) =>
    api.get<PagedResponse<Payment>>('/payments', { params: p }).then((r) => r.data),
  listRecent: () =>
    api.get<Payment[]>('/payments/recent').then((r) => r.data ?? []),
  create: (data: PaymentInput) =>
    api.post<Payment>('/payments', data).then((r) => r.data),
  getBalance: (courseId: string) =>
    api.get<PaymentBalance>('/payments/balance', { params: { course_id: courseId } })
      .then((r) => r.data),
  monthlyIncome: () =>
    api.get<{ total: number }>('/payments/monthly-income').then((r) => r.data.total),
}
```

- [ ] **Step 2: Add `usePaymentsPaged` to `usePayments.ts`**

In `frontend/src/lib/hooks/usePayments.ts`, add to imports:

```ts
import { paymentsApi, PaymentListParams } from '@/lib/api/payments'
```

Update `paymentKeys`:

```ts
export const paymentKeys = {
  byCourse:      (courseId: string) => ['payments', 'course', courseId] as const,
  paged:         (p: PaymentListParams) => ['payments', 'list', p] as const,
  recent:        ['payments', 'recent'] as const,
  monthlyIncome: ['payments', 'monthly-income'] as const,
}
```

Add at end of file:

```ts
export function usePaymentsPaged(params: PaymentListParams) {
  return useQuery({
    queryKey: paymentKeys.paged(params),
    queryFn:  () => paymentsApi.listPaged(params),
  })
}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api/payments.ts frontend/src/lib/hooks/usePayments.ts
git commit -m "feat: add usePaymentsPaged hook"
```

---

## Task 8: Students page — URL state + pagination

**Files:**
- Modify: `frontend/src/app/(dashboard)/students/page.tsx`

> Before writing, read `node_modules/next/dist/docs/` for the current version's `useSearchParams` requirements — specifically whether a `<Suspense>` boundary is needed.

- [ ] **Step 1: Rewrite students page**

Replace `frontend/src/app/(dashboard)/students/page.tsx`:

```tsx
'use client'

import { Suspense, useState, useEffect, useRef } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { toast } from 'sonner'
import { Users, Plus, Pencil, Trash2, ChevronRight } from 'lucide-react'

import { useStudentsPaged, useCreateStudent, useUpdateStudent, useDeleteStudent } from '@/lib/hooks/useStudents'
import { StudentForm } from '@/components/students/StudentForm'
import { PageHeader } from '@/components/common/PageHeader'
import { EmptyState } from '@/components/common/EmptyState'
import { Pagination } from '@/components/common/Pagination'
import { StudentFormValues } from '@/schemas/student'
import { Student } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const LIMIT = 20

function StudentsPageInner() {
  const router      = useRouter()
  const searchParams = useSearchParams()

  const page   = Math.max(1, Number(searchParams.get('page') ?? '1'))
  const search = searchParams.get('search') ?? ''

  const [localSearch, setLocalSearch] = useState(search)
  const mounted = useRef(false)

  // Sync input when URL changes externally (browser back/forward)
  useEffect(() => { setLocalSearch(search) }, [search])

  // Debounce URL write on search input
  useEffect(() => {
    if (!mounted.current) { mounted.current = true; return }
    if (localSearch === search) return
    const t = setTimeout(() => {
      const p = new URLSearchParams()
      if (localSearch) p.set('search', localSearch)
      p.set('page', '1')
      router.replace(`/students?${p}`)
    }, 300)
    return () => clearTimeout(t)
  }, [localSearch]) // eslint-disable-line react-hooks/exhaustive-deps

  function handlePageChange(newPage: number) {
    const p = new URLSearchParams(searchParams.toString())
    p.set('page', String(newPage))
    router.push(`/students?${p}`)
  }

  const { data, isLoading } = useStudentsPaged({ page, limit: LIMIT, search })
  const students   = data?.data ?? []
  const total      = data?.total ?? 0
  const totalPages = Math.ceil(total / LIMIT)

  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing]   = useState<Student | undefined>()

  const createStudent = useCreateStudent()
  const updateStudent = useUpdateStudent(editing?.id ?? '')
  const deleteStudent = useDeleteStudent()

  function openCreate() { setEditing(undefined); setFormOpen(true) }
  function openEdit(s: Student) { setEditing(s); setFormOpen(true) }

  async function handleSubmit(values: StudentFormValues) {
    if (editing) {
      await updateStudent.mutateAsync(values)
      toast.success('Ученик обновлён')
    } else {
      await createStudent.mutateAsync(values)
      toast.success('Ученик добавлен')
    }
  }

  async function handleDelete(s: Student) {
    if (!confirm(`Удалить ${s.first_name}${s.last_name ? ` ${s.last_name}` : ''}?`)) return
    await deleteStudent.mutateAsync(s.id)
    toast.success('Ученик удалён')
  }

  return (
    <>
      <PageHeader
        title="Ученики"
        description={`${total} учеников`}
        icon={Users}
        iconBg="oklch(0.94 0.03 280)"
        iconColor="oklch(0.42 0.14 280)"
        actions={
          <Button size="sm" onClick={openCreate}>
            <Plus className="h-4 w-4 mr-1.5" /> Добавить
          </Button>
        }
      />

      <div className="mb-4">
        <Input
          placeholder="Поиск по имени или email..."
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
      ) : students.length === 0 ? (
        <EmptyState
          icon={Users}
          title={search ? 'Ничего не найдено' : 'Нет учеников'}
          description={search ? 'Попробуй другой запрос' : 'Добавь первого ученика'}
          action={!search ? { label: 'Добавить ученика', onClick: openCreate } : undefined}
        />
      ) : (
        <>
          <div className="border rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/40">
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Имя</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Email</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Телефон</th>
                  <th className="w-4" />
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody>
                {students.map((student) => (
                  <tr
                    key={student.id}
                    className="border-b last:border-0 hover:bg-muted/30 cursor-pointer group"
                    onClick={() => router.push(`/students/${student.id}`)}
                  >
                    <td className="px-4 py-3 font-medium">
                      {student.first_name}{student.last_name ? ` ${student.last_name}` : ''}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">{student.email}</td>
                    <td className="px-4 py-3 text-muted-foreground">{student.phone || '—'}</td>
                    <td className="pr-1 py-3 w-4">
                      <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity duration-150" />
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-1" onClick={(e) => e.stopPropagation()}>
                        <Button size="icon" variant="ghost" className="h-8 w-8" onClick={() => openEdit(student)}>
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button size="icon" variant="ghost"
                          className="h-8 w-8 text-destructive hover:text-destructive"
                          onClick={() => handleDelete(student)}>
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

      <StudentForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSubmit={handleSubmit}
        initial={editing}
      />
    </>
  )
}

export default function StudentsPage() {
  return (
    <Suspense>
      <StudentsPageInner />
    </Suspense>
  )
}
```

- [ ] **Step 2: Start dev server and test students page manually**

```bash
cd frontend && npm run dev
```

Open http://localhost:3000/students. Verify:
- Table shows students (up to 20)
- Search input filters by calling API (watch Network tab — new request per search after 300ms)
- Pagination controls appear when > 20 students
- URL updates: `/students?page=2` or `/students?search=алия`
- Browser back navigates correctly

- [ ] **Step 3: Commit**

```bash
git add frontend/src/app/\(dashboard\)/students/page.tsx
git commit -m "feat: students page — URL-state pagination and server-side search"
```

---

## Task 9: Courses page — URL state + pagination

**Files:**
- Modify: `frontend/src/app/(dashboard)/courses/page.tsx`

- [ ] **Step 1: Rewrite courses page**

Replace `frontend/src/app/(dashboard)/courses/page.tsx`:

```tsx
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
```

- [ ] **Step 2: Test courses page manually**

Navigate to http://localhost:3000/courses. Verify:
- Table shows courses (up to 20)
- Search by subject name filters server-side
- Pagination controls appear when > 20 courses
- URL updates: `/courses?page=2&search=матем`
- CourseForm still works (student dropdown populated correctly)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/app/\(dashboard\)/courses/page.tsx
git commit -m "feat: courses page — URL-state pagination and server-side search"
```

---

## Task 10: Payments page — paginated API

**Files:**
- Modify: `frontend/src/app/(dashboard)/payments/page.tsx`

**Trade-off note:** The previous "За всё время" and "Прошлый месяц" stat cards required all payments loaded client-side. With pagination that's no longer feasible. This task replaces them with a single "Этот месяц" card (server-calculated via `useMonthlyIncome`) and a count from `total`.

- [ ] **Step 1: Rewrite payments page**

Replace `frontend/src/app/(dashboard)/payments/page.tsx`:

```tsx
'use client'

import { Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { ChevronRight, CreditCard } from 'lucide-react'

import { useCourses } from '@/lib/hooks/useCourses'
import { usePaymentsPaged, useMonthlyIncome } from '@/lib/hooks/usePayments'
import { PageHeader } from '@/components/common/PageHeader'
import { Pagination } from '@/components/common/Pagination'

const LIMIT = 20

function PaymentsPageInner() {
  const router       = useRouter()
  const searchParams = useSearchParams()

  const page = Math.max(1, Number(searchParams.get('page') ?? '1'))

  function handlePageChange(newPage: number) {
    const p = new URLSearchParams(searchParams.toString())
    p.set('page', String(newPage))
    router.push(`/payments?${p}`)
  }

  const { data: courses = [] }                       = useCourses()
  const { data: pagedPayments, isLoading }           = usePaymentsPaged({ page, limit: LIMIT })
  const { data: monthlyIncome = 0 }                  = useMonthlyIncome()

  const payments   = pagedPayments?.data ?? []
  const total      = pagedPayments?.total ?? 0
  const totalPages = Math.ceil(total / LIMIT)

  const courseMap = Object.fromEntries(courses.map((c) => [c.id, c.subject]))

  return (
    <>
      <PageHeader
        title="Платежи"
        description={`${total} записей`}
        icon={CreditCard}
        iconBg="var(--accent-light)"
        iconColor="oklch(0.52 0.18 55)"
      />

      <div className="bg-card border border-border rounded-[var(--radius-lg)] p-5 shadow-[var(--shadow-card)] mt-4 inline-block min-w-[200px]">
        <p className="text-xs font-medium text-muted-foreground mb-1">Этот месяц</p>
        <p className="text-[28px] font-bold leading-none bg-gradient-to-r from-amber-500 to-yellow-400 bg-clip-text text-transparent">
          {monthlyIncome.toLocaleString()} ₸
        </p>
      </div>

      <div className="border rounded-lg mt-4 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/40">
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">Дата</th>
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">Курс</th>
              <th className="text-right px-4 py-3 font-medium text-muted-foreground">Сумма</th>
              <th className="text-right px-4 py-3 font-medium text-muted-foreground">Уроков</th>
              <th className="w-4" />
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              [...Array(4)].map((_, i) => (
                <tr key={i}>
                  <td colSpan={5} className="px-4 py-3">
                    <div className="h-4 rounded bg-muted animate-pulse" />
                  </td>
                </tr>
              ))
            ) : payments.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-6 text-center text-muted-foreground">
                  Нет оплат
                </td>
              </tr>
            ) : (
              payments.map((p) => (
                <tr
                  key={p.id}
                  className="border-b last:border-0 hover:bg-muted/30 cursor-pointer group"
                  onClick={() => router.push(`/courses/${p.course_id}`)}
                >
                  <td className="px-4 py-3 text-muted-foreground">
                    {new Date(p.paid_at).toLocaleDateString('ru-RU')}
                  </td>
                  <td className="px-4 py-3 font-medium">{courseMap[p.course_id] ?? '—'}</td>
                  <td className="px-4 py-3 text-right font-medium">{p.amount.toLocaleString()} ₸</td>
                  <td className="px-4 py-3 text-right text-muted-foreground">{p.lessons_count} ур.</td>
                  <td className="pr-3 py-3 w-4">
                    <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity duration-150" />
                  </td>
                </tr>
              ))
            )}
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
  )
}

export default function PaymentsPage() {
  return (
    <Suspense>
      <PaymentsPageInner />
    </Suspense>
  )
}
```

- [ ] **Step 2: Test payments page manually**

Navigate to http://localhost:3000/payments. Verify:
- Single "Этот месяц" stat card shows correct monthly income
- Table shows paginated payments (up to 20 per page)
- Course names resolve correctly
- Pagination controls appear when > 20 payments
- Clicking a row navigates to the course page

- [ ] **Step 3: Commit**

```bash
git add frontend/src/app/\(dashboard\)/payments/page.tsx
git commit -m "feat: payments page — paginated API, remove client-side useQueries"
```

---

## Task 11: Dashboard — use resource counts

**Files:**
- Modify: `frontend/src/app/(dashboard)/dashboard/page.tsx`

**Trade-off note:** "Активных курсов" now shows total courses (not filtered by `ended_at`), since the count comes from a server `total`. The `activeCourses` client-side filter is removed.

- [ ] **Step 1: Update dashboard imports and hooks**

In `frontend/src/app/(dashboard)/dashboard/page.tsx`, replace the `useStudents` and `useCourses` imports and their usage:

Remove:
```ts
import { useStudents } from '@/lib/hooks/useStudents'
import { useCourses } from '@/lib/hooks/useCourses'
```

Add:
```ts
import { useStudentCount } from '@/lib/hooks/useStudents'
import { useCourseCount } from '@/lib/hooks/useCourses'
```

In `DashboardPage`, replace:
```ts
const { data: students  = [] } = useStudents()
const { data: courses   = [] } = useCourses()
// ...
const activeCourses = useMemo(
  () => courses.filter((c) => !c.ended_at),
  [courses],
)
```

With:
```ts
const { data: studentCount = 0 } = useStudentCount()
const { data: courseCount  = 0 } = useCourseCount()
```

Also remove the `useMemo` import if no longer used elsewhere in the file.

- [ ] **Step 2: Update stat cards that use the counts**

Replace in `DashboardPage` JSX:

```tsx
// Before:
<StatCard
  label="Активных учеников"
  value={students.length}
  ...
/>
<StatCard
  label="Активных курсов"
  value={activeCourses.length}
  ...
/>

// After:
<StatCard
  label="Учеников"
  value={studentCount}
  icon={<Users className="h-4 w-4" style={{ color: 'oklch(0.42 0.14 280)' }} />}
  iconBg="oklch(0.94 0.03 280)"
/>
<StatCard
  label="Курсов"
  value={courseCount}
  icon={<BookOpen className="h-4 w-4" style={{ color: 'oklch(0.36 0.10 155)' }} />}
  iconBg="oklch(0.92 0.05 155)"
/>
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 4: Test dashboard manually**

Navigate to http://localhost:3000/dashboard. Verify:
- "Учеников" and "Курсов" stat cards show correct counts
- "Уроков сегодня" and "Доход за месяц" still work
- Network tab: two lightweight requests `GET /students?page=1&limit=1` and `GET /courses?page=1&limit=1`

- [ ] **Step 5: Run full TypeScript check**

```bash
cd frontend && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/app/\(dashboard\)/dashboard/page.tsx
git commit -m "feat: dashboard — use resource counts instead of full list fetches"
```

---

## Done

All tasks complete. Verify end-to-end:
1. `go test ./...` — all Go tests pass
2. `cd frontend && npx tsc --noEmit` — no TypeScript errors
3. Manual smoke test on each page: students search + pagination, courses search + pagination, payments pagination, dashboard counts.
