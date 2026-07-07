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
	InsertPendingPayment(ctx context.Context, tutorID, orderID, plan string, amount int) (bool, error)
	MarkPaymentSuccess(ctx context.Context, orderID, providerPaymentID string) (bool, error)
	MarkPaymentFailed(ctx context.Context, orderID string) error
	GetPaymentByOrderID(ctx context.Context, orderID string) (*models.SubscriptionPayment, error)
	StartPaidPeriod(ctx context.Context, tutorID, plan, cardToken string, periodEnd time.Time) error
	RenewPeriod(ctx context.Context, tutorID, plan string, periodEnd time.Time) error
	ListDueAutopay(ctx context.Context, now time.Time) ([]models.DueSubscription, error)
	Cancel(ctx context.Context, tutorID string) error
	SetPendingPlan(ctx context.Context, tutorID, plan string) error
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

func (r *subscriptionRepository) InsertPendingPayment(ctx context.Context, tutorID, orderID, plan string, amount int) (bool, error) {
	tag, err := r.conn.Exec(ctx,
		`INSERT INTO subscription_payments (tutor_id, order_id, plan, amount, status)
		 VALUES ($1, $2, $3, $4, 'pending')
		 ON CONFLICT (order_id) DO NOTHING`,
		tutorID, orderID, plan, amount,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *subscriptionRepository) MarkPaymentSuccess(ctx context.Context, orderID, providerPaymentID string) (bool, error) {
	tag, err := r.conn.Exec(ctx,
		`UPDATE subscription_payments
		 SET status = 'success', provider_payment_id = $2
		 WHERE order_id = $1 AND status = 'pending'`,
		orderID, providerPaymentID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *subscriptionRepository) MarkPaymentFailed(ctx context.Context, orderID string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE subscription_payments SET status = 'failed'
		 WHERE order_id = $1 AND status = 'pending'`,
		orderID,
	)
	return err
}

func (r *subscriptionRepository) GetPaymentByOrderID(ctx context.Context, orderID string) (*models.SubscriptionPayment, error) {
	var p models.SubscriptionPayment
	err := r.conn.QueryRow(ctx,
		`SELECT tutor_id, order_id, plan, amount, status
		 FROM subscription_payments WHERE order_id = $1`,
		orderID,
	).Scan(&p.TutorID, &p.OrderID, &p.Plan, &p.Amount, &p.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *subscriptionRepository) StartPaidPeriod(ctx context.Context, tutorID, plan, cardToken string, periodEnd time.Time) error {
	tag, err := r.conn.Exec(ctx,
		`UPDATE subscriptions
		 SET plan = $2, period_end = $3, card_token = $4, autopay = TRUE,
		     pending_plan = NULL, updated_at = now()
		 WHERE tutor_id = $1`,
		tutorID, plan, periodEnd, cardToken,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("subscription not found")
	}
	return nil
}

func (r *subscriptionRepository) RenewPeriod(ctx context.Context, tutorID, plan string, periodEnd time.Time) error {
	tag, err := r.conn.Exec(ctx,
		`UPDATE subscriptions
		 SET plan = $2, period_end = $3, pending_plan = NULL, updated_at = now()
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

func (r *subscriptionRepository) ListDueAutopay(ctx context.Context, now time.Time) ([]models.DueSubscription, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT tutor_id, plan, pending_plan, card_token, period_end
		 FROM subscriptions
		 WHERE autopay = TRUE AND card_token IS NOT NULL AND period_end <= $1`,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.DueSubscription
	for rows.Next() {
		var d models.DueSubscription
		var plan *string
		if err := rows.Scan(&d.TutorID, &plan, &d.PendingPlan, &d.CardToken, &d.PeriodEnd); err != nil {
			return nil, err
		}
		if plan != nil {
			d.Plan = *plan
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *subscriptionRepository) Cancel(ctx context.Context, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE subscriptions
		 SET autopay = FALSE, card_token = NULL, pending_plan = NULL, updated_at = now()
		 WHERE tutor_id = $1`,
		tutorID,
	)
	return err
}

func (r *subscriptionRepository) SetPendingPlan(ctx context.Context, tutorID, plan string) error {
	tag, err := r.conn.Exec(ctx,
		`UPDATE subscriptions SET pending_plan = $2, updated_at = now()
		 WHERE tutor_id = $1`,
		tutorID, plan,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("subscription not found")
	}
	return nil
}
