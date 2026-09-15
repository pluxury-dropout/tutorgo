//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Позиция в цикле считается по ученику в тех же периодах участия, что и
// сгорание в балансе (спека 2026-09-06-price-units…, п. 3.8, 6.7). Запуск:
// make test-integration.

// Индивидуальный курс: отменённый и замороженный уроки не ранжируются, ранги
// идут подряд по оставшимся.
func TestGetRanksForStudent_SkipsCancelledAndPaused(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	courseID := addIndividualCourse(t, pool, tutorID, s)
	first := addLessonAt(t, pool, courseID, "NOW() - interval '20 days'", "completed")
	cancelled := addLessonAt(t, pool, courseID, "NOW() - interval '10 days'", "cancelled")
	paused := addLessonAt(t, pool, courseID, "NOW() - interval '3 days'", "completed")
	next := addLessonAt(t, pool, courseID, "NOW() + interval '2 days'", "scheduled")
	addPause(t, pool, s, "CURRENT_DATE - 5", "CURRENT_DATE - 1")

	ranks, err := repository.NewLessonRepository(pool).GetRanksForStudent(context.Background(), courseID, s)
	require.NoError(t, err)

	assert.Equal(t, map[string]int{first: 1, next: 2}, ranks)
	assert.NotContains(t, ranks, cancelled)
	assert.NotContains(t, ranks, paused)
}

// Группа: ранги ученика начинаются с его записи.
func TestGetRanksForStudent_GroupStartsAtEnrollment(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	addLessonAt(t, pool, groupID, "NOW() - interval '14 days'", "completed")
	mine := addLessonAt(t, pool, groupID, "NOW() - interval '7 days'", "completed")
	enroll(t, pool, groupID, s, "NOW() - interval '10 days'")

	ranks, err := repository.NewLessonRepository(pool).GetRanksForStudent(context.Background(), groupID, s)
	require.NoError(t, err)
	assert.Equal(t, map[string]int{mine: 1}, ranks)
}

// Календарь репетитора: у группового урока ранга нет, у индивидуального — есть
// (спека, п. 3.8).
func TestGetCalendar_OnlyIndividualLessonsAreRanked(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	courseID := addIndividualCourse(t, pool, tutorID, s)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	individual := addLessonAt(t, pool, courseID, "NOW() + interval '1 day'", "scheduled")
	group := addLessonAt(t, pool, groupID, "NOW() + interval '1 day'", "scheduled")

	from := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	to := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)
	lessons, err := repository.NewLessonRepository(pool).GetCalendar(context.Background(), tutorID, from, to)
	require.NoError(t, err)

	byID := map[string]models.CalendarLesson{}
	for _, l := range lessons {
		byID[l.ID] = l
	}
	require.Contains(t, byID, individual)
	require.Contains(t, byID, group)
	require.NotNil(t, byID[individual].Rank)
	assert.Equal(t, 1, *byID[individual].Rank)
	assert.Nil(t, byID[group].Rank)
}

// Кабинет ученика: урок группы до записи виден, но в цикл не входит.
func TestListLessons_RanksWithinParticipation(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	before := addLessonAt(t, pool, groupID, "NOW() - interval '14 days'", "completed")
	mine := addLessonAt(t, pool, groupID, "NOW() - interval '7 days'", "completed")
	enroll(t, pool, groupID, s, "NOW() - interval '10 days'")

	lessons, err := repository.NewStudentRepository(pool).ListLessons(context.Background(), s, true)
	require.NoError(t, err)

	byID := map[string]models.CalendarLesson{}
	for _, l := range lessons {
		byID[l.ID] = l
	}
	require.Contains(t, byID, before)
	require.Contains(t, byID, mine)
	assert.Nil(t, byID[before].Rank)
	require.NotNil(t, byID[mine].Rank)
	assert.Equal(t, 1, *byID[mine].Rank)
}

// Платежи ученика по его курсам — без чужих платежей той же группы.
func TestGetByStudentBatch_OnlyOwnPayments(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	other := addStudent(t, pool, tutorID, "Другой")
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	enroll(t, pool, groupID, s, "NOW() - interval '1 month'")
	enroll(t, pool, groupID, other, "NOW() - interval '1 month'")
	payFor(t, pool, groupID, s, 4)
	payFor(t, pool, groupID, other, 8)

	byCourse, err := repository.NewPaymentRepository(pool).GetByStudentBatch(context.Background(), s)
	require.NoError(t, err)
	require.Len(t, byCourse[groupID], 1)
	assert.Equal(t, 4, byCourse[groupID][0].LessonsCount)
}
