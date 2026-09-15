//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Оплата на несколько предметов пишется целиком или никак (спека
// 2026-09-06-price-units…, п. 6.8). Запуск: make test-integration.
func TestCreateBulk_AllOrNothing(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tutorID, s := seedTutorStudent(t, pool)
	math := addIndividualCourse(t, pool, tutorID, s)
	var physics string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at)
		 VALUES ($1, $2, 'Физика', 6000, 1, NOW() - interval '1 month') RETURNING id`,
		s, tutorID).Scan(&physics))

	repo := repository.NewPaymentRepository(pool)
	paidAt := time.Date(2026, time.September, 14, 0, 0, 0, 0, time.UTC)
	countPayments := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM payments WHERE course_id = ANY($1::uuid[])`,
			[]string{math, physics}).Scan(&n))
		return n
	}

	// Вторая строка ссылается на несуществующего ученика — внешний ключ валит
	// оператор, и первая строка не должна остаться в базе.
	_, err := repo.CreateBulk(ctx, models.CreateBulkPaymentRequest{PaidAt: paidAt, Items: []models.BulkPaymentItem{
		{CourseID: math, StudentID: s, Amount: 40000, LessonsCount: 8},
		{CourseID: physics, StudentID: uuid.NewString(), Amount: 24000, LessonsCount: 4},
	}})
	require.Error(t, err)
	assert.Equal(t, 0, countPayments())

	payments, err := repo.CreateBulk(ctx, models.CreateBulkPaymentRequest{PaidAt: paidAt, Items: []models.BulkPaymentItem{
		{CourseID: math, StudentID: s, Amount: 40000, LessonsCount: 8},
		{CourseID: physics, StudentID: s, Amount: 24000, LessonsCount: 4},
	}})
	require.NoError(t, err)
	require.Len(t, payments, 2)
	assert.Equal(t, 2, countPayments())
	for _, p := range payments {
		require.NotNil(t, p.StudentID)
		assert.Equal(t, s, *p.StudentID)
		assert.True(t, paidAt.Equal(p.PaidAt))
	}
}
