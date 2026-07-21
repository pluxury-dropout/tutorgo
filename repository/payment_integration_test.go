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

// GetMonthlyExpected прогнозирует поступления: платёж ждут на первом уроке
// каждого неоплаченного цикла. Запуск: make test-integration.
//
// Даты уроков задаются выражениями от date_trunc('month', NOW()), потому что
// сам запрос сравнивает с текущим месяцем — фиксированные даты сломались бы
// при следующем прогоне. Смещения держим в пределах +27 дней, чтобы тест не
// выпадал из февраля.

// seedCourse создаёт репетитора, ученика и курс, регистрирует cleanup.
// Каскад от tutors сносит students → courses → lessons → payments.
func seedCourse(t *testing.T, pool *pgxpool.Pool, pricePerCycle float64, lessonsPerCycle int) (tutorID, courseID string) {
	ctx := context.Background()
	email := fmt.Sprintf("monthly-expected-%d@example.com", time.Now().UnixNano())

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO tutors (email, password_hash, first_name, last_name)
		 VALUES ($1, 'x', 'Test', 'Tutor') RETURNING id`, email).Scan(&tutorID))
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM tutors WHERE id=$1`, tutorID)
		assert.NoError(t, err)
	})

	var studentID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO students (tutor_id, first_name, last_name)
		 VALUES ($1, 'Иван', 'Петров') RETURNING id`, tutorID).Scan(&studentID))

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at)
		 VALUES ($1, $2, 'Математика', $3, $4, NOW() - interval '2 months') RETURNING id`,
		studentID, tutorID, pricePerCycle, lessonsPerCycle).Scan(&courseID))

	return tutorID, courseID
}

// addLessons вставляет n уроков подряд с шагом в час от базового выражения.
// baseExpr — SQL-выражение (не значение), чтобы привязаться к текущему месяцу.
func addLessons(t *testing.T, pool *pgxpool.Pool, courseID, baseExpr string, n int, status string) {
	for i := 0; i < n; i++ {
		_, err := pool.Exec(context.Background(),
			fmt.Sprintf(`INSERT INTO lessons (course_id, scheduled_at, duration_minutes, status)
			             VALUES ($1, (%s) + $2 * interval '1 hour', 60, $3)`, baseExpr),
			courseID, i, status)
		require.NoError(t, err)
	}
}

func addPayment(t *testing.T, pool *pgxpool.Pool, courseID string, amount float64, lessonsCount int, paidAtExpr string) {
	_, err := pool.Exec(context.Background(),
		fmt.Sprintf(`INSERT INTO payments (course_id, amount, lessons_count, paid_at)
		             VALUES ($1, $2, $3, %s)`, paidAtExpr),
		courseID, amount, lessonsCount)
	require.NoError(t, err)
}

// Сценарий-первопричина: за уроки текущего месяца заплатили ещё в прошлом.
// Денег в этом месяце не будет, и прогноз обязан показать 0 — старая формула
// (уроки месяца × цена урока) насчитала бы здесь полный price_per_cycle.
func TestMonthlyExpected_PrepaidMonthExpectsNothing(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewPaymentRepository(pool)
	tutorID, courseID := seedCourse(t, pool, 50000, 8)

	// Ранги 1–8 — прошлый месяц, ранги 9–16 — текущий.
	addLessons(t, pool, courseID, `date_trunc('month', NOW()) - interval '20 days'`, 8, "completed")
	addLessons(t, pool, courseID, `date_trunc('month', NOW()) + interval '2 days'`, 8, "scheduled")

	// Оба цикла оплачены в прошлом месяце: второй платёж — авансом за текущий.
	addPayment(t, pool, courseID, 50000, 8, `date_trunc('month', NOW()) - interval '25 days'`)
	addPayment(t, pool, courseID, 50000, 8, `date_trunc('month', NOW()) - interval '1 day'`)

	total, err := repo.GetMonthlyExpected(context.Background(), tutorID)
	require.NoError(t, err)
	assert.Equal(t, 0.0, total, "уроки оплачены авансом — поступлений в этом месяце не ждём")
}

