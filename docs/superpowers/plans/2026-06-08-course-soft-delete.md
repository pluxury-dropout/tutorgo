# Course Soft Delete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace hard-delete on courses with soft-delete (`is_active = FALSE`), preserving all lessons in the calendar and enabling archive/restore from the frontend.

**Architecture:** Add `is_active BOOLEAN NOT NULL DEFAULT TRUE` to `courses`; `DELETE /courses/:id` becomes an UPDATE; new endpoints `GET /courses/archived` and `POST /courses/:id/restore`; frontend courses page gets an Активные/Архив tab switcher.

**Tech Stack:** Go 1.22 / Gin / pgx v5 / goose — Next.js 14 / TanStack Query v5

---

## File Map

| File | Change |
|------|--------|
| `migrations/013_course_soft_delete.sql` | **create** — add `is_active` column |
| `models/course.go` | add `IsActive bool` to `Course` |
| `repository/course.go` | interface + impl: `GetAll` filter, `Delete`→archive, `GetAllArchived`, `Restore` |
| `service/course.go` | remove `lessonRepo`, simplify `Delete`, add `GetArchived`/`Restore` |
| `service/course_test.go` | update mock + delete tests + new tests |
| `service/lesson.go` | check `course.IsActive` in `Create`/`CreateBulk` |
| `handlers/course.go` | add `GetArchived`, `Restore` handlers |
| `router/router.go` | register two new routes; update `NewCourseService` call |
| `frontend/src/types/api.ts` | add `is_active: boolean` to `Course` |
| `frontend/src/lib/api/courses.ts` | add `listArchived`, `restore` |
| `frontend/src/lib/hooks/useCourses.ts` | add `useArchivedCoursesPaged`, `useRestoreCourse` |
| `frontend/src/app/(dashboard)/courses/page.tsx` | add tab switcher Активные/Архив |

---

## Task 1: Migration

**Files:**
- Create: `migrations/013_course_soft_delete.sql`

- [ ] **Step 1: Create migration file**

```sql
-- +goose Up
ALTER TABLE courses ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE;

-- +goose Down
ALTER TABLE courses DROP COLUMN is_active;
```

- [ ] **Step 2: Apply migration**

```bash
goose -dir migrations postgres "$DB_URL" up
```

Expected output:
```
2026/06/08 OK   013_course_soft_delete.sql
goose: successfully migrated database to version: 13
```

- [ ] **Step 3: Verify column exists**

```bash
psql "$DB_URL" -c "\d courses" | grep is_active
```

Expected: `is_active | boolean | not null | true`

- [ ] **Step 4: Commit**

```bash
git add migrations/013_course_soft_delete.sql
git commit -m "feat: add is_active column to courses for soft-delete"
```

---

## Task 2: Go Model

**Files:**
- Modify: `models/course.go`

- [ ] **Step 1: Add `IsActive` to `Course` struct**

Replace the `Course` struct in `models/course.go`:

```go
type Course struct {
	ID              string     `json:"id"`
	StudentID       *string    `json:"student_id"`
	TutorID         string     `json:"tutor_id"`
	Subject         string     `json:"subject"`
	PricePerCycle   float64    `json:"price_per_cycle"`
	LessonsPerCycle int        `json:"lessons_per_cycle"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at"`
	IsActive        bool       `json:"is_active"`
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

Expected: no output (clean build).

- [ ] **Step 3: Commit**

```bash
git add models/course.go
git commit -m "feat: add IsActive field to Course model"
```

---

## Task 3: Repository

**Files:**
- Modify: `repository/course.go`

- [ ] **Step 1: Update interface**

Replace the `CourseRepository` interface:

```go
type CourseRepository interface {
	Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error)
	GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Course, error)
	GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error)
	Delete(ctx context.Context, id string, tutorID string) error
	GetAllArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error)
	Restore(ctx context.Context, id string, tutorID string) error
}
```

