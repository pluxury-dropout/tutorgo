package repository

import (
	"context"
	"errors"
	"time"

	"tutorgo/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier реализуют и *pgxpool.Pool, и pgx.Tx — позволяет одному методу
// работать как вне, так и внутри транзакции.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type SubscriptionRepository interface {
	GetByTutor(ctx context.Context, tutorID string) (*models.Subscription, error)
	Activate(ctx context.Context, tutorID, plan string, periodEnd time.Time) error
	CreateTrialTx(ctx context.Context, q Querier, tutorID string) error
}

type subscriptionRepository struct {
	conn *pgxpool.Pool
}

func NewSubscriptionRepository(conn *pgxpool.Pool) SubscriptionRepository {
	return &subscriptionRepository{conn: conn}
}

func (r *subscriptionRepository) GetByTutor(ctx context.Context, tutorID string) (*models.Subscription, error) {
	var s models.Subscription
	err := r.conn.QueryRow(ctx,
		`SELECT tutor_id, plan, period_end, grandfathered
		 FROM subscriptions WHERE tutor_id = $1`,
		tutorID,
	).Scan(&s.TutorID, &s.Plan, &s.PeriodEnd, &s.Grandfathered)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // нет строки — вызывающий трактует как blocked
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *subscriptionRepository) Activate(ctx context.Context, tutorID, plan string, periodEnd time.Time) error {
	tag, err := r.conn.Exec(ctx,
		`UPDATE subscriptions
		 SET plan = $2, period_end = $3, updated_at = now()
		 WHERE tutor_id = $1`,
		tutorID, plan, periodEnd,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("subscription not found")
	}
	return nil
}

func (r *subscriptionRepository) CreateTrialTx(ctx context.Context, q Querier, tutorID string) error {
	_, err := q.Exec(ctx,
		`INSERT INTO subscriptions (tutor_id, plan, period_end, grandfathered)
		 VALUES ($1, NULL, now() + interval '30 days', FALSE)`,
		tutorID,
	)
	return err
}
