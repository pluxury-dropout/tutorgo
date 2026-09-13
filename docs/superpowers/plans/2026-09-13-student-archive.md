# Фаза 1.5 — удаление ученика без потери истории: план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ученик без истории удаляется физически, ученик с историей уходит в архив (платежи, уроки, посещаемость остаются), уход из группы перестаёт стирать запись об участии.

**Architecture:** проверка истории встроена в `DELETE` одним запросом (иначе 409). Архив — существующая колонка `students.active` плюс оркестрация в `studentService.Archive` из уже готовых действий (архивация курса, закрытие записей, отзыв refresh-токенов). Уход из группы — `course_enrollments.left_at` (миграция 038) и фильтр `left_at IS NULL` там, где читается текущий состав.

**Tech Stack:** Go + Gin + pgx/v5, goose-миграции, testify/mock, интеграционные тесты с build-тегом `integration`; фронт — Next.js App Router + React Query.

**Spec:** `docs/specs/2026-09-06-price-units-and-student-centric-money.md`, раздел **5a** (фаза 1.5), а также п. 1.6, 3.9, 6.3, 6.5.

## Global Constraints

- Правки файлов — **только через Edit/Write**, не через `sed`/`python`/heredoc в Bash (CLAUDE.md).
- **Никогда не запускать `make migrate-up` / `make migrate-down` / `make migrate-status`**: `Makefile` делает `include .env`, и `$(DB_URL)` там — прод-Supabase. Миграции на тестовую БД — только `make migrate-test` или `goose` с `TEST_DB_URL`. Дефолт `TEST_DB_URL` — `postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable`; перед первым прогоном убедиться, что `TEST_DB_URL` не переопределён в `.env` на не-localhost.
- Интеграционный прогон по пакету: `make migrate-test && TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/ -run '<regex>' -v`.
- Известное красное, не связанное с фазой: `TestGetAllByTutor_CarriesSubjectAndStudentName` (с 2026-08-24). Любое другое падение — стоп.
- Ветка `feat/student-archive`. Не пушить, не мержить.
- Каждый коммит заканчивается строками:
  ```
  Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn
  ```
- Комментарии в коде — по-русски, в стиле соседнего кода: объясняют «почему», ссылаются на спеку как `(спека, п. 5a.N)`.
- Главный footgun проекта: расширил интерфейс репозитория или сервиса — обнови мок в `service/*_test.go` / `handlers/mocks_test.go` в том же коммите.
- Интеграционные тесты `repository/*_integration_test.go` — один пакет `repository_test`: имена хелперов не должны совпадать с существующими (`seedCourse`, `addLessons`, `addPayment`, `seedTutorStudent`, `addPriced`, `createFromCalendar`, `seedSeries`, `seedEventSeries`, `nextWeekday`, `seedTutorWithSubscription`, `testPool`, `seedPage`, `element`, `stored`, `gz`).

---

### Task 1: Миграция 038 и мягкий уход из группы

**Files:**
- Create: `migrations/038_enrollments_left_at.sql`
- Modify: `repository/enrollment.go` (интерфейс, `Add`, `AddBulk`, `Remove`, `GetByCourse`, новый `LeaveAllByStudent`)
- Modify: `repository/course.go` (`GetByStudent`, групповая ветка UNION)
- Modify: `repository/student.go` (`EnrolledInLesson`, `ListLessons`)
- Modify: `service/enrollment_test.go` (мок `mockEnrollmentRepo` — метод `LeaveAllByStudent`)
- Create: `repository/enrollment_integration_test.go`

**Interfaces:**
- Consumes: хелперы `testPool`, `seedTutorStudent` из существующих integration-тестов.
- Produces:
  - `EnrollmentRepository.LeaveAllByStudent(ctx context.Context, studentID string) error`
  - integration-хелперы (пакет `repository_test`): `addGroupCourse(t *testing.T, pool *pgxpool.Pool, tutorID, subject string) string`, `addLessonAt(t *testing.T, pool *pgxpool.Pool, courseID, atExpr, status string) string`, `leftAt(t *testing.T, pool *pgxpool.Pool, courseID, studentID string) (bool, *time.Time)`

- [ ] **Step 1: Написать падающие интеграционные тесты**

Create `repository/enrollment_integration_test.go`:

```go
//go:build integration

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"tutorgo/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Уход из группы мягкий (спека 2026-09-06-price-units…, п. 5a.4): строка записи
// остаётся с left_at, а текущий состав, доступ к уроку и будущие уроки кабинета
// её больше не видят. Запуск: make test-integration.

// addGroupCourse — групповой курс (student_id IS NULL) репетитора.
func addGroupCourse(t *testing.T, pool *pgxpool.Pool, tutorID, subject string) string {
	var id string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO courses (tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at)
		 VALUES ($1, $2, 5000, 1, NOW() - interval '1 month') RETURNING id`,
		tutorID, subject).Scan(&id))
	return id
}

// addLessonAt — один урок курса; atExpr — SQL-выражение времени, а не значение.
func addLessonAt(t *testing.T, pool *pgxpool.Pool, courseID, atExpr, status string) string {
	var id string
	require.NoError(t, pool.QueryRow(context.Background(),
		fmt.Sprintf(`INSERT INTO lessons (course_id, scheduled_at, duration_minutes, status)
		             VALUES ($1, %s, 60, $2) RETURNING id`, atExpr),
		courseID, status).Scan(&id))
	return id
}

// leftAt — есть ли строка записи и её left_at.
func leftAt(t *testing.T, pool *pgxpool.Pool, courseID, studentID string) (bool, *time.Time) {
	var left *time.Time
	err := pool.QueryRow(context.Background(),
		`SELECT left_at FROM course_enrollments WHERE course_id = $1 AND student_id = $2`,
		courseID, studentID).Scan(&left)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	require.NoError(t, err)
	return true, left
}

func TestEnrollmentRemove_KeepsRowAndHidesFromRoster(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)
	courseID := addGroupCourse(t, pool, tutorID, "Группа")
	enrollments := repository.NewEnrollmentRepository(pool)
	ctx := context.Background()

	_, err := enrollments.Add(ctx, courseID, studentID)
	require.NoError(t, err)
	require.NoError(t, enrollments.Remove(ctx, courseID, studentID))

	exists, left := leftAt(t, pool, courseID, studentID)
	assert.True(t, exists, "строка записи должна пережить уход")
	assert.NotNil(t, left)

	roster, err := enrollments.GetByCourse(ctx, courseID)
	require.NoError(t, err)
	assert.Empty(t, roster)

	courses, err := repository.NewCourseRepository(pool).GetByStudent(ctx, studentID, tutorID)
	require.NoError(t, err)
	assert.Empty(t, courses, "ушедший не видит группу на карточке ученика")
}

