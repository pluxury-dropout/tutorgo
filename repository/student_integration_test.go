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