- [ ] **Step 2: Update `Create` scan to include `is_active`**

Replace the `Create` method:

```go
func (r *courseRepository) Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active`,
		req.StudentID, tutorID, req.Subject, req.PricePerCycle, req.LessonsPerCycle, req.StartedAt, req.EndedAt,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive)
	return course, err
}
```

- [ ] **Step 3: Update `GetAll` to filter active only and scan `is_active`**

Replace the `GetAll` method:

```go
func (r *courseRepository) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM courses
		 WHERE tutor_id = $1 AND is_active = TRUE
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')`,
		tutorID, p.Search,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active
		 FROM courses
		 WHERE tutor_id = $1 AND is_active = TRUE
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
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive); err != nil {
			return nil, 0, err
		}
		courses = append(courses, course)
	}
	return courses, total, rows.Err()
}
```

- [ ] **Step 4: Update `GetByID` to scan `is_active`**

Replace the `GetByID` method:

```go
func (r *courseRepository) GetByID(ctx context.Context, id string, tutorID string) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active
		 FROM courses WHERE id = $1 AND tutor_id = $2`, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive)
	return course, err
}
```

- [ ] **Step 5: Update `GetByStudent` to scan `is_active`**

Replace the `GetByStudent` method:

```go
func (r *courseRepository) GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT c.id, c.student_id, c.tutor_id, c.subject, c.price_per_cycle, c.lessons_per_cycle, c.started_at, c.ended_at, c.is_active
		 FROM courses c
		 WHERE c.tutor_id = $2 AND c.student_id = $1
		 UNION
		 SELECT c.id, c.student_id, c.tutor_id, c.subject, c.price_per_cycle, c.lessons_per_cycle, c.started_at, c.ended_at, c.is_active
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
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive); err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	return courses, rows.Err()
}
```

- [ ] **Step 6: Update `Update` to scan `is_active`**

Replace the `Update` method:

```go
func (r *courseRepository) Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`UPDATE courses SET subject=$1, price_per_cycle=$2, lessons_per_cycle=$3, started_at=$4, ended_at=$5
		 WHERE id=$6 AND tutor_id=$7
		 RETURNING id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active`,
		req.Subject, req.PricePerCycle, req.LessonsPerCycle, req.StartedAt, req.EndedAt, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive)
	return course, err
}
```

- [ ] **Step 7: Change `Delete` to soft-delete**

Replace the `Delete` method:

```go
func (r *courseRepository) Delete(ctx context.Context, id string, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE courses SET is_active = FALSE WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}
```

- [ ] **Step 8: Add `GetAllArchived`**

Add after the `Delete` method:

```go
func (r *courseRepository) GetAllArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM courses
		 WHERE tutor_id = $1 AND is_active = FALSE
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')`,
		tutorID, p.Search,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active
		 FROM courses
		 WHERE tutor_id = $1 AND is_active = FALSE
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
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive); err != nil {
			return nil, 0, err
		}
		courses = append(courses, course)
	}
	return courses, total, rows.Err()
}
```

- [ ] **Step 9: Add `Restore`**

Add after `GetAllArchived`:

```go
func (r *courseRepository) Restore(ctx context.Context, id string, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE courses SET is_active = TRUE WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}
```

- [ ] **Step 10: Build to check for compile errors**

```bash
go build ./...
```

Expected: no output (clean build). If there are compile errors, the mock in `service/course_test.go` doesn't yet implement the new interface methods — that's OK for now, it will be fixed in Task 4.

- [ ] **Step 11: Commit**

```bash
git add repository/course.go
git commit -m "feat: update course repo for soft-delete (GetAllArchived, Restore)"
```

---

## Task 4: Course Service Tests (write failing tests first)

**Files:**
- Modify: `service/course_test.go`

- [ ] **Step 1: Add new mock methods to `mockCourseRepo`**

After the existing `Delete` mock method in `service/course_test.go`, add:

```go
func (m *mockCourseRepo) GetAllArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	args := m.Called(ctx, tutorID, p)
	return args.Get(0).([]models.Course), args.Int(1), args.Error(2)
}
func (m *mockCourseRepo) Restore(ctx context.Context, id string, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
}
```

- [ ] **Step 2: Update `newCourseSvc` — remove `lessonRepo` param**

Replace `newCourseSvc`:

```go
func newCourseSvc(courseRepo *mockCourseRepo, studentRepo *mockStudentRepo) service.CourseService {
	return service.NewCourseService(courseRepo, studentRepo)
}
```

- [ ] **Step 3: Update all `newCourseSvc` call sites and remove unused `lessonRepo` vars**

In every test function in `service/course_test.go` that currently creates `lessonRepo := new(mockLessonRepo)` and calls `newCourseSvc(courseRepo, studentRepo, lessonRepo)`:

Remove the `lessonRepo := new(mockLessonRepo)` line and change `newCourseSvc(courseRepo, studentRepo, lessonRepo)` → `newCourseSvc(courseRepo, studentRepo)`.

Functions to update: `TestCourseCreate_Success`, `TestCourseCreate_StudentNotFound`, `TestCourseCreate_RepoError`, `TestCourseGetAll_Success`, `TestCourseGetAll_Error`, `TestCourseGetByID_Success`, `TestCourseGetByID_NotFound`, `TestCourseUpdate_Success`, `TestCourseUpdate_NotFound`, `TestCourseUpdate_RepoError`, `TestCourseDelete_Success`, `TestCourseDelete_CourseNotFound`, `TestCourseDelete_HasLessons`, `TestCourseDelete_RepoError`.

- [ ] **Step 4: Rewrite the Delete tests**

Replace the four Delete tests with these three (remove `TestCourseDelete_HasLessons` entirely — archiving is always safe):

```go
func TestCourseDelete_Success(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	svc := newCourseSvc(courseRepo, studentRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	courseRepo.On("Delete", mock.Anything, courseID, tutorID).Return(nil)

	err := svc.Delete(context.Background(), courseID, tutorID)

	assert.NoError(t, err)
	courseRepo.AssertExpectations(t)
}

func TestCourseDelete_CourseNotFound(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	svc := newCourseSvc(courseRepo, studentRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	err := svc.Delete(context.Background(), courseID, tutorID)

	assert.ErrorIs(t, err, service.ErrNotFound)
	courseRepo.AssertNotCalled(t, "Delete")
	courseRepo.AssertExpectations(t)
}

func TestCourseDelete_RepoError(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	svc := newCourseSvc(courseRepo, studentRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	courseRepo.On("Delete", mock.Anything, courseID, tutorID).Return(errors.New("db error"))

	err := svc.Delete(context.Background(), courseID, tutorID)

	assert.Error(t, err)
	courseRepo.AssertExpectations(t)
}
```

- [ ] **Step 5: Add GetArchived tests**

Append at the bottom of `service/course_test.go`:

```go
// GetArchived

func TestCourseGetArchived_Success(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	svc := newCourseSvc(courseRepo, studentRepo)

	p := models.Pagination{Page: 1, Limit: 20}
	archived := []models.Course{{ID: courseID, TutorID: tutorID, IsActive: false}}
	courseRepo.On("GetAllArchived", mock.Anything, tutorID, p).Return(archived, 1, nil)

	courses, total, err := svc.GetArchived(context.Background(), tutorID, p)

	assert.NoError(t, err)
	assert.Equal(t, archived, courses)
	assert.Equal(t, 1, total)
	courseRepo.AssertExpectations(t)
}

func TestCourseGetArchived_Error(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	svc := newCourseSvc(courseRepo, studentRepo)

	p := models.Pagination{Page: 1, Limit: 20}
	courseRepo.On("GetAllArchived", mock.Anything, tutorID, p).Return([]models.Course{}, 0, errors.New("db error"))

	courses, total, err := svc.GetArchived(context.Background(), tutorID, p)

	assert.Error(t, err)
	assert.Empty(t, courses)
	assert.Equal(t, 0, total)
	courseRepo.AssertExpectations(t)
}

// Restore

func TestCourseRestore_Success(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	svc := newCourseSvc(courseRepo, studentRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{ID: courseID, TutorID: tutorID, IsActive: false}, nil)
	courseRepo.On("Restore", mock.Anything, courseID, tutorID).Return(nil)

	err := svc.Restore(context.Background(), courseID, tutorID)

	assert.NoError(t, err)
	courseRepo.AssertExpectations(t)
}

func TestCourseRestore_NotFound(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	svc := newCourseSvc(courseRepo, studentRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	err := svc.Restore(context.Background(), courseID, tutorID)

	assert.ErrorIs(t, err, service.ErrNotFound)
	courseRepo.AssertNotCalled(t, "Restore")
	courseRepo.AssertExpectations(t)
}
```

- [ ] **Step 6: Run tests — expect failures**

```bash
go test ./service/... -run TestCourse -v 2>&1 | tail -20
```

Expected: compile error — `CourseService` missing `GetArchived`/`Restore`, `NewCourseService` signature mismatch.

---

## Task 5: Course Service Implementation

**Files:**
- Modify: `service/course.go`

- [ ] **Step 1: Update `CourseService` interface**

Replace the `CourseService` interface:

```go
type CourseService interface {
	Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error)
	GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Course, error)
	GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error)
	Delete(ctx context.Context, id string, tutorID string) error
	GetArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error)
	Restore(ctx context.Context, id string, tutorID string) error
}
```

- [ ] **Step 2: Remove `lessonRepo` from struct and constructor**

Replace `courseService` struct and `NewCourseService`:

```go
type courseService struct {
	repo        repository.CourseRepository
	studentRepo repository.StudentRepository
}