func TestEnrollmentAddBulk_ReturnsLeftStudentWithoutDuplicate(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)
	courseID := addGroupCourse(t, pool, tutorID, "Группа")
	enrollments := repository.NewEnrollmentRepository(pool)
	ctx := context.Background()

	_, err := enrollments.Add(ctx, courseID, studentID)
	require.NoError(t, err)
	require.NoError(t, enrollments.Remove(ctx, courseID, studentID))

	added, err := enrollments.AddBulk(ctx, courseID, []string{studentID}, tutorID)
	require.NoError(t, err)
	assert.Len(t, added, 1, "возвращение ушедшего — это запись")

	exists, left := leftAt(t, pool, courseID, studentID)
	assert.True(t, exists)
	assert.Nil(t, left)

	var rows int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM course_enrollments WHERE course_id = $1`, courseID).Scan(&rows))
	assert.Equal(t, 1, rows)

	again, err := enrollments.AddBulk(ctx, courseID, []string{studentID}, tutorID)
	require.NoError(t, err)
	assert.Empty(t, again, "уже состоящий молча пропускается, как раньше")
}

func TestEnrollmentAdd_ReturnsLeftStudent(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)
	courseID := addGroupCourse(t, pool, tutorID, "Группа")
	enrollments := repository.NewEnrollmentRepository(pool)
	ctx := context.Background()

	_, err := enrollments.Add(ctx, courseID, studentID)
	require.NoError(t, err)
	require.NoError(t, enrollments.Remove(ctx, courseID, studentID))

	e, err := enrollments.Add(ctx, courseID, studentID)
	require.NoError(t, err)
	assert.Equal(t, studentID, e.StudentID)
	_, left := leftAt(t, pool, courseID, studentID)
	assert.Nil(t, left)

	_, err = enrollments.Add(ctx, courseID, studentID)
	assert.Error(t, err, "запись уже состоящего остаётся ошибкой, как раньше")
}

func TestEnrolledInLesson_FalseAfterLeave(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)
	courseID := addGroupCourse(t, pool, tutorID, "Группа")
	lessonID := addLessonAt(t, pool, courseID, "NOW() + interval '1 day'", "scheduled")
	enrollments := repository.NewEnrollmentRepository(pool)
	students := repository.NewStudentRepository(pool)
	ctx := context.Background()

	_, err := enrollments.Add(ctx, courseID, studentID)
	require.NoError(t, err)
	ok, err := students.EnrolledInLesson(ctx, studentID, lessonID)
	require.NoError(t, err)
	assert.True(t, ok)

	require.NoError(t, enrollments.Remove(ctx, courseID, studentID))
	ok, err = students.EnrolledInLesson(ctx, studentID, lessonID)
	require.NoError(t, err)
	assert.False(t, ok, "ушедшего не пускает на урок и доску группы")
}

func TestListLessons_HidesGroupLessonsAfterLeave(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)
	courseID := addGroupCourse(t, pool, tutorID, "Группа")
	past := addLessonAt(t, pool, courseID, "NOW() - interval '3 days'", "completed")
	addLessonAt(t, pool, courseID, "NOW() + interval '3 days'", "scheduled")
	ctx := context.Background()

	_, err := repository.NewEnrollmentRepository(pool).Add(ctx, courseID, studentID)
	require.NoError(t, err)
	// Ушёл позавчера: прошлый урок был при нём, будущий — уже нет.
	_, err = pool.Exec(ctx,
		`UPDATE course_enrollments SET left_at = NOW() - interval '2 days'
		 WHERE course_id = $1 AND student_id = $2`, courseID, studentID)
	require.NoError(t, err)

	students := repository.NewStudentRepository(pool)
	pastLessons, err := students.ListLessons(ctx, studentID, true)
	require.NoError(t, err)
	require.Len(t, pastLessons, 1)
	assert.Equal(t, past, pastLessons[0].ID)

	upcoming, err := students.ListLessons(ctx, studentID, false)
	require.NoError(t, err)
	assert.Empty(t, upcoming)
}

func TestLeaveAllByStudent_KeepsEarlierLeaveDate(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)
	first := addGroupCourse(t, pool, tutorID, "Группа 1")
	second := addGroupCourse(t, pool, tutorID, "Группа 2")
	enrollments := repository.NewEnrollmentRepository(pool)
	ctx := context.Background()

	for _, courseID := range []string{first, second} {
		_, err := enrollments.Add(ctx, courseID, studentID)
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx,
		`UPDATE course_enrollments SET left_at = '2026-01-01T00:00:00Z'
		 WHERE course_id = $1 AND student_id = $2`, first, studentID)
	require.NoError(t, err)

	require.NoError(t, enrollments.LeaveAllByStudent(ctx, studentID))

	_, leftFirst := leftAt(t, pool, first, studentID)
	require.NotNil(t, leftFirst)
	assert.Equal(t, 2026, leftFirst.Year(), "прежняя дата ухода — история, не переписывается")
	assert.Equal(t, time.January, leftFirst.Month())

	_, leftSecond := leftAt(t, pool, second, studentID)
	assert.NotNil(t, leftSecond)
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются / падают**

Run: `go vet -tags=integration ./repository/`
Expected: ошибка компиляции `enrollments.LeaveAllByStudent undefined`.

- [ ] **Step 3: Миграция**

Create `migrations/038_enrollments_left_at.sql`:

```sql
-- +goose Up
-- Уход из группы мягкий: строка записи — кусок истории участия. Фаза 2 строит
-- на ней разметку легаси-платежей и периоды участия, а DELETE стирал и то и
-- другое (docs/specs/2026-09-06-price-units-and-student-centric-money.md, п. 5a.4).
ALTER TABLE course_enrollments ADD COLUMN left_at TIMESTAMPTZ;  -- NULL = занимается

-- +goose Down
-- Прежний смысл «убран — строки нет»: иначе после отката ушедшие снова в группе.
DELETE FROM course_enrollments WHERE left_at IS NOT NULL;
ALTER TABLE course_enrollments DROP COLUMN left_at;
```

- [ ] **Step 4: Репозиторий записей**

`repository/enrollment.go` — в интерфейс `EnrollmentRepository` добавить последней строкой:

```go
	LeaveAllByStudent(ctx context.Context, studentID string) error
```

`Add` целиком заменить на:

```go
// Add записывает ученика в группу. Возвращение ушедшего снимает left_at: строка
// одна на пару (UNIQUE), и запись продолжается. Запись уже состоящего ничего не
// обновляет — RETURNING пуст, и это ошибка, как раньше была ошибка уникальности.
//
// ponytail: возвращение склеивает периоды участия в один — после фазы 2 уроки
// между уходом и возвращением сгорят. Таблица периодов — когда «ушёл и вернулся
// в ту же группу» станет частым (спека, п. 5a.4).
func (r *enrollmentRepository) Add(ctx context.Context, courseID string, studentID string) (models.CourseEnrollment, error) {
	var e models.CourseEnrollment
	err := r.pool.QueryRow(ctx,
		`INSERT INTO course_enrollments (course_id, student_id)
		 VALUES ($1, $2)
		 ON CONFLICT (course_id, student_id) DO UPDATE SET left_at = NULL
		 WHERE course_enrollments.left_at IS NOT NULL
		 RETURNING id, course_id, student_id`,
		courseID, studentID,
	).Scan(&e.ID, &e.CourseID, &e.StudentID)
	return e, err
}
```

В `AddBulk` заменить докблок и SQL (остальное тело без изменений):

```go
// AddBulk записывает в группу сразу нескольких учеников. Вставка идёт SELECT'ом
// из students — чужие ученики отсеиваются там же, отдельной проверкой владения
// по одному запросу на каждого. Повторная запись уже состоящего в группе
// молча пропускается: собрать группу дважды — не ошибка пользователя. Ушедший
// (left_at задан) возвращается в группу той же строкой.
```

```go
		`INSERT INTO course_enrollments (course_id, student_id)
		 SELECT $1::uuid, s.id
		 FROM students s
		 WHERE s.tutor_id = $2::uuid AND s.id = ANY($3::uuid[])
		 ON CONFLICT (course_id, student_id) DO UPDATE SET left_at = NULL
		 WHERE course_enrollments.left_at IS NOT NULL
		 RETURNING id, course_id, student_id`,
```

`Remove` целиком заменить на:

```go
// Remove — мягкий уход: строка остаётся с датой ухода. Она кусок истории
// участия, и фазе 2 нужна для разметки легаси-платежей группы (спека, п. 5a.4).
func (r *enrollmentRepository) Remove(ctx context.Context, courseID string, studentID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE course_enrollments SET left_at = NOW()
		 WHERE course_id = $1 AND student_id = $2 AND left_at IS NULL`,
		courseID, studentID)
	return err
}

// LeaveAllByStudent закрывает все текущие записи ученика — шаг архивации
// (спека, п. 5a.3). Уже закрытые не трогает: прежняя дата ухода — история.
// Владение учеником проверяет вызывающий сервис.
func (r *enrollmentRepository) LeaveAllByStudent(ctx context.Context, studentID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE course_enrollments SET left_at = NOW()
		 WHERE student_id = $1 AND left_at IS NULL`, studentID)
	return err
}
```

В `GetByCourse` SQL:

```go
		`SELECT ce.id, ce.course_id, ce.student_id, s.first_name, s.last_name
		 FROM course_enrollments ce
		 JOIN students s ON s.id = ce.student_id
		 WHERE ce.course_id = $1 AND ce.left_at IS NULL`, courseID)
