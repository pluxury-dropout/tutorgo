//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/repository"
	"tutorgo/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Заморозка отменяет уроки индивидуальных курсов в интервале и сдвигает хвост
// их правил; расписание групп не трогает (спека 2026-09-06-price-units…,
// п. 6.9). Запуск: make test-integration.

// seedIndividualSeries — seedSeries (6 еженедельных уроков через неделю от
// сегодня), чей курс отдан новому ученику: заморозка меняет расписание только
// индивидуальных курсов.
func seedIndividualSeries(t *testing.T, pool *pgxpool.Pool) (studentID, courseID, ruleID string, first time.Time) {
	base := time.Now().UTC().AddDate(0, 0, 7)
	first = time.Date(base.Year(), base.Month(), base.Day(), 12, 0, 0, 0, time.UTC)
	tutorID, courseID, ruleID := seedSeries(t, pool, first, 6)
	studentID = addStudent(t, pool, tutorID, "Уехал")
	_, err := pool.Exec(context.Background(),
		`UPDATE courses SET student_id = $1 WHERE id = $2`, studentID, courseID)
	require.NoError(t, err)
	return studentID, courseID, ruleID, first
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func countLessons(t *testing.T, pool *pgxpool.Pool, courseID, status string) int {
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM lessons WHERE course_id = $1 AND status = $2`, courseID, status).Scan(&n))
	return n
}

// Пауза на вторую и третью недели: два урока отменены, конец правила уехал на
// 14 дней, после материализации запланированных снова шесть.
func TestPauseCreate_CancelsLessonsAndShiftsEndsOn(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	studentID, courseID, ruleID, first := seedIndividualSeries(t, pool)

	var oldEnds time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT ends_on FROM recurrence_rules WHERE id = $1`, ruleID).Scan(&oldEnds))

	starts, ends := dateOnly(first.AddDate(0, 0, 7)), dateOnly(first.AddDate(0, 0, 20))
	pause, shifted, err := repository.NewPauseRepository(pool).Create(ctx, studentID,
		models.CreatePauseRequest{StartsOn: starts, EndsOn: ends})
	require.NoError(t, err)
	assert.Equal(t, studentID, pause.StudentID)
	assert.Equal(t, []string{ruleID}, shifted)
	assert.Equal(t, 2, countLessons(t, pool, courseID, "cancelled"))

	var newEnds, materializedUntil time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT ends_on, materialized_until FROM recurrence_rules WHERE id = $1`, ruleID).
		Scan(&newEnds, &materializedUntil))
	assert.Equal(t, oldEnds.AddDate(0, 0, 14).Format(time.DateOnly), newEnds.Format(time.DateOnly))
	assert.Equal(t, starts.Format(time.DateOnly), materializedUntil.Format(time.DateOnly))

	_, err = service.NewRecurrenceService(repository.NewRecurrenceRepository(pool)).
		Materialize(ctx, ruleID, first.AddDate(0, 0, 7*10))
	require.NoError(t, err)
	assert.Equal(t, 6, countLessons(t, pool, courseID, "scheduled"))
	assert.Equal(t, 2, countLessons(t, pool, courseID, "cancelled"))
}

// Правило со счётчиком: max_count растёт ровно на число отменённых вхождений.
func TestPauseCreate_ShiftsMaxCount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	studentID, courseID, ruleID, first := seedIndividualSeries(t, pool)
	_, err := pool.Exec(ctx, `UPDATE recurrence_rules SET ends_on = NULL, max_count = 6 WHERE id = $1`, ruleID)
	require.NoError(t, err)

	_, shifted, err := repository.NewPauseRepository(pool).Create(ctx, studentID, models.CreatePauseRequest{
		StartsOn: dateOnly(first.AddDate(0, 0, 7)), EndsOn: dateOnly(first.AddDate(0, 0, 20)),
	})
	require.NoError(t, err)
	assert.Equal(t, []string{ruleID}, shifted)

	var maxCount int
	var endsOn *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT max_count, ends_on FROM recurrence_rules WHERE id = $1`, ruleID).Scan(&maxCount, &endsOn))
	assert.Equal(t, 8, maxCount)
	assert.Nil(t, endsOn)

	_, err = service.NewRecurrenceService(repository.NewRecurrenceRepository(pool)).
		Materialize(ctx, ruleID, first.AddDate(0, 0, 7*10))
	require.NoError(t, err)
	assert.Equal(t, 6, countLessons(t, pool, courseID, "scheduled"))
}

