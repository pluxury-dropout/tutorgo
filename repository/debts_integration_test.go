//go:build integration

package repository_test

import (
	"context"
	"testing"

	"tutorgo/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Долги по всем курсам репетитора, включая архивные, и по всем ученикам,
// включая ушедших (спека 2026-09-06-price-units…, п. 6.5). Запуск:
// make test-integration.

// Ученик с двумя долгами — две строки с ценой урока своего курса; архивный курс
// долг не прощает; урок группы после ухода не сгорает; погасивший в выборку не
// попадает.
func TestGetDebts_ArchivedCourseLeftGroupAndPaidOff(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tutorID, s := seedTutorStudent(t, pool)

	math := addIndividualCourse(t, pool, tutorID, s) // 40000 за 8 → 5000 за урок
	addLessonAt(t, pool, math, "NOW() - interval '2 days'", "completed")
	addLessonAt(t, pool, math, "NOW() - interval '1 day'", "missed")
	_, err := pool.Exec(ctx, `UPDATE courses SET is_active = FALSE WHERE id = $1`, math)
	require.NoError(t, err)

	groupID := addGroupCourse(t, pool, tutorID, "Группа") // 5000 за урок
	enroll(t, pool, groupID, s, "NOW() - interval '1 month'")
	addLessonAt(t, pool, groupID, "NOW() - interval '3 days'", "completed")
	_, err = pool.Exec(ctx,
		`UPDATE course_enrollments SET left_at = NOW() - interval '2 days'
		 WHERE course_id = $1 AND student_id = $2`, groupID, s)
	require.NoError(t, err)
	addLessonAt(t, pool, groupID, "NOW() - interval '1 day'", "completed")

	paidOff := addStudent(t, pool, tutorID, "Погасил")
	enroll(t, pool, groupID, paidOff, "NOW() - interval '1 month'")
	payFor(t, pool, groupID, paidOff, 4)

	debts, err := repository.NewPaymentRepository(pool).GetDebts(ctx, tutorID)
	require.NoError(t, err)
	require.Len(t, debts, 2)

	assert.Equal(t, s, debts[0].StudentID)
	assert.Equal(t, "Иван Петров", debts[0].StudentName)
	assert.Equal(t, "Группа", debts[0].Subject)
	assert.Equal(t, 1, debts[0].LessonsOwed)
	assert.Equal(t, 5000.0, debts[0].LessonPrice)

	assert.Equal(t, "Математика", debts[1].Subject)
	assert.Equal(t, 2, debts[1].LessonsOwed)
	assert.Equal(t, 5000.0, debts[1].LessonPrice)
}