```

- [ ] **Step 5: Карточка ученика, доступ к уроку, кабинет**

`repository/course.go`, `GetByStudent` — групповая ветка UNION:

```go
		 SELECT c.id, c.student_id, c.tutor_id, c.subject, c.price_per_cycle, c.lessons_per_cycle, c.started_at, c.ended_at, c.is_active
		 FROM courses c
		 JOIN course_enrollments ce ON ce.course_id = c.id
		 WHERE c.tutor_id = $2 AND ce.student_id = $1 AND ce.left_at IS NULL AND c.is_active = TRUE
```

`repository/student.go`, `EnrolledInLesson` — внутренний EXISTS:

```go
		     OR EXISTS (SELECT 1 FROM course_enrollments ce
		                WHERE ce.course_id=c.id AND ce.student_id=$1 AND ce.left_at IS NULL)
```

`repository/student.go`, `ListLessons` — дополнить комментарий над `base` строкой:

```go
	// Уроки группы после ухода (left_at) не показываются — та же граница, что
	// в burned фазы 2; прошлые остаются историей (спека, п. 5a.4).
```

и заменить хвост `base` (от `FROM lessons l` до конца строки):

```go
	         FROM lessons l
	         JOIN courses c ON c.id = l.course_id
	         LEFT JOIN students s ON s.id = c.student_id
	         LEFT JOIN ranked r ON r.id = l.id
	         LEFT JOIN course_enrollments ce ON ce.course_id = l.course_id AND ce.student_id = $1
	         WHERE l.course_id IN (SELECT id FROM stu_courses)
	           AND l.scheduled_at < COALESCE(ce.left_at, 'infinity'::timestamptz)`
```

`UNIQUE(course_id, student_id)` гарантирует не больше одной строки `ce` на урок — дублей `LEFT JOIN` не даёт.

- [ ] **Step 6: Мок**

`service/enrollment_test.go`, после метода `GetByCourse` мока `mockEnrollmentRepo`:

```go
func (m *mockEnrollmentRepo) LeaveAllByStudent(ctx context.Context, studentID string) error {
	return m.Called(ctx, studentID).Error(0)
}
```

- [ ] **Step 7: Прогнать**

Run:
```bash
go build ./... && go test ./service/ ./handlers/ && \
make migrate-test && \
TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" \
  go test -tags=integration -count=1 ./repository/ -run 'TestEnrollment|TestEnrolledInLesson|TestListLessons|TestLeaveAllByStudent' -v
```
Expected: все шесть тестов PASS, `go test` без FAIL.

- [ ] **Step 8: Commit**

```bash
git add migrations/038_enrollments_left_at.sql repository/enrollment.go repository/course.go repository/student.go service/enrollment_test.go repository/enrollment_integration_test.go
git commit -m "feat(enrollments): мягкий уход из группы — left_at вместо DELETE" -m "Строка записи — история участия: фазе 2 она нужна для разметки легаси-платежей
группы. Текущий состав, карточка ученика, доступ к уроку и будущие уроки
кабинета фильтруют left_at; возвращение в группу снимает уход той же строкой." -m "Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

---

### Task 2: Удаление ученика — только без истории

**Files:**
- Modify: `repository/student.go` (интерфейс и `Delete`)
- Modify: `service/student.go` (`Delete`)
- Modify: `service/student_test.go` (мок `Delete`, `TestDeleteStudent_Success`, новый тест)
- Modify: `handlers/student_test.go` (новый тест 409)
- Create: `repository/student_integration_test.go`

**Interfaces:**
- Consumes: из Task 1 — `addGroupCourse`, `addLessonAt`; существующие `testPool`, `seedTutorStudent`, `addPayment`.
- Produces:
  - `StudentRepository.Delete(ctx context.Context, id string, tutorID string) (bool, error)` — `false`, если ничего не удалено.
  - `studentService.Delete` возвращает ошибку с `service.ErrConflict`, когда у ученика есть история.
  - integration-хелперы: `addIndividualCourse(t *testing.T, pool *pgxpool.Pool, tutorID, studentID string) string`, `studentExists(t *testing.T, pool *pgxpool.Pool, studentID string) bool`

- [ ] **Step 1: Падающий интеграционный тест**

Create `repository/student_integration_test.go`:

```go
//go:build integration

package repository_test

import (
	"context"
	"testing"

	"tutorgo/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ученик удаляется физически, только если у него нет истории — платежей,
// проведённых уроков, отметок посещаемости (спека 2026-09-06-price-units…,
// п. 5a.1–5a.2). Запуск: make test-integration.

// addIndividualCourse — индивидуальный курс ученика.
func addIndividualCourse(t *testing.T, pool *pgxpool.Pool, tutorID, studentID string) string {
	var id string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at)
		 VALUES ($1, $2, 'Математика', 40000, 8, NOW() - interval '1 month') RETURNING id`,
		studentID, tutorID).Scan(&id))
	return id
}

func studentExists(t *testing.T, pool *pgxpool.Pool, studentID string) bool {
	var ok bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM students WHERE id = $1)`, studentID).Scan(&ok))
	return ok
}

// Заведённый по ошибке: расписание вперёд и отменённый урок — не история,
// ученик уходит целиком.
func TestStudentDelete_WithoutHistoryRemovesStudent(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)
	courseID := addIndividualCourse(t, pool, tutorID, studentID)
	addLessonAt(t, pool, courseID, "NOW() + interval '2 days'", "scheduled")
	addLessonAt(t, pool, courseID, "NOW() - interval '2 days'", "cancelled")

	deleted, err := repository.NewStudentRepository(pool).Delete(context.Background(), studentID, tutorID)
	require.NoError(t, err)
	assert.True(t, deleted)
	assert.False(t, studentExists(t, pool, studentID))
}

func TestStudentDelete_HistoryBlocksDeletion(t *testing.T) {
	cases := []struct {
		name string
		seed func(t *testing.T, pool *pgxpool.Pool, tutorID, studentID string)
	}{
		{"платёж по архивному курсу", func(t *testing.T, pool *pgxpool.Pool, tutorID, studentID string) {
			courseID := addIndividualCourse(t, pool, tutorID, studentID)
			addPayment(t, pool, courseID, 40000, 8, "NOW() - interval '1 month'")
			_, err := pool.Exec(context.Background(), `UPDATE courses SET is_active = FALSE WHERE id = $1`, courseID)
			require.NoError(t, err)
		}},
		{"проведённый урок", func(t *testing.T, pool *pgxpool.Pool, tutorID, studentID string) {
			courseID := addIndividualCourse(t, pool, tutorID, studentID)
			addLessonAt(t, pool, courseID, "NOW() - interval '2 days'", "completed")
		}},
		{"пропущенный урок", func(t *testing.T, pool *pgxpool.Pool, tutorID, studentID string) {
			courseID := addIndividualCourse(t, pool, tutorID, studentID)
			addLessonAt(t, pool, courseID, "NOW() - interval '2 days'", "missed")
		}},
		{"отметка посещаемости в группе", func(t *testing.T, pool *pgxpool.Pool, tutorID, studentID string) {
			groupID := addGroupCourse(t, pool, tutorID, "Группа")
			lessonID := addLessonAt(t, pool, groupID, "NOW() - interval '2 days'", "completed")
			_, err := pool.Exec(context.Background(),
				`INSERT INTO lesson_attendances (lesson_id, student_id, status) VALUES ($1, $2, 'absent')`,
				lessonID, studentID)
			require.NoError(t, err)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := testPool(t)
			tutorID, studentID := seedTutorStudent(t, pool)
			tc.seed(t, pool, tutorID, studentID)

			deleted, err := repository.NewStudentRepository(pool).Delete(context.Background(), studentID, tutorID)
			require.NoError(t, err)
			assert.False(t, deleted)
			assert.True(t, studentExists(t, pool, studentID))
		})
	}
}
```

- [ ] **Step 2: Падающие юнит-тесты**

`service/student_test.go` — мок `Delete` заменить на:

```go
func (m *mockStudentRepo) Delete(ctx context.Context, id string, tutorID string) (bool, error) {
	args := m.Called(ctx, id, tutorID)
	return args.Bool(0), args.Error(1)
}
```

В `TestDeleteStudent_Success` строку `repo.On("Delete", mock.Anything, "student-1", "tutor-1").Return(nil)` заменить на `repo.On("Delete", mock.Anything, "student-1", "tutor-1").Return(true, nil)`.

После `TestStudentDelete_NotFound` добавить:

```go
// Ученик существует, а репозиторий ничего не удалил — мешает история. Это 409,
// а не 404: фронт предложит архив (спека, п. 5a.2).
func TestStudentDelete_WithHistoryIsConflict(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo))

	repo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{ID: "student-1"}, nil)
	repo.On("Delete", mock.Anything, "student-1", "tutor-1").Return(false, nil)

	err := svc.Delete(context.Background(), "student-1", "tutor-1")

	assert.ErrorIs(t, err, service.ErrConflict)
	repo.AssertExpectations(t)
}
```

`handlers/student_test.go`, после `TestStudentDelete_ServiceError`:

```go
func TestStudentDelete_HistoryConflict(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	svc.On("Delete", mock.Anything, testStudentID, testTutorID).
		Return(fmt.Errorf("student has history: %w", service.ErrConflict))

	w := makeRequest(t, r, http.MethodDelete, "/students/"+testStudentID, nil)

	assert.Equal(t, http.StatusConflict, w.Code)
	svc.AssertExpectations(t)
}
```

- [ ] **Step 3: Убедиться, что падает**

Run: `go test ./service/ -run TestStudentDelete -v`
Expected: ошибка компиляции — `*mockStudentRepo does not implement repository.StudentRepository (wrong type for method Delete)`.

- [ ] **Step 4: Реализация**

`repository/student.go` — в интерфейсе `Delete(ctx context.Context, id string, tutorID string) error` заменить на:

```go
	Delete(ctx context.Context, id string, tutorID string) (bool, error)
