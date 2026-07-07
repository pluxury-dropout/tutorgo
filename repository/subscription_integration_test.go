//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"tutorgo/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Требует применённой migration 019 и репетитора-фикстуры.
// Запуск: go test -tags=integration ./repository/ -run TestClaim
func connect(t *testing.T) *pgxpool.Pool {
	url := os.Getenv("DB_URL")
	if url == "" {
		t.Skip("DB_URL не задан — пропускаем integration-тест")
	}
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// seedTutorWithSubscription создаёт минимально валидного репетитора и его
// подписку (план monthly, period_end = now), регистрирует cleanup, возвращает UUID.
func seedTutorWithSubscription(t *testing.T, pool *pgxpool.Pool) string {
	ctx := context.Background()
	email := fmt.Sprintf("integration-%d@example.com", time.Now().UnixNano())

	var tutorID string
	err := pool.QueryRow(ctx,
		`INSERT INTO tutors (email, password_hash, first_name, last_name)
		 VALUES ($1, 'x', 'Test', 'Tutor')
		 RETURNING id`,
		email,
	).Scan(&tutorID)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM tutors WHERE id=$1`, tutorID) })

	_, err = pool.Exec(ctx,
		`INSERT INTO subscriptions (tutor_id, plan, period_end)
		 VALUES ($1, 'monthly', now())`,
		tutorID,
	)
	require.NoError(t, err)

	return tutorID
}

func TestClaim_InsertPending_OnlyOneWinsConcurrently(t *testing.T) {
	pool := connect(t)
	repo := repository.NewSubscriptionRepository(pool)
	ctx := context.Background()
	tutorID := seedTutorWithSubscription(t, pool) // helper: INSERT tutor + subscription, вернуть id
	orderID := tutorID + ":claim-test"
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM subscription_payments WHERE order_id=$1`, orderID) })

	const n = 8
	var wg sync.WaitGroup
	claims := make([]bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			claimed, err := repo.InsertPendingPayment(ctx, tutorID, orderID, "monthly", 10000)
			assert.NoError(t, err)
			claims[i] = claimed
		}(i)
	}
	wg.Wait()

	won := 0
	for _, c := range claims {
		if c {
			won++
		}
	}
	assert.Equal(t, 1, won, "ровно один инстанс должен выиграть insert-claim")
}

func TestClaim_MarkSuccess_OnlyOneActivatesConcurrently(t *testing.T) {
	pool := connect(t)
	repo := repository.NewSubscriptionRepository(pool)
	ctx := context.Background()
	tutorID := seedTutorWithSubscription(t, pool)
	orderID := tutorID + ":success-test"
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM subscription_payments WHERE order_id=$1`, orderID) })
	_, err := repo.InsertPendingPayment(ctx, tutorID, orderID, "monthly", 10000)
	require.NoError(t, err)

	const n = 8
	var wg sync.WaitGroup
	acts := make([]bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			activated, err := repo.MarkPaymentSuccess(ctx, orderID, "pp-"+orderID)
			assert.NoError(t, err)
			acts[i] = activated
		}(i)
	}
	wg.Wait()

	won := 0
	for _, a := range acts {
		if a {
			won++
		}
	}
	assert.Equal(t, 1, won, "ровно один вызов должен активировать период")
}
