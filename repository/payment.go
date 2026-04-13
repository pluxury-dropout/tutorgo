package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentRepository interface {
	Create(req models.CreatePaymentRequest) (models.Payment, error)
	GetByCourse(courseID string) ([]models.Payment, error)
	GetBalance(courseID string) (models.CourseBalance, error)
}

type paymentRepository struct {
	conn *pgxpool.Pool
}

func NewPaymentRepository(conn *pgxpool.Pool) PaymentRepository {
	return &paymentRepository{conn: conn}
}

func (r *paymentRepository) Create(req models.CreatePaymentRequest) (models.Payment, error) {
	var payment models.Payment
	err := r.conn.QueryRow(context.Background(),
		`INSERT INTO payments (course_id, amount, lessons_count, paid_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, course_id, amount, lessons_count, paid_at`,
		req.CourseID, req.Amount, req.LessonsCount, req.PaidAt,
	).Scan(&payment.ID, &payment.CourseID, &payment.Amount, &payment.LessonsCount, &payment.PaidAt)
	return payment, err
}

func (r *paymentRepository) GetByCourse(courseID string) ([]models.Payment, error) {
	rows, err := r.conn.Query(context.Background(),
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

func (r *paymentRepository) GetBalance(courseID string) (models.CourseBalance, error) {
	var paid, completed int
	err := r.conn.QueryRow(context.Background(),
		`SELECT
			COALESCE(SUM(p.lessons_count), 0),
			COUNT(l.id) FILTER (WHERE l.status = 'completed')
		FROM payments p
		LEFT JOIN lessons l ON l.course_id = p.course_id
		WHERE p.course_id = $1`,
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