```

Метод `Delete` целиком:

```go
// Delete удаляет ученика, только если у него нет истории — платежей, проведённых
// уроков, отметок посещаемости (спека, п. 5a.1). Проверка внутри DELETE, а не
// отдельным SELECT: платёж, записанный между проверкой и удалением, иначе уехал
// бы в каскад students → courses → payments. false — ничего не удалено: ученика
// нет или у него есть история; различает сервис.
func (r *studentRepository) Delete(ctx context.Context, id string, tutorID string) (bool, error) {
	tag, err := r.conn.Exec(ctx,
		`DELETE FROM students s
		 WHERE s.id = $1 AND s.tutor_id = $2
		   AND NOT EXISTS (SELECT 1 FROM payments p JOIN courses c ON c.id = p.course_id
		                    WHERE c.student_id = s.id)
		   AND NOT EXISTS (SELECT 1 FROM lessons l JOIN courses c ON c.id = l.course_id
		                    WHERE c.student_id = s.id AND l.status IN ('completed', 'missed'))
		   AND NOT EXISTS (SELECT 1 FROM lesson_attendances la WHERE la.student_id = s.id)`,
		id, tutorID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
```

`service/student.go` — `Delete` целиком:

```go
// Delete удаляет ученика без истории. С историей — ErrConflict: удаление стёрло
// бы платежи и уроки, фронт предлагает архив (спека, п. 5a.2).
func (s *studentService) Delete(ctx context.Context, id string, tutorID string) error {
	if _, err := s.repo.GetByID(ctx, id, tutorID); err != nil {
		return fmt.Errorf("student: %w", ErrNotFound)
	}
	deleted, err := s.repo.Delete(ctx, id, tutorID)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("student has history: %w", ErrConflict)
	}
	return nil
}
```

Откат онбординга (`service/onboarding.go:128`) зовёт `studentService.Delete` — сигнатура сервиса не менялась, правка не нужна.

- [ ] **Step 5: Прогнать**

Run:
```bash
go build ./... && go test ./service/ ./handlers/ && \
TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" \
  go test -tags=integration -count=1 ./repository/ -run 'TestStudentDelete' -v
```
Expected: `TestStudentDelete_WithoutHistoryRemovesStudent` и все четыре подтеста `TestStudentDelete_HistoryBlocksDeletion` PASS; юнит-тесты PASS.

- [ ] **Step 6: Commit**

```bash
git add repository/student.go service/student.go service/student_test.go handlers/student_test.go repository/student_integration_test.go
git commit -m "feat(students): удаление только без истории, иначе 409" -m "Каскад students → courses → payments стирал платежи индивидуальных курсов.
Проверка истории (платежи, проведённые уроки, посещаемость) встроена в DELETE
одним запросом — без окна между проверкой и удалением." -m "Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

---

### Task 3: Архив ученика — backend

**Files:**
- Modify: `repository/student.go` (интерфейс; `GetAll` с `archived`; новый `SetActive`; `GetByInviteToken`, `GetCredentialsByLogin` — `AND active`)
- Modify: `repository/enrollment.go` (`AddBulk` — `AND s.active`)
- Modify: `service/student.go` (узкие интерфейсы, конструктор, `GetAll`, `Archive`, `Restore`)
- Modify: `service/enrollment.go` (`Add` отказывает архивному)
- Modify: `handlers/student.go` (`GetAll` читает `archived`; `Archive`, `Restore`)
- Modify: `router/router.go` (порядок проводки, роуты)
- Modify: `service/student_test.go`, `service/enrollment_test.go`, `handlers/mocks_test.go`, `handlers/student_test.go`
- Modify: `repository/student_integration_test.go`, `repository/enrollment_integration_test.go`

**Interfaces:**
- Consumes: из Task 1 — `EnrollmentRepository.LeaveAllByStudent`, `mockEnrollmentRepo.LeaveAllByStudent`, `addGroupCourse`; из Task 2 — `StudentRepository.Delete (bool, error)`.
- Produces:
  - `StudentRepository.GetAll(ctx context.Context, tutorID string, p models.Pagination, archived bool) ([]models.Student, int, error)`
  - `StudentRepository.SetActive(ctx context.Context, id string, tutorID string, active bool) error`
  - `StudentService.GetAll(ctx context.Context, tutorID string, p models.Pagination, archived bool) ([]models.Student, int, error)`
  - `StudentService.Archive(ctx context.Context, id string, tutorID string) error`
  - `StudentService.Restore(ctx context.Context, id string, tutorID string) error`
  - `service.NewStudentService(repo repository.StudentRepository, paymentRepo repository.PaymentRepository, courses studentCourses, enrollments enrollmentLeaver, sessions studentSessions) StudentService`
  - HTTP: `GET /students?archived=true`, `POST /students/:id/archive` → 204, `POST /students/:id/restore` → 204.

- [ ] **Step 1: Падающие интеграционные тесты**

`repository/student_integration_test.go` — добавить в импорты `"github.com/jackc/pgx/v5"` и `"tutorgo/models"`, в конец файла:

```go
// Архивный ученик пропадает из обычного списка и появляется в архиве; войти в
// кабинет и принять приглашение не может (спека, п. 5a.3).
func TestStudentArchived_HiddenFromListAndLogin(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)
	ctx := context.Background()
	login := "archived-" + studentID

	_, err := pool.Exec(ctx,
		`UPDATE students SET username = $2, password_hash = 'hash', invite_token = $2,
		        invite_expires_at = NOW() + interval '1 day'
		 WHERE id = $1`, studentID, login)
	require.NoError(t, err)

	students := repository.NewStudentRepository(pool)
	require.NoError(t, students.SetActive(ctx, studentID, tutorID, false))

	p := models.Pagination{Page: 1, Limit: 20}
	current, total, err := students.GetAll(ctx, tutorID, p, false)
	require.NoError(t, err)
	assert.Empty(t, current)
	assert.Equal(t, 0, total)

	archived, total, err := students.GetAll(ctx, tutorID, p, true)
	require.NoError(t, err)
	require.Len(t, archived, 1)
	assert.Equal(t, 1, total)
	assert.False(t, archived[0].Active)

	_, _, err = students.GetCredentialsByLogin(ctx, login)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	_, _, err = students.GetByInviteToken(ctx, login)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}
```

`repository/enrollment_integration_test.go`, в конец файла:

```go
// Пикеры архивных не показывают, но вкладка, открытая до архивации, может
// прислать его id — запись должна молча отсеяться (спека, п. 5a.3).
func TestEnrollmentAddBulk_SkipsArchivedStudent(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)
	courseID := addGroupCourse(t, pool, tutorID, "Группа")
	ctx := context.Background()

	_, err := pool.Exec(ctx, `UPDATE students SET active = FALSE WHERE id = $1`, studentID)
	require.NoError(t, err)

	added, err := repository.NewEnrollmentRepository(pool).AddBulk(ctx, courseID, []string{studentID}, tutorID)
	require.NoError(t, err)
	assert.Empty(t, added)
}
```

- [ ] **Step 2: Падающие юнит-тесты сервиса**

`service/student_test.go`:

1. Мок `GetAll` заменить на:

```go
func (m *mockStudentRepo) GetAll(ctx context.Context, tutorID string, p models.Pagination, archived bool) ([]models.Student, int, error) {
	args := m.Called(ctx, tutorID, p, archived)
	return args.Get(0).([]models.Student), args.Int(1), args.Error(2)
}
```

2. После мока `Delete` добавить:

```go
func (m *mockStudentRepo) SetActive(ctx context.Context, id string, tutorID string, active bool) error {
	return m.Called(ctx, id, tutorID, active).Error(0)
}

// mockStudentSessions — отзыв refresh-токенов кабинета при архивации.
type mockStudentSessions struct{ mock.Mock }

func (m *mockStudentSessions) DeleteByStudentID(ctx context.Context, studentID string) error {
	return m.Called(ctx, studentID).Error(0)
}
```

3. Все 15 вызовов `service.NewStudentService(repo, new(mockPaymentRepo))` заменить (Edit с `replace_all: true`) на `service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil)`.

4. В `TestGetAllStudents_Success` и `TestGetAllStudents_Error`: `repo.On("GetAll", mock.Anything, "tutor-1", p)` → `repo.On("GetAll", mock.Anything, "tutor-1", p, false)`; `svc.GetAll(context.Background(), "tutor-1", p)` → `svc.GetAll(context.Background(), "tutor-1", p, false)`.

5. В конец файла (если `require` не импортирован — добавить `"github.com/stretchr/testify/require"`):

```go
// Архивация складывает существующие действия в порядке, в котором сбой не
// оставляет полуархивного ученика вне списка: курсы, группы, сессии и только
// последним active = false (спека, п. 5a.3). Групповой курс не архивируется —
// он идёт для остальных, из него ученик уходит через left_at.
func TestStudentArchive_StepsInOrder(t *testing.T) {
	repo := new(mockStudentRepo)
	courses := new(mockCourseRepo) // тот же GetByStudent/Delete, что у courseService
	enrollments := new(mockEnrollmentRepo)
	sessions := new(mockStudentSessions)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), courses, enrollments, sessions)

	studentID := "student-1"
	individual := models.Course{ID: "course-ind", StudentID: &studentID}
	group := models.Course{ID: "course-group"}

	var order []string
	step := func(name string) func(mock.Arguments) {
		return func(mock.Arguments) { order = append(order, name) }
	}

	repo.On("GetByID", mock.Anything, studentID, "tutor-1").Return(models.Student{ID: studentID, Active: true}, nil)
	courses.On("GetByStudent", mock.Anything, studentID, "tutor-1").Return([]models.Course{individual, group}, nil)
	courses.On("Delete", mock.Anything, "course-ind", "tutor-1").Run(step("course")).Return(nil)
	enrollments.On("LeaveAllByStudent", mock.Anything, studentID).Run(step("groups")).Return(nil)
	sessions.On("DeleteByStudentID", mock.Anything, studentID).Run(step("sessions")).Return(nil)
	repo.On("SetActive", mock.Anything, studentID, "tutor-1", false).Run(step("inactive")).Return(nil)

	require.NoError(t, svc.Archive(context.Background(), studentID, "tutor-1"))

	assert.Equal(t, []string{"course", "groups", "sessions", "inactive"}, order)
	courses.AssertNotCalled(t, "Delete", mock.Anything, "course-group", "tutor-1")
}

// Сбой посередине не выставляет active = false: ученик остаётся в списке, и
// повторная архивация доделает начатое.
func TestStudentArchive_FailureKeepsStudentActive(t *testing.T) {
	repo := new(mockStudentRepo)
	courses := new(mockCourseRepo)
	enrollments := new(mockEnrollmentRepo)
	sessions := new(mockStudentSessions)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), courses, enrollments, sessions)

	studentID := "student-1"
	individual := models.Course{ID: "course-ind", StudentID: &studentID}

	repo.On("GetByID", mock.Anything, studentID, "tutor-1").Return(models.Student{ID: studentID, Active: true}, nil)
	courses.On("GetByStudent", mock.Anything, studentID, "tutor-1").Return([]models.Course{individual}, nil)
	courses.On("Delete", mock.Anything, "course-ind", "tutor-1").Return(errors.New("db down"))

	err := svc.Archive(context.Background(), studentID, "tutor-1")

	assert.Error(t, err)
	enrollments.AssertNotCalled(t, "LeaveAllByStudent", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "SetActive", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestStudentArchive_NotFound(t *testing.T) {
	repo := new(mockStudentRepo)
	courses := new(mockCourseRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), courses, new(mockEnrollmentRepo), new(mockStudentSessions))

	repo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{}, errors.New("not found"))

	err := svc.Archive(context.Background(), "student-1", "tutor-1")

	assert.ErrorIs(t, err, service.ErrNotFound)
	courses.AssertNotCalled(t, "GetByStudent", mock.Anything, mock.Anything, mock.Anything)
}

func TestStudentRestore_SetsActive(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil)

	repo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{ID: "student-1"}, nil)
	repo.On("SetActive", mock.Anything, "student-1", "tutor-1", true).Return(nil)

	require.NoError(t, svc.Restore(context.Background(), "student-1", "tutor-1"))
	repo.AssertExpectations(t)
}
```

`service/enrollment_test.go`, в конец файла:

```go
func TestEnrollmentAdd_ArchivedStudentRejected(t *testing.T) {
	repo := new(mockEnrollmentRepo)
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	svc := service.NewEnrollmentService(repo, courseRepo, studentRepo)

	group := models.Course{ID: "course-1", TutorID: "tutor-1", IsActive: true}
	courseRepo.On("GetByID", mock.Anything, "course-1", "tutor-1").Return(group, nil)
	studentRepo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{ID: "student-1", Active: false}, nil)

	_, err := svc.Add(context.Background(), "course-1", models.EnrollStudentRequest{StudentID: "student-1"}, "tutor-1")

	assert.ErrorIs(t, err, service.ErrBadRequest)
	repo.AssertNotCalled(t, "Add", mock.Anything, mock.Anything, mock.Anything)
}
```

- [ ] **Step 3: Падающие тесты хендлеров**

`handlers/mocks_test.go` — мок `GetAll` у `mockStudentService` заменить на:

```go
func (m *mockStudentService) GetAll(ctx context.Context, tutorID string, p models.Pagination, archived bool) ([]models.Student, int, error) {
	args := m.Called(ctx, tutorID, p, archived)
	return args.Get(0).([]models.Student), args.Int(1), args.Error(2)
}
```

после мока `Delete` добавить:

```go
func (m *mockStudentService) Archive(ctx context.Context, id string, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
}
func (m *mockStudentService) Restore(ctx context.Context, id string, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
}
```

`handlers/student_test.go`:

1. В `newStudentRouter` после `r.DELETE("/students/:id", h.Delete)`:

```go
	r.POST("/students/:id/archive", h.Archive)
	r.POST("/students/:id/restore", h.Restore)
```

2. Во всех `svc.On("GetAll", mock.Anything, testTutorID, p)` добавить последним аргументом `false` (тесты `TestStudentGetAll_Success`, `_ServiceError`, `_WithSearch`).

3. В конец файла:

```go
func TestStudentGetAll_Archived(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	p := models.Pagination{Page: 1, Limit: 20}
	svc.On("GetAll", mock.Anything, testTutorID, p, true).Return([]models.Student{testStudent}, 1, nil)

	w := makeRequest(t, r, http.MethodGet, "/students?page=1&limit=20&archived=true", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestStudentArchive_Success(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	svc.On("Archive", mock.Anything, testStudentID, testTutorID).Return(nil)

	w := makeRequest(t, r, http.MethodPost, "/students/"+testStudentID+"/archive", nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestStudentArchive_NotFound(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	svc.On("Archive", mock.Anything, testStudentID, testTutorID).Return(fmt.Errorf("student: %w", service.ErrNotFound))

	w := makeRequest(t, r, http.MethodPost, "/students/"+testStudentID+"/archive", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestStudentRestore_Success(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	svc.On("Restore", mock.Anything, testStudentID, testTutorID).Return(nil)

	w := makeRequest(t, r, http.MethodPost, "/students/"+testStudentID+"/restore", nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}
```

- [ ] **Step 4: Убедиться, что падает**

Run: `go test ./service/ ./handlers/ 2>&1 | head -20`
Expected: ошибки компиляции (`too many arguments in call to service.NewStudentService`, `h.Archive undefined` и т.п.).

- [ ] **Step 5: Репозитории**

`repository/student.go`:

Интерфейс: `GetAll(...)` заменить на
```go
	GetAll(ctx context.Context, tutorID string, p models.Pagination, archived bool) ([]models.Student, int, error)
```
и после `Delete` добавить
```go
	SetActive(ctx context.Context, id string, tutorID string, active bool) error
```

`GetAll` целиком (скан строк без изменений):

```go
// GetAll — ученики репетитора: активные или, с archived, только архивные
// (спека, п. 5a.3). Выбор ученика в календаре, состав группы и счётчик дашборда
// ходят сюда же — архивный пропадает из них всех одним условием.
func (r *studentRepository) GetAll(ctx context.Context, tutorID string, p models.Pagination, archived bool) ([]models.Student, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM students
		 WHERE tutor_id = $1
		   AND active = NOT $3::boolean
		   AND ($2 = '' OR first_name ILIKE '%' || $2 || '%'
		                 OR last_name  ILIKE '%' || $2 || '%'
		                 OR email      ILIKE '%' || $2 || '%')`,
		tutorID, p.Search, archived,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, tutor_id, first_name, last_name, phone, email, notes, active
		 FROM students
		 WHERE tutor_id = $1
		   AND active = NOT $5::boolean
		   AND ($2 = '' OR first_name ILIKE '%' || $2 || '%'
		                 OR last_name  ILIKE '%' || $2 || '%'
		                 OR email      ILIKE '%' || $2 || '%')
		 ORDER BY first_name, last_name
		 LIMIT $3 OFFSET $4`,
		tutorID, p.Search, p.Limit, p.Offset(), archived)
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

После `Delete`:

```go
// SetActive — архивация и восстановление ученика. Курсы и записи в группы не
// трогает: их закрывает studentService.Archive отдельными шагами.
func (r *studentRepository) SetActive(ctx context.Context, id string, tutorID string, active bool) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE students SET active = $3 WHERE id = $1 AND tutor_id = $2`, id, tutorID, active)
	return err
}
```

`GetByInviteToken` — SQL: `` `SELECT id, invite_expires_at FROM students WHERE invite_token=$1 AND active` ``.

`GetCredentialsByLogin` — SQL:
```go
		`SELECT id, password_hash FROM students
		 WHERE password_hash IS NOT NULL AND active AND (username=$1 OR phone=$1) LIMIT 1`, identifier,
```

`repository/enrollment.go`, `AddBulk` — строка WHERE:
```go
		 WHERE s.tutor_id = $2::uuid AND s.id = ANY($3::uuid[]) AND s.active
```

- [ ] **Step 6: Сервисы**

`service/student.go`:

После `import` добавить:

```go
// Архивация складывает уже существующие действия — зависимости узкими
// интерфейсами, как в onboarding.go.

// studentCourses — курсы ученика и их архивация. В проде это courseService, а
// не courseRepo: у репозитория тот же Delete, но без закрытия правил и удаления
// будущих уроков — архивный курс продолжал бы материализовать расписание.
type studentCourses interface {
	GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error)
	Delete(ctx context.Context, id string, tutorID string) error
}

type enrollmentLeaver interface {
	LeaveAllByStudent(ctx context.Context, studentID string) error
}

type studentSessions interface {
	DeleteByStudentID(ctx context.Context, studentID string) error
}
```

В интерфейсе `StudentService`: `GetAll` — сигнатура с `archived bool` (как в Interfaces), после `Delete` добавить:

```go
	Archive(ctx context.Context, id string, tutorID string) error
	Restore(ctx context.Context, id string, tutorID string) error
```

Структура, конструктор, `GetAll`:

```go
type studentService struct {
	repo        repository.StudentRepository
	paymentRepo repository.PaymentRepository
	courses     studentCourses
	enrollments enrollmentLeaver
	sessions    studentSessions
}

func NewStudentService(repo repository.StudentRepository, paymentRepo repository.PaymentRepository,
	courses studentCourses, enrollments enrollmentLeaver, sessions studentSessions) StudentService {
	return &studentService{repo: repo, paymentRepo: paymentRepo, courses: courses, enrollments: enrollments, sessions: sessions}
}
```

```go
func (s *studentService) GetAll(ctx context.Context, tutorID string, p models.Pagination, archived bool) ([]models.Student, int, error) {
	return s.repo.GetAll(ctx, tutorID, p, archived)
}
```

После `Delete`:

```go
// Archive убирает ученика с историей из работы, ничего не стирая (спека, п. 5a.3).
//
// Транзакции нет: каждый шаг идемпотентен, а active = false стоит последним и
// служит признаком «архивация завершена». Сбой посередине оставляет ученика в
// списке — повторный вызов доделает, уже архивные курсы в GetByStudent не попадут.
//
// ponytail: access-JWT ученика живёт 30 дней, а AuthStudent в базу не ходит —
// открытая сессия до истечения видит в кабинете свою историю; отзываются только
// refresh-токены. Проверка active в AuthStudent — если окно станет проблемой.
func (s *studentService) Archive(ctx context.Context, id string, tutorID string) error {
	if _, err := s.repo.GetByID(ctx, id, tutorID); err != nil {
		return fmt.Errorf("student: %w", ErrNotFound)
	}
	courses, err := s.courses.GetByStudent(ctx, id, tutorID)
	if err != nil {
		return err
	}
	for _, c := range courses {
		// Группа идёт для остальных — из неё ученик уходит через left_at ниже.
		if c.StudentID == nil {
			continue
		}
		if err := s.courses.Delete(ctx, c.ID, tutorID); err != nil {
			return err
		}
	}
	if err := s.enrollments.LeaveAllByStudent(ctx, id); err != nil {
		return err
	}
	if err := s.sessions.DeleteByStudentID(ctx, id); err != nil {
		return err
	}
	return s.repo.SetActive(ctx, id, tutorID, false)
}

// Restore возвращает ученика в список — и только: курсы восстанавливаются из
// архива курсов, в группу добавляют заново (спека, п. 5a.3).
func (s *studentService) Restore(ctx context.Context, id string, tutorID string) error {
	if _, err := s.repo.GetByID(ctx, id, tutorID); err != nil {
		return fmt.Errorf("student: %w", ErrNotFound)
	}
	return s.repo.SetActive(ctx, id, tutorID, true)
}
```

`service/enrollment.go`, в `Add` блок проверки ученика заменить на:

```go
	student, err := s.studentRepo.GetByID(ctx, req.StudentID, tutorID)
	if err != nil {
		return models.CourseEnrollment{}, fmt.Errorf("student: %w", ErrNotFound)
	}
	// Пикеры архивных не показывают — это на случай вкладки, открытой до архивации.
	if !student.Active {
		return models.CourseEnrollment{}, fmt.Errorf("student archived: %w", ErrBadRequest)
	}
```

- [ ] **Step 7: Хендлеры и роутер**

`handlers/student.go`, в `GetAll` вызов сервиса:

```go
	// archived=true — вкладка «Архив» на /students (спека, п. 5a.3).
	archived := c.Query("archived") == "true"
	students, total, err := h.service.GetAll(c.Request.Context(), tutorID, p, archived)
```

После `Delete`:

```go
func (h *StudentHandler) Archive(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	if err := h.service.Archive(c.Request.Context(), id, tutorID); err != nil {
		h.log.Error("Failed to archive student", slog.String("id", id), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Student archived", slog.String("id", id))
	c.Status(http.StatusNoContent)
}

func (h *StudentHandler) Restore(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	if err := h.service.Restore(c.Request.Context(), id, tutorID); err != nil {
		h.log.Error("Failed to restore student", slog.String("id", id), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Student restored", slog.String("id", id))
	c.Status(http.StatusNoContent)
}
```

`router/router.go`:

1. Удалить строку `studentService := service.NewStudentService(studentRepo, paymentRepo)`.
2. Сразу после строки `courseService := service.NewCourseService(courseRepo, studentRepo, lessonService)` вставить:

```go
	// Ниже courseService: архивация ученика архивирует его курсы именно сервисом —
	// у courseRepo тот же Delete, но без закрытия правил и будущих уроков.
	studentService := service.NewStudentService(studentRepo, paymentRepo, courseService, enrollmentRepo, studentRefreshRepo)
```

3. После `auth.DELETE("/students/:id", studentHandler.Delete)`:

```go
		auth.POST("/students/:id/archive", studentHandler.Archive)
		auth.POST("/students/:id/restore", studentHandler.Restore)
```

- [ ] **Step 8: Прогнать**

Run:
```bash
go build ./... && go vet -tags=integration ./repository/ && go test ./service/ ./handlers/ && \
TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" \
  go test -tags=integration -count=1 ./repository/ -run 'TestStudentArchived|TestEnrollmentAddBulk_SkipsArchivedStudent|TestStudentDelete|TestEnrollment' -v
```
Expected: всё PASS.

- [ ] **Step 9: Commit**

```bash
git add repository/student.go repository/enrollment.go service/student.go service/enrollment.go handlers/student.go router/router.go service/student_test.go service/enrollment_test.go handlers/mocks_test.go handlers/student_test.go repository/student_integration_test.go repository/enrollment_integration_test.go
git commit -m "feat(students): архив ученика — archive/restore, фильтр списков, кабинет" -m "Archive архивирует индивидуальные курсы существующим courseService.Delete,
закрывает записи в группы, отзывает refresh-токены и последним ставит
active = false. GET /students по умолчанию отдаёт активных, ?archived=true —
архив. Архивный не входит в кабинет и не записывается в группу." -m "Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

---

### Task 4: Фронт — удаление с предложением архива, вкладка «Архив», восстановление

**Files:**
- Modify: `frontend/src/types/api.ts` (`Student.active`)
- Modify: `frontend/src/lib/api/students.ts` (`StudentListParams.archived`, `archive`, `restore`)
- Modify: `frontend/src/lib/hooks/useStudents.ts` (`useArchiveStudent`, `useRestoreStudent`, `useRemoveStudent`)
- Modify: `frontend/src/components/students/StudentsList.tsx` (`onRestore`)
- Modify: `frontend/src/app/(dashboard)/students/page.tsx`
- Modify: `frontend/src/app/(dashboard)/students/[id]/page.tsx`

**Interfaces:**
- Consumes: HTTP из Task 2–3 — `DELETE /students/:id` (204 | 409), `POST /students/:id/archive` (204), `POST /students/:id/restore` (204), `GET /students?archived=true`; ответ `Student` несёт `active: boolean`. Ошибки axios-интерцептор нормализует в `ApiError { message, status }` (`frontend/src/lib/api/client.ts`).
- Produces: `useRemoveStudent(): (s: Student) => Promise<'deleted' | 'archived' | null>`, `useArchiveStudent()`, `useRestoreStudent()`.

Тест-раннера на фронте нет; проверка — `eslint` и `tsc` по затронутым файлам, затем ручной smoke (Task 5).

- [ ] **Step 1: Типы и API**

`frontend/src/types/api.ts`, интерфейс `Student` — после `tutor_id: string`:

```ts
  active: boolean
```

`frontend/src/lib/api/students.ts` — `StudentListParams`:

```ts
export interface StudentListParams {
  page:      number
  limit:     number
  search:    string
  archived?: boolean
}
```

В `studentsApi` после `delete`:

```ts
  archive: (id: string) => api.post(`/students/${id}/archive`).then(() => id),
  restore: (id: string) => api.post(`/students/${id}/restore`).then(() => id),
```

- [ ] **Step 2: Хуки**

`frontend/src/lib/hooks/useStudents.ts` — импорт типов заменить на:

```ts
import { ApiError, OnboardingStudentInput, Student } from '@/types/api'
```

Комментарий в `useDeleteStudent` заменить на:

```ts
    // Удаляется только ученик без истории (иначе 409, см. useRemoveStudent) —
    // вместе с курсами и будущими уроками; без этих сбросов они висели бы в
    // календаре до протухания кэша.
```

После `useDeleteStudent` добавить:

```ts
export function useArchiveStudent() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: studentsApi.archive,
    // Архивация уводит в архив курсы ученика и удаляет их будущие уроки.
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: studentKeys.all })
      qc.invalidateQueries({ queryKey: courseKeys.all })
      qc.invalidateQueries({ queryKey: ['calendar'] })
    },
  })
}

