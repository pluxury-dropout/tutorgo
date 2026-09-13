//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"tutorgo/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GetOrCreateIndividual создаёт неявный курс из календаря с дефолтным прайсом
// тьютора. Цена — пакет «N уроков за X ₸», и он не обязан делиться на уроки
// нацело, поэтому дефолт обязан браться ПАРОЙ из одного пакета. Запуск:
// make test-integration.

// seedTutorStudent создаёт репетитора и ученика, регистрирует cleanup.
// Каскад от tutors сносит students → courses.
func seedTutorStudent(t *testing.T, pool *pgxpool.Pool) (tutorID, studentID string) {
	ctx := context.Background()
	email := fmt.Sprintf("course-defaults-%d@example.com", time.Now().UnixNano())

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO tutors (email, password_hash, first_name, last_name)
		 VALUES ($1, 'x', 'Test', 'Tutor') RETURNING id`, email).Scan(&tutorID))
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM tutors WHERE id=$1`, tutorID)
		assert.NoError(t, err)
	})

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO students (tutor_id, first_name, last_name)
		 VALUES ($1, 'Иван', 'Петров') RETURNING id`, tutorID).Scan(&studentID))
	return tutorID, studentID
}

// addPriced — активный курс с заданным пакетом, начатый daysAgo дней назад.
// Предмет уникален на курс: (student_id, subject) — уникальный ключ.
func addPriced(t *testing.T, pool *pgxpool.Pool, tutorID, studentID string, price float64, lessons, daysAgo int) {
	_, err := pool.Exec(context.Background(),
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at)
		 VALUES ($1, $2, $3, $4, $5, NOW() - $6 * interval '1 day')`,
		studentID, tutorID, fmt.Sprintf("Предмет %.0f/%d/%d", price, lessons, daysAgo), price, lessons, daysAgo)
	require.NoError(t, err)
}

func createFromCalendar(t *testing.T, pool *pgxpool.Pool, tutorID, studentID string) (float64, int) {
	course, err := repository.NewCourseRepository(pool).
		GetOrCreateIndividual(context.Background(), tutorID, studentID, "Из календаря", time.Now())
	require.NoError(t, err)
	return course.PricePerCycle, course.LessonsPerCycle
}

// По мотивам тьютора с прода, у которого все пакеты уникальны: прежний
// tie-break «минимальная цена» выбирал тестовый курс за 1 ₸ на 12 уроков.
// Ожидается текущий прайс — пакет самого позднего курса.
func TestGetOrCreateIndividual_TieTakesLatestPackage(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)

	addPriced(t, pool, tutorID, studentID, 4500, 1, 130)
	addPriced(t, pool, tutorID, studentID, 1, 12, 90)
	addPriced(t, pool, tutorID, studentID, 85000, 12, 60)
	addPriced(t, pool, tutorID, studentID, 90000, 12, 40)
	addPriced(t, pool, tutorID, studentID, 80000, 12, 7)

	price, lessons := createFromCalendar(t, pool, tutorID, studentID)
	assert.Equal(t, 80000.0, price)
	assert.Equal(t, 12, lessons)
}

// Независимые «самая частая сумма» (60 000, дважды) и «самое частое число
// уроков» (8, трижды) склеились бы в пакет 60 000 за 8, которого у тьютора нет.
func TestGetOrCreateIndividual_PriceAndLessonsComeFromOnePackage(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)

	addPriced(t, pool, tutorID, studentID, 60000, 12, 30)
	addPriced(t, pool, tutorID, studentID, 60000, 12, 20)
	addPriced(t, pool, tutorID, studentID, 40000, 8, 10)
	addPriced(t, pool, tutorID, studentID, 48000, 8, 5)
	addPriced(t, pool, tutorID, studentID, 32000, 8, 1)

	price, lessons := createFromCalendar(t, pool, tutorID, studentID)
	assert.Equal(t, 60000.0, price)
	assert.Equal(t, 12, lessons)
}

func TestGetOrCreateIndividual_FirstCourseGetsZeroForOneLesson(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)

	price, lessons := createFromCalendar(t, pool, tutorID, studentID)
	assert.Equal(t, 0.0, price)
	assert.Equal(t, 1, lessons)
}