// Заморозка участника группы расписание группы не меняет: занятия идут для
// остальных.
func TestPauseCreate_GroupScheduleUntouched(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	base := time.Now().UTC().AddDate(0, 0, 7)
	first := time.Date(base.Year(), base.Month(), base.Day(), 12, 0, 0, 0, time.UTC)
	tutorID, groupID, ruleID := seedSeries(t, pool, first, 6) // курс seedSeries — групповой
	s := addStudent(t, pool, tutorID, "Участник")
	enroll(t, pool, groupID, s, "NOW() - interval '1 month'")

	var endsBefore time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT ends_on FROM recurrence_rules WHERE id = $1`, ruleID).Scan(&endsBefore))

	_, shifted, err := repository.NewPauseRepository(pool).Create(ctx, s, models.CreatePauseRequest{
		StartsOn: dateOnly(first), EndsOn: dateOnly(first.AddDate(0, 0, 20)),
	})
	require.NoError(t, err)
	assert.Empty(t, shifted)
	assert.Equal(t, 0, countLessons(t, pool, groupID, "cancelled"))

	var endsAfter time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT ends_on FROM recurrence_rules WHERE id = $1`, ruleID).Scan(&endsAfter))
	assert.Equal(t, endsBefore, endsAfter)
}

// Заморозка задним числом статусы не трогает: прошедшие уроки уже проведены,
// их выводит из счёта только предикат участия.
func TestPauseCreate_RetroactiveKeepsStatuses(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tutorID, s := seedTutorStudent(t, pool)
	courseID := addIndividualCourse(t, pool, tutorID, s)
	addLessonAt(t, pool, courseID, "NOW() - interval '3 days'", "completed")

	today := dateOnly(time.Now().UTC())
	_, shifted, err := repository.NewPauseRepository(pool).Create(ctx, s, models.CreatePauseRequest{
		StartsOn: today.AddDate(0, 0, -7), EndsOn: today.AddDate(0, 0, -1),
	})
	require.NoError(t, err)
	assert.Empty(t, shifted)
	assert.Equal(t, 1, countLessons(t, pool, courseID, "completed"))
}

// Правило без ends_on и без max_count — бессрочное: пауза отменяет его уроки в
// интервале, но само правило не трогает (спека, п. 6.9, «оба NULL — ничего»).
func TestPauseCreate_OpenEndedRuleUntouched(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	studentID, courseID, ruleID, first := seedIndividualSeries(t, pool)
	_, err := pool.Exec(ctx, `UPDATE recurrence_rules SET ends_on = NULL, max_count = NULL WHERE id = $1`, ruleID)
	require.NoError(t, err)

	var before time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT materialized_until FROM recurrence_rules WHERE id = $1`, ruleID).Scan(&before))

	_, shifted, err := repository.NewPauseRepository(pool).Create(ctx, studentID, models.CreatePauseRequest{
		StartsOn: dateOnly(first.AddDate(0, 0, 7)), EndsOn: dateOnly(first.AddDate(0, 0, 20)),
	})
	require.NoError(t, err)
	assert.Empty(t, shifted)
	assert.Equal(t, 2, countLessons(t, pool, courseID, "cancelled"))

	var after time.Time
	var endsOn *time.Time
	var maxCount *int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT materialized_until, ends_on, max_count FROM recurrence_rules WHERE id = $1`, ruleID).
		Scan(&after, &endsOn, &maxCount))
	assert.Equal(t, before, after)
	assert.Nil(t, endsOn)
	assert.Nil(t, maxCount)
}

func TestPauseListAndDelete_ScopedByStudent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tutorID, s := seedTutorStudent(t, pool)
	other := addStudent(t, pool, tutorID, "Другой")
	repo := repository.NewPauseRepository(pool)

	today := dateOnly(time.Now().UTC())
	reason := "сессия"
	pause, _, err := repo.Create(ctx, s, models.CreatePauseRequest{StartsOn: today, EndsOn: today.AddDate(0, 0, 3), Reason: &reason})
	require.NoError(t, err)

	list, err := repo.ListByStudent(ctx, s)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "сессия", *list[0].Reason)

	deleted, err := repo.Delete(ctx, pause.ID, other)
	require.NoError(t, err)
	assert.False(t, deleted)

	deleted, err = repo.Delete(ctx, pause.ID, s)
	require.NoError(t, err)
	assert.True(t, deleted)

	list, err = repo.ListByStudent(ctx, s)
	require.NoError(t, err)
	assert.Empty(t, list)
}