export function useRestoreStudent() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: studentsApi.restore,
    onSuccess:  () => qc.invalidateQueries({ queryKey: studentKeys.all }),
  })
}

/** «Удалить» из интерфейса. Ученик без истории удаляется; с платежами или
 *  проведёнными уроками сервер отвечает 409, и тогда предлагаем архив — удаление
 *  стёрло бы их из истории (спека, п. 5a.2). Одна функция на список и карточку,
 *  чтобы тексты диалогов не разъехались. */
export function useRemoveStudent() {
  const del     = useDeleteStudent()
  const archive = useArchiveStudent()

  return async (s: Student): Promise<'deleted' | 'archived' | null> => {
    const name = `${s.first_name}${s.last_name ? ` ${s.last_name}` : ''}`
    if (!confirm(`Удалить ${name}?`)) return null
    try {
      await del.mutateAsync(s.id)
      return 'deleted'
    } catch (e) {
      if ((e as ApiError).status !== 409) throw e
    }
    if (!confirm(`${name}: есть платежи или проведённые уроки — удаление стёрло бы их из истории. Перенести в архив?`)) {
      return null
    }
    await archive.mutateAsync(s.id)
    return 'archived'
  }
}
```

- [ ] **Step 3: Строка списка**

`frontend/src/components/students/StudentsList.tsx`:

Импорт иконок:
```ts
import { Pencil, Trash2, ChevronRight, UserPlus, Copy, RotateCcw } from 'lucide-react'
```

Props:
```ts
interface StudentsListProps {
  students: Student[]
  onEdit: (s: Student) => void
  onDelete: (s: Student) => void
  /** Задан — список показывает архив: вместо правки и удаления одна кнопка «Восстановить». */
  onRestore?: (s: Student) => void
}

