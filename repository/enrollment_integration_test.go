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