func NewCourseService(repo repository.CourseRepository, studentRepo repository.StudentRepository) CourseService {
	return &courseService{repo: repo, studentRepo: studentRepo}
}
```

- [ ] **Step 3: Simplify `Delete` — no lesson guard**

Replace the `Delete` method:

```go
func (s *courseService) Delete(ctx context.Context, id string, tutorID string) error {
	_, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.Delete(ctx, id, tutorID)
}
```

- [ ] **Step 4: Add `GetArchived` and `Restore` methods**

Append after `Delete`:

```go
func (s *courseService) GetArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	return s.repo.GetAllArchived(ctx, tutorID, p)
}

func (s *courseService) Restore(ctx context.Context, id string, tutorID string) error {
	_, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.Restore(ctx, id, tutorID)
}
```

- [ ] **Step 5: Run tests — expect compile error in router**

```bash
go test ./service/... -run TestCourse -v 2>&1 | tail -20
```

Expected: service tests pass, but `router/router.go` won't compile because `NewCourseService` now takes 2 args.

- [ ] **Step 6: Fix router — update `NewCourseService` call**

In `router/router.go`, find:

```go
courseService := service.NewCourseService(courseRepo, studentRepo, lessonRepo)
```

Replace with:

```go
courseService := service.NewCourseService(courseRepo, studentRepo)
```

- [ ] **Step 7: Build and run all tests**

```bash
go build ./... && go test ./service/... -v 2>&1 | tail -30
```

Expected: clean build, all tests PASS.

- [ ] **Step 8: Commit**

```bash
git add service/course.go service/course_test.go router/router.go
git commit -m "feat: soft-delete courses (archive/restore service layer)"
```

---

## Task 6: Lesson Service — Block new lessons on archived courses

**Files:**
- Modify: `service/lesson.go`

- [ ] **Step 1: Write failing test in `service/lesson_test.go`**

Find the `TestLessonCreate_*` section in `service/lesson_test.go`. Add a new test after the existing ones:

```go
func TestLessonCreate_ArchivedCourse(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	payRepo := new(mockPaymentRepo)
	svc := service.NewLessonService(lessonRepo, courseRepo, payRepo)

	archivedCourse := models.Course{ID: courseID, TutorID: tutorID, IsActive: false}
	courseRepo.On("GetByID", mock.Anything, createLessonReq.CourseID, tutorID).Return(archivedCourse, nil)

	lesson, err := svc.Create(context.Background(), createLessonReq, tutorID)

	assert.ErrorIs(t, err, service.ErrConflict)
	assert.Empty(t, lesson)
	lessonRepo.AssertNotCalled(t, "Create")
	courseRepo.AssertExpectations(t)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./service/... -run TestLessonCreate_ArchivedCourse -v
```

Expected: FAIL — currently `Create` does not check `IsActive`.

- [ ] **Step 3: Update `Create` in `service/lesson.go`**

Replace the `Create` method:

```go
func (s *lessonService) Create(ctx context.Context, req models.CreateLessonRequest, tutorID string) (models.Lesson, error) {
	course, err := s.courseRepo.GetByID(ctx, req.CourseID, tutorID)
	if err != nil {
		return models.Lesson{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	if !course.IsActive {
		return models.Lesson{}, fmt.Errorf("course is archived: %w", ErrConflict)
	}
	lesson, err := s.repo.Create(ctx, req)
	if err == nil {
		globalCalendarCache.Invalidate(tutorID)
	}
	return lesson, err
}
```

- [ ] **Step 4: Update `CreateBulk` the same way**

Replace the `CreateBulk` method:

```go
func (s *lessonService) CreateBulk(ctx context.Context, req models.CreateBulkLessonRequest, tutorID string) ([]models.Lesson, error) {
	course, err := s.courseRepo.GetByID(ctx, req.CourseID, tutorID)
	if err != nil {
		return nil, fmt.Errorf("course: %w", ErrNotFound)
	}
	if !course.IsActive {
		return nil, fmt.Errorf("course is archived: %w", ErrConflict)
	}
	lessons, err := s.repo.CreateBulk(ctx, req)
	if err == nil {
		globalCalendarCache.Invalidate(tutorID)
	}
	return lessons, err
}
```

- [ ] **Step 5: Run the new test to verify it passes**

```bash
go test ./service/... -run TestLessonCreate_ArchivedCourse -v
```

Expected: PASS.

- [ ] **Step 6: Run all tests**

```bash
go test ./... 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add service/lesson.go service/lesson_test.go
git commit -m "feat: block lesson creation on archived courses"
```

---

## Task 7: Handlers + Router

**Files:**
- Modify: `handlers/course.go`
- Modify: `router/router.go`

- [ ] **Step 1: Add `GetArchived` handler to `handlers/course.go`**

Append after the `Delete` method:

```go
func (h *CourseHandler) GetArchived(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var p models.Pagination
	_ = c.ShouldBindQuery(&p)
	p.Normalize()

	courses, total, err := h.service.GetArchived(c.Request.Context(), tutorID, p)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.PagedResponse[models.Course]{
		Data: courses, Total: total, Page: p.Page, Limit: p.Limit,
	})
}
```

- [ ] **Step 2: Add `Restore` handler to `handlers/course.go`**

Append after `GetArchived`:

```go
func (h *CourseHandler) Restore(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	if err := h.service.Restore(c.Request.Context(), id, tutorID); err != nil {
		h.log.Error("Failed to restore course", slog.String("id", id), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Course restored", slog.String("id", id))
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 3: Register routes in `router/router.go`**

In `router/router.go`, find the courses route group:

```go
auth.GET("/courses", courseHandler.GetAll)
auth.POST("/courses", courseHandler.Create)
auth.GET("/courses/:id", courseHandler.GetByID)
auth.PUT("/courses/:id", courseHandler.Update)
auth.DELETE("/courses/:id", courseHandler.Delete)
```

Replace with (static `/archived` must appear before `/:id`):

```go
auth.GET("/courses", courseHandler.GetAll)
auth.POST("/courses", courseHandler.Create)
auth.GET("/courses/archived", courseHandler.GetArchived)
auth.GET("/courses/:id", courseHandler.GetByID)
auth.PUT("/courses/:id", courseHandler.Update)
auth.DELETE("/courses/:id", courseHandler.Delete)
auth.POST("/courses/:id/restore", courseHandler.Restore)
```

- [ ] **Step 4: Build**

```bash
go build ./...
```

Expected: no output (clean build).

- [ ] **Step 5: Smoke test the new endpoints**

Start the server:
```bash
go build -o ./tmp/main.exe . && ./tmp/main.exe &
```

Get a JWT (replace with a real token from your `.env`):
```bash
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"your@email.com","password":"yourpassword"}' | jq -r .token)

# List archived (should return empty array)
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/courses/archived | jq .

# Archive a course (replace COURSE_ID)
curl -s -X DELETE -H "Authorization: Bearer $TOKEN" http://localhost:8080/courses/COURSE_ID

# Verify it appears in archived
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/courses/archived | jq .

# Restore it
curl -s -X POST -H "Authorization: Bearer $TOKEN" http://localhost:8080/courses/COURSE_ID/restore
```

Expected: archive endpoint returns the course after DELETE; restore brings it back to active list.

Kill the server:
```bash
pkill -f tmp/main.exe
```

- [ ] **Step 6: Commit**

```bash
git add handlers/course.go router/router.go
git commit -m "feat: add GetArchived and Restore course handlers"
```

---

## Task 8: Frontend — Types, API, Hooks

**Files:**
- Modify: `frontend/src/types/api.ts`
- Modify: `frontend/src/lib/api/courses.ts`
- Modify: `frontend/src/lib/hooks/useCourses.ts`

- [ ] **Step 1: Add `is_active` to `Course` type**

In `frontend/src/types/api.ts`, replace the `Course` interface:

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
  is_active: boolean
}
```

- [ ] **Step 2: Add `listArchived` and `restore` to `coursesApi`**

In `frontend/src/lib/api/courses.ts`, add two entries to `coursesApi` after `listPaged`:

```ts
listArchived: (p: CourseListParams) =>
  api.get<PagedResponse<Course>>('/courses/archived', { params: p }).then((r) => r.data),
restore: (id: string) => api.post(`/courses/${id}/restore`).then(() => id),
```

- [ ] **Step 3: Add hooks to `useCourses.ts`**

In `frontend/src/lib/hooks/useCourses.ts`, add `archived` key to `courseKeys` and two new hooks.

Replace `courseKeys`:

```ts
export const courseKeys = {
  all:         ['courses'] as const,
  detail:      (id: string) => ['courses', id] as const,
  balance:     (id: string) => ['courses', id, 'balance'] as const,
  enrollments: (id: string) => ['courses', id, 'enrollments'] as const,
  byStudent:   (studentId: string) => ['courses', 'student', studentId] as const,
  archived:    ['courses', 'archived'] as const,
}
```

Append after `useCourseCount`:

```ts
export function useArchivedCoursesPaged(params: CourseListParams) {
  return useQuery({
    queryKey: [...courseKeys.archived, 'list', params],
    queryFn:  () => coursesApi.listArchived(params),
  })
}

export function useRestoreCourse() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: coursesApi.restore,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: courseKeys.all })
      qc.invalidateQueries({ queryKey: courseKeys.archived })
    },
  })
}
```

- [ ] **Step 4: Type-check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/types/api.ts frontend/src/lib/api/courses.ts frontend/src/lib/hooks/useCourses.ts
git commit -m "feat: add is_active type, listArchived/restore API, useArchivedCoursesPaged/useRestoreCourse hooks"
```

