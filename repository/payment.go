package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentRepository interface {
	Create(ctx context.Context, req models.CreatePaymentRequest) (models.Payment, error)
	GetByCourse(ctx context.Context, courseID string, p models.Pagination) ([]models.Payment, int, error)
	GetAllByTutor(ctx context.Context, tutorID string, limit int) ([]models.Payment, error)
	GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error)
	GetBalance(ctx context.Context, courseID string) (models.CourseBalance, error)
	GetMonthlyIncome(ctx context.Context, tutorID string) (float64, error)
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

func (r *paymentRepository) GetByCourse(ctx context.Context, courseID string, p models.Pagination) ([]models.Payment, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments WHERE course_id = $1`, courseID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, course_id, amount, lessons_count, paid_at
		 FROM payments WHERE course_id = $1
		 ORDER BY paid_at DESC
		 LIMIT $2 OFFSET $3`,
		courseID, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	payments := []models.Payment{}
	for rows.Next() {
		var payment models.Payment
		if err := rows.Scan(&payment.ID, &payment.CourseID, &payment.Amount, &payment.LessonsCount, &payment.PaidAt); err != nil {
			return nil, 0, err
		}
		payments = append(payments, payment)
	}
	return payments, total, rows.Err()
}

func (r *paymentRepository) GetAllByTutor(ctx context.Context, tutorID string, limit int) ([]models.Payment, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT p.id, p.course_id, p.amount, p.lessons_count, p.paid_at
		 FROM payments p
		 JOIN courses c ON c.id = p.course_id
		 WHERE c.tutor_id = $1
		 ORDER BY p.paid_at DESC
		 LIMIT $2`, tutorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []models.Payment
	for rows.Next() {
		var p models.Payment
		if err := rows.Scan(&p.ID, &p.CourseID, &p.Amount, &p.LessonsCount, &p.PaidAt); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

func (r *paymentRepository) GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments p
		 JOIN courses c ON c.id = p.course_id
		 WHERE c.tutor_id = $1`, tutorID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT p.id, p.course_id, p.amount, p.lessons_count, p.paid_at
		 FROM payments p
		 JOIN courses c ON c.id = p.course_id
		 WHERE c.tutor_id = $1
		 ORDER BY p.paid_at DESC
		 LIMIT $2 OFFSET $3`,
		tutorID, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	payments := []models.Payment{}
	for rows.Next() {
		var payment models.Payment
		if err := rows.Scan(&payment.ID, &payment.CourseID, &payment.Amount, &payment.LessonsCount, &payment.PaidAt); err != nil {
			return nil, 0, err
		}
		payments = append(payments, payment)
	}
	return payments, total, rows.Err()
}

func (r *paymentRepository) GetMonthlyIncome(ctx context.Context, tutorID string) (float64, error) {
	var total float64
	err := r.conn.QueryRow(ctx,
		`SELECT COALESCE(SUM(p.amount), 0)
		 FROM payments p
		 JOIN courses c ON c.id = p.course_id
		 WHERE c.tutor_id = $1
		   AND p.paid_at >= date_trunc('month', NOW())
		   AND p.paid_at <  date_trunc('month', NOW()) + interval '1 month'`,
		tutorID,
	).Scan(&total)
	return total, err
}

func (r *paymentRepository) GetBalance(ctx context.Context, courseID string) (models.CourseBalance, error) {
	var paid, completed int
	err := r.conn.QueryRow(ctx,
		`SELECT
			COALESCE((SELECT SUM(lessons_count) FROM payments WHERE course_id = $1), 0),
			COUNT(id) FILTER (WHERE status IN ('completed', 'missed'))
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
