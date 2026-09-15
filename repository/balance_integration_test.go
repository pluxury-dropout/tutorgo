//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"

	"tutorgo/models"
	"tutorgo/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Баланс и прогноз считаются по паре «курс + ученик» в периодах участия
// (спека 2026-09-06-price-units…, п. 3.6, 3.9, 6.4, 6.6). Запуск:
// make test-integration.

// addPause — заморозка ученика; границы — SQL-выражения дат.
func addPause(t *testing.T, pool *pgxpool.Pool, studentID, startsExpr, endsExpr string) {
	_, err := pool.Exec(context.Background(),
		fmt.Sprintf(`INSERT INTO student_pauses (student_id, starts_on, ends_on)
		             VALUES ($1, %s, %s)`, startsExpr, endsExpr),
		studentID)
	require.NoError(t, err)
}

func balanceOf(t *testing.T, pool *pgxpool.Pool, courseID, studentID string) models.CourseBalance {
	b, err := repository.NewPaymentRepository(pool).GetBalance(context.Background(), courseID, studentID)
	require.NoError(t, err)
	return b
}

// Группа из трёх: каждый оплатил 4, проведено 2, один отмечен absent — у всех
// осталось по 2. Посещаемость денег не касается (спека, п. 3.6).
func TestBalance_GroupAttendanceDoesNotTouchMoney(t *testing.T) {
	pool := testPool(t)
	tutorID, a := seedTutorStudent(t, pool)
	b := addStudent(t, pool, tutorID, "Бекзат")
	c := addStudent(t, pool, tutorID, "Вика")
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	for _, s := range []string{a, b, c} {
		enroll(t, pool, groupID, s, "NOW() - interval '1 month'")
		payFor(t, pool, groupID, s, 4)
	}
	first := addLessonAt(t, pool, groupID, "NOW() - interval '2 weeks'", "completed")
	addLessonAt(t, pool, groupID, "NOW() - interval '1 week'", "completed")
	_, err := pool.Exec(context.Background(),
		`INSERT INTO lesson_attendances (lesson_id, student_id, status) VALUES ($1, $2, 'absent')`, first, c)
	require.NoError(t, err)

	for _, s := range []string{a, b, c} {
		assert.Equal(t, models.CourseBalance{LessonsPaid: 4, LessonsCompleted: 2, LessonsRemaining: 2},
			balanceOf(t, pool, groupID, s))
	}
}

// Записан после третьего занятия — за них не должен (спека, п. 3.7).
func TestBalance_LateEnrolleeOwesNothingForEarlierLessons(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	for i := 3; i >= 1; i-- {
		addLessonAt(t, pool, groupID, fmt.Sprintf("NOW() - interval '%d weeks'", i), "completed")
	}
	enroll(t, pool, groupID, s, "NOW() - interval '1 day'")

	assert.Equal(t, models.CourseBalance{}, balanceOf(t, pool, groupID, s))
}

// Отменённый не сгорает, пропущенный — сгорает (спека, п. 3.6).
func TestBalance_CancelledNotBurnedMissedBurned(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	courseID := addIndividualCourse(t, pool, tutorID, s)
	addLessonAt(t, pool, courseID, "NOW() - interval '3 days'", "cancelled")
	addLessonAt(t, pool, courseID, "NOW() - interval '2 days'", "missed")
	addLessonAt(t, pool, courseID, "NOW() - interval '1 day'", "completed")
	payFor(t, pool, courseID, s, 8)

	assert.Equal(t, models.CourseBalance{LessonsPaid: 8, LessonsCompleted: 2, LessonsRemaining: 6},
		balanceOf(t, pool, courseID, s))
}

// Платёж за математику не меняет баланс физики.
func TestBalance_TwoSubjectsAreSeparate(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	math := addIndividualCourse(t, pool, tutorID, s)
	var physics string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at)
		 VALUES ($1, $2, 'Физика', 6000, 1, NOW() - interval '1 month') RETURNING id`,
		s, tutorID).Scan(&physics))
	addLessonAt(t, pool, physics, "NOW() - interval '1 day'", "completed")
	payFor(t, pool, math, s, 8)

	assert.Equal(t, models.CourseBalance{LessonsPaid: 0, LessonsCompleted: 1, LessonsRemaining: -1},
		balanceOf(t, pool, physics, s))
	assert.Equal(t, 8, balanceOf(t, pool, math, s).LessonsPaid)
}

// Ушедший: занятия после ухода не сгорают; легаси-платёж группы без адресата
// в его баланс не входит (спека, п. 3.4, 5a.4).
func TestBalance_LeftStudentStopsBurningAndLegacyPaymentIgnored(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	enroll(t, pool, groupID, s, "NOW() - interval '1 month'")
	addLessonAt(t, pool, groupID, "NOW() - interval '2 weeks'", "completed")
	_, err := pool.Exec(context.Background(),
		`UPDATE course_enrollments SET left_at = NOW() - interval '10 days'
		 WHERE course_id = $1 AND student_id = $2`, groupID, s)
	require.NoError(t, err)
	addLessonAt(t, pool, groupID, "NOW() - interval '1 week'", "completed")
	addPayment(t, pool, groupID, 5000, 4, "NOW() - interval '3 weeks'")

	assert.Equal(t, models.CourseBalance{LessonsPaid: 0, LessonsCompleted: 1, LessonsRemaining: -1},
		balanceOf(t, pool, groupID, s))
}

// Заморозка задним числом: урок в паузе не сгорает ни на индивидуальном курсе,
// ни в группе, где занятие шло для остальных (спека, п. 6.9).
func TestBalance_PauseExcludesLessonsOnIndividualAndGroup(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	courseID := addIndividualCourse(t, pool, tutorID, s)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	enroll(t, pool, groupID, s, "NOW() - interval '1 month'")
	addLessonAt(t, pool, courseID, "NOW() - interval '20 days'", "completed")
	addLessonAt(t, pool, courseID, "NOW() - interval '3 days'", "completed")
	addLessonAt(t, pool, groupID, "NOW() - interval '3 days'", "completed")
	addPause(t, pool, s, "CURRENT_DATE - 7", "CURRENT_DATE - 1")

	assert.Equal(t, 1, balanceOf(t, pool, courseID, s).LessonsCompleted)
	assert.Equal(t, 0, balanceOf(t, pool, groupID, s).LessonsCompleted)
}

// Группа из пяти по 5000 за урок: прогноз — пять пакетов, а не один (спека, п. 6.6).
func TestMonthlyExpected_GroupCountsEveryMember(t *testing.T) {
	pool := testPool(t)
	tutorID, first := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	members := []string{first}
	for i := 0; i < 4; i++ {
		members = append(members, addStudent(t, pool, tutorID, fmt.Sprintf("Ученик %d", i)))
	}
	for _, m := range members {
		enroll(t, pool, groupID, m, "date_trunc('month', NOW()) - interval '1 month'")
	}
	addLessonAt(t, pool, groupID, "date_trunc('month', NOW()) + interval '10 days'", "scheduled")

	total, err := repository.NewPaymentRepository(pool).GetMonthlyExpected(context.Background(), tutorID)
	require.NoError(t, err)
	assert.Equal(t, 25000.0, total)
}