// Неоплаченные циклы дают по одному ожиданию на свой первый урок, в том числе
// просроченный: деньги ждали в этом месяце и не пришли — долг остаётся в сумме.
func TestMonthlyExpected_CountsEachUnpaidCycleStart(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewPaymentRepository(pool)
	tutorID, courseID := seedCourse(t, pool, 20000, 4)

	// 8 уроков в текущем месяце, платежей нет: paid_through = 0.
	// Старты циклов — ранги 1 и 5, оба внутри месяца → ждём 2 × 20000.
	// Ранг 1 уже прошёл (2-е число) — это и есть просрочка, она в счёт идёт.
	addLessons(t, pool, courseID, `date_trunc('month', NOW()) + interval '1 day'`, 4, "completed")
	addLessons(t, pool, courseID, `date_trunc('month', NOW()) + interval '20 days'`, 4, "scheduled")

	total, err := repo.GetMonthlyExpected(context.Background(), tutorID)
	require.NoError(t, err)
	assert.Equal(t, 40000.0, total, "два неоплаченных цикла в месяце — два ожидаемых платежа")
}

// Отменённые уроки не занимают слот цикла: ранжирование в запросе обязано
// совпадать с computeCyclePositions, иначе старты циклов уедут.
func TestMonthlyExpected_IgnoresCancelledLessons(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewPaymentRepository(pool)
	tutorID, courseID := seedCourse(t, pool, 30000, 4)

	// Первые 4 урока оплачены. Пятый отменён, шестой — старт нового цикла.
	addLessons(t, pool, courseID, `date_trunc('month', NOW()) + interval '1 day'`, 4, "completed")
	addLessons(t, pool, courseID, `date_trunc('month', NOW()) + interval '10 days'`, 1, "cancelled")
	addLessons(t, pool, courseID, `date_trunc('month', NOW()) + interval '15 days'`, 1, "scheduled")
	addPayment(t, pool, courseID, 30000, 4, `date_trunc('month', NOW()) + interval '1 day'`)

	total, err := repo.GetMonthlyExpected(context.Background(), tutorID)
	require.NoError(t, err)
	assert.Equal(t, 30000.0, total, "отменённый урок пропущен — стартом цикла считается следующий за ним")
}

// Списки платежей несут курс и имя плательщика. Проверяются все три ветки
// studentNameExpr: обычный ученик, ученик без фамилии (last_name NULL — прямая
// конкатенация обнулила бы имя целиком) и групповой курс без student_id.
func TestGetAllByTutor_CarriesSubjectAndStudentName(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewPaymentRepository(pool)
	ctx := context.Background()
	tutorID, individualCourseID := seedCourse(t, pool, 10000, 4)

	// Ученик без фамилии + свой курс.
	var namelessID, namelessCourseID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO students (tutor_id, first_name, last_name)
		 VALUES ($1, 'Мария', NULL) RETURNING id`, tutorID).Scan(&namelessID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at)
		 VALUES ($1, $2, 'Физика', 10000, 4, NOW()) RETURNING id`,
		namelessID, tutorID).Scan(&namelessCourseID))

	// Групповой курс: student_id NULL.
	var groupCourseID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at)
		 VALUES (NULL, $1, 'Английский', 10000, 4, NOW()) RETURNING id`,
		tutorID).Scan(&groupCourseID))

	// Даты разнесены: GetAllByTutor сортирует по paid_at DESC.
	addPayment(t, pool, individualCourseID, 1000, 4, `NOW() - interval '3 days'`)
	addPayment(t, pool, namelessCourseID, 2000, 4, `NOW() - interval '2 days'`)
	addPayment(t, pool, groupCourseID, 3000, 4, `NOW() - interval '1 day'`)

	payments, err := repo.GetAllByTutor(ctx, tutorID, 10)
	require.NoError(t, err)
	require.Len(t, payments, 3)

	assert.Equal(t, "Английский", payments[0].Subject)
	assert.Nil(t, payments[0].StudentName, "у группового курса плательщик не один")

	assert.Equal(t, "Физика", payments[1].Subject)
	require.NotNil(t, payments[1].StudentName)
	assert.Equal(t, "Мария", *payments[1].StudentName, "отсутствие фамилии не должно съедать имя")

	assert.Equal(t, "Математика", payments[2].Subject)
	require.NotNil(t, payments[2].StudentName)
	assert.Equal(t, "Иван Петров", *payments[2].StudentName)
}