export function StudentsList({ students, onEdit, onDelete, onRestore }: StudentsListProps) {
```

Блок кнопок (`<div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>` … `</div>`) целиком заменить на:

```tsx
            <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
              {onRestore ? (
                <Button size="icon" variant="ghost" className="h-8 w-8"
                  title="Восстановить"
                  onClick={() => onRestore(student)}>
                  <RotateCcw className="h-3.5 w-3.5" />
                </Button>
              ) : (
                <>
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
                </>
              )}
            </div>
```

- [ ] **Step 4: Страница списка**

`frontend/src/app/(dashboard)/students/page.tsx`:

Импорт хуков:
```ts
import { useStudentsPaged, useUpdateStudent, useRemoveStudent, useRestoreStudent } from '@/lib/hooks/useStudents'
```

Состояние и сброс страницы — после `const [page, setPage] = useState(1)`:

```ts
  const [archived, setArchived] = useState(false)
```

после `handleSearch`:

```ts
  function switchArchived(value: boolean) {
    setArchived(value)
    setPage(1)
  }
```

Запрос: `useStudentsPaged({ page, limit: LIMIT, search })` → `useStudentsPaged({ page, limit: LIMIT, search, archived })`.

`const deleteStudent = useDeleteStudent()` заменить на:

```ts
  const removeStudent  = useRemoveStudent()
  const restoreStudent = useRestoreStudent()
```

`handleDelete` целиком:

```ts
  async function handleDelete(s: Student) {
    const result = await removeStudent(s)
    if (result === 'deleted')  toast.success('Ученик удалён')
    if (result === 'archived') toast.success('Ученик перенесён в архив')
  }

  async function handleRestore(s: Student) {
    await restoreStudent.mutateAsync(s.id)
    toast.success('Ученик восстановлен')
  }
```

Метрика заголовка: `{total} учеников` → `{total} {archived ? 'в архиве' : 'учеников'}`.

Блок поиска (`<div className="mb-4">` … `</div>`) заменить на:

```tsx
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <Input
          placeholder="Поиск по имени или email..."
          value={search}
          onChange={(e) => handleSearch(e.target.value)}
          className="max-w-sm"
        />
        <Button size="sm" variant={archived ? 'outline' : 'secondary'} onClick={() => switchArchived(false)}>
          Активные
        </Button>
        <Button size="sm" variant={archived ? 'secondary' : 'outline'} onClick={() => switchArchived(true)}>
          Архив
        </Button>
      </div>
```

`EmptyState` целиком:

```tsx
        <EmptyState
          icon={Users}
          title={search ? 'Ничего не найдено' : archived ? 'Архив пуст' : 'Учеников пока нет'}
          description={search
            ? 'Попробуй другой запрос — поиск идёт по имени и контактам'
            : archived
              ? 'Сюда попадают ученики с платежами или проведёнными уроками, когда их удаляют: история оплат остаётся'
              : 'Ученик — карточка с контактами. К ней привязываются курсы, уроки и оплаты, а сам ученик может получить доступ в личный кабинет'}
          action={!search && !archived ? { label: 'Добавить ученика', onClick: () => setOnboardOpen(true) } : undefined}
        />
```

Список:

```tsx
          <StudentsList
            students={students}
            onEdit={openEdit}
            onDelete={handleDelete}
            onRestore={archived ? handleRestore : undefined}
          />
```

- [ ] **Step 5: Карточка ученика**

`frontend/src/app/(dashboard)/students/[id]/page.tsx`:

Импорты:
```ts
import { ArrowLeft, Pencil, RotateCcw, Trash2 } from 'lucide-react'

import { useStudent, useUpdateStudent, useRemoveStudent, useRestoreStudent } from '@/lib/hooks/useStudents'
```
```ts
import { PageHeader, HeaderMetric } from '@/components/common/PageHeader'
```

`const deleteStudent = useDeleteStudent()` заменить на:

```ts
  const removeStudent  = useRemoveStudent()
  const restoreStudent = useRestoreStudent()
```

`handleDelete` целиком:

```ts
  async function handleDelete() {
    if (!student) return
    const result = await removeStudent(student)
    if (result === 'deleted') {
      toast.success('Ученик удалён')
      router.push('/students')
    }
    // Архивный остаётся на карточке — она покажет плашку «В архиве».
    if (result === 'archived') toast.success('Ученик перенесён в архив')
  }

  async function handleRestore() {
    await restoreStudent.mutateAsync(id)
    toast.success('Ученик восстановлен')
  }
```

`PageHeader` целиком:

```tsx
      <PageHeader
        title={`${student.first_name}${student.last_name ? ` ${student.last_name}` : ''}`}
        meta={!student.active ? <HeaderMetric color="var(--muted-foreground)">В архиве</HeaderMetric> : undefined}
        actions={
          <div className="flex gap-2">
            <Button size="sm" variant="outline" onClick={() => setFormOpen(true)}>
              <Pencil className="h-4 w-4 mr-1.5" /> Редактировать
            </Button>
            {student.active ? (
              <Button size="sm" variant="destructive" onClick={handleDelete}>
                <Trash2 className="h-4 w-4 mr-1.5" /> Удалить
              </Button>
            ) : (
              <Button size="sm" variant="outline" onClick={handleRestore}>
                <RotateCcw className="h-4 w-4 mr-1.5" /> Восстановить
              </Button>
            )}
          </div>
        }
      />
```

- [ ] **Step 6: Проверить**

Run:
```bash
cd /home/dragonbrn/tutorgo/frontend && \
npx eslint src/types/api.ts src/lib/api/students.ts src/lib/hooks/useStudents.ts \
  src/components/students/StudentsList.tsx "src/app/(dashboard)/students/page.tsx" "src/app/(dashboard)/students/[id]/page.tsx" && \
npx tsc --noEmit 2>&1 | grep -E "students|useStudents|types/api|StudentsList" ; echo "tsc-filter-exit: $?"
```
Expected: eslint без ошибок; `grep` ничего не нашёл (`tsc-filter-exit: 1`). Если `tsc` показывает ошибки в других местах, где объект `Student` собирается руками без `active`, — добавить туда `active: true` и перепроверить.

- [ ] **Step 7: Commit**

```bash
cd /home/dragonbrn/tutorgo && git add frontend/src/types/api.ts frontend/src/lib/api/students.ts frontend/src/lib/hooks/useStudents.ts frontend/src/components/students/StudentsList.tsx "frontend/src/app/(dashboard)/students/page.tsx" "frontend/src/app/(dashboard)/students/[id]/page.tsx"
git commit -m "feat(students): фронт — удаление с предложением архива, вкладка «Архив»" -m "На 409 от DELETE /students/:id второй диалог предлагает перенести ученика в
архив. /students получает переключатель «Активные / Архив» и восстановление;
карточка архивного — плашку «В архиве» и кнопку «Восстановить»." -m "Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

---

### Task 5: Полная проверка и статус спеки

**Files:**
- Modify: `docs/specs/2026-09-06-price-units-and-student-centric-money.md` (строка статуса)

- [ ] **Step 1: Откат и повторная накатка миграции на тестовой БД**

Run:
```bash
goose -dir migrations postgres "postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" down && \
goose -dir migrations postgres "postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" status 2>&1 | tail -3 && \
goose -dir migrations postgres "postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" up
```
Expected: `down` откатывает `038_enrollments_left_at.sql`, `status` показывает её Pending, `up` накатывает снова. **Не `make migrate-down`** — он смотрит в прод.

- [ ] **Step 2: Все тесты**

Run:
```bash
go build ./... && go test ./... 2>&1 | grep -v "^ok\|no test files"; \
make test-integration 2>&1 | grep -E "^(--- FAIL|FAIL|ok)" 
```
Expected: `go test` без FAIL; в `make test-integration` единственный FAIL — известный `TestGetAllByTutor_CarriesSubjectAndStudentName`.

- [ ] **Step 3: Статус спеки**

В `docs/specs/2026-09-06-price-units-and-student-centric-money.md` строку
`**Статус:** фаза 0 в main; фаза 1 — PR из \`fix/package-pricing\`; фаза 1.5 — к реализации (\`feat/student-archive\`); фазы 2–4 к реализации`
заменить на
`**Статус:** фаза 0 в main; фаза 1 — PR из \`fix/package-pricing\`; фаза 1.5 — реализована в \`feat/student-archive\`, ждёт smoke; фазы 2–4 к реализации`.

- [ ] **Step 4: Commit**

```bash
git add docs/specs/2026-09-06-price-units-and-student-centric-money.md
git commit -m "docs(specs): фаза 1.5 реализована, ждёт smoke" -m "Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

- [ ] **Step 5: Ручной smoke (за человеком)** — по критериям приёмки п. 5a спеки, на локальном стеке:
  - ученик из модалки с расписанием, без оплат → «Удалить» → исчез вместе с курсом;
  - ученик с платежом → «Удалить» → второй диалог → «Архив»: нет в списке, есть во вкладке «Архив», платёж на `/payments`;
  - архивного нет в выборе ученика в календаре и в выборе участников группы;
  - «Восстановить» из вкладки и с карточки;
  - убрать ученика из группы на странице курса → пропал из состава; добавить снова → вернулся без дубля.
