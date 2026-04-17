package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentRepository interface {
	Create(ctx context.Context, req models.CreatePaymentRequest) (models.Payment, error)
	GetByCourse(ctx context.Context, courseID string) ([]models.Payment, error)
	GetBalance(ctx context.Context, courseID string) (models.CourseBalance, error)
}

type paymentRepository struct {
	conn *pgxpool.Pool
}

func NewPaymentRepository(conn *pgxpool.Pool) PaymentRepository {
	return &paymentRepository{conn: conn}
}

func (r *paymentRepository) Create(ctx context.Context, req models.CreatePaymentRequest) (models.Payment, error) {
	var payment models.Payment
	err := r.conn.QueryRow(ctx,
		`INSERT INTO payments (course_id, amount, lessons_count, paid_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, course_id, amount, lessons_count, paid_at`,
		req.CourseID, req.Amount, req.LessonsCount, req.PaidAt,
	).Scan(&payment.ID, &payment.CourseID, &payment.Amount, &payment.LessonsCount, &payment.PaidAt)
	return payment, err
}

func (r *paymentRepository) GetByCourse(ctx context.Context, courseID string) ([]models.Payment, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT id, course_id, amount, lessons_count, paid_at
		 FROM payments WHERE course_id = $1`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []models.Payment
	for rows.Next() {
		var payment models.Payment
		err := rows.Scan(&payment.ID, &payment.CourseID, &payment.Amount, &payment.LessonsCount, &payment.PaidAt)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}
	return payments, rows.Err()
}

func (r *paymentRepository) GetBalance(ctx context.Context, courseID string) (models.CourseBalance, error) {
	var paid, completed int
	err := r.conn.QueryRow(ctx,
		`SELECT
			COALESCE((SELECT SUM(lessons_count) FROM payments WHERE course_id = $1), 0),
			COUNT(id) FILTER (WHERE status = 'completed')
		FROM lessons
		WHERE course_id = $1`,
		courseID,
	).Scan(&paid, &completed)
	if err != nil {
		return models.CourseBalance{}, err
	}
	return models.CourseBalance{
		LessonsPaid:      paid,
		LessonsCompleted: completed,
		LessonsRemaining: paid - completed,
	}, nil
}