---

## Task 9: Frontend — Courses Page Tabs

**Files:**
- Modify: `frontend/src/app/(dashboard)/courses/page.tsx`

- [ ] **Step 1: Update imports**

Add `ArchiveRestore` to lucide imports and add new hooks. Replace the import block at the top of `page.tsx`:

```tsx
'use client'

import { Suspense, useState, useEffect, useRef } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { toast } from 'sonner'
import { BookOpen, Plus, Pencil, Trash2, ChevronRight, ArchiveRestore } from 'lucide-react'

import { useCoursesPaged, useCreateCourse, useUpdateCourse, useDeleteCourse, useArchivedCoursesPaged, useRestoreCourse } from '@/lib/hooks/useCourses'
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
```

- [ ] **Step 2: Add tab state, archived hooks, and update handleDelete**

Inside `CoursesPageInner`, after the existing hook and state declarations, add:

```tsx
const [tab, setTab] = useState<'active' | 'archive'>('active')
const [archivePage, setArchivePage] = useState(1)

const { data: archivedData, isLoading: archivedLoading } = useArchivedCoursesPaged({
  page: archivePage, limit: LIMIT, search,
})
const archivedCourses = archivedData?.data ?? []
const archivedTotal   = archivedData?.total ?? 0
const archivedPages   = Math.ceil(archivedTotal / LIMIT)

const restoreCourse = useRestoreCourse()
```

Replace the existing `handleDelete` function:

```tsx
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
```

- [ ] **Step 3: Add tab switcher and archive list to the JSX**

Replace the `return (...)` block inside `CoursesPageInner` with the full updated JSX:

```tsx
return (
  <div style={{ maxWidth: 900 }}>
    <PageHeader
      title="Курсы"
      description={tab === 'active' ? `${total} курсов` : `${archivedTotal} в архиве`}
      icon={BookOpen}
      iconBg="oklch(0.92 0.05 155)"
      iconColor="oklch(0.36 0.10 155)"
      actions={
        tab === 'active' ? (
          <Button size="sm" onClick={openCreate}>
            <Plus className="h-4 w-4 mr-1.5" /> Добавить
          </Button>
        ) : null
      }
    />

    {/* Tab switcher */}
    <div className="flex gap-1 mb-4" style={{ borderBottom: '1px solid var(--border)', paddingBottom: 0 }}>
      {(['active', 'archive'] as const).map((t) => (
        <button
          key={t}
          onClick={() => setTab(t)}
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
          <div>
            <div style={{
              display: 'grid',
              gridTemplateColumns: '1.5fr 130px 1fr 160px 90px 16px 72px',
              alignItems: 'baseline',
              gap: 12,
              paddingBottom: 8,
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
                  padding: '8px 0',
                  borderTop: i === 0 ? 'none' : '1px solid var(--border)',
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
          </div>
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
          <div>
            <div style={{
              display: 'grid',
              gridTemplateColumns: '1.5fr 130px 1fr 160px 90px 16px 120px',
              alignItems: 'baseline',
              gap: 12,
              paddingBottom: 8,
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
                  padding: '8px 0',
                  borderTop: i === 0 ? 'none' : '1px solid var(--border)',
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
          </div>
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
```

- [ ] **Step 4: Type-check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/app/(dashboard)/courses/page.tsx
git commit -m "feat: add Активные/Архив tabs to courses page"
```

---

## Final Verification

- [ ] **Run all Go tests**

```bash
go test ./... 2>&1 | tail -5
```

Expected: `ok` for all packages.

- [ ] **Start the full app and test the flow**

1. Open `/courses` — should show only active courses
2. Archive a course using the trash icon — confirm dialog appears, course disappears from Активные
3. Switch to Архив tab — archived course appears with "Восстановить" button
4. Click "Восстановить" — course moves back to Активные tab
5. Open `/calendar` — lessons from the archived course are still visible
6. Try to create a lesson on an archived course via API — should return `409 Conflict`
