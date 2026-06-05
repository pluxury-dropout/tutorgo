package repository

import (
	"context"
	"errors"
	"tutorgo/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentRepository interface {
	Create(ctx context.Context, req models.CreatePaymentRequest) (models.Payment, error)
	GetByCourse(ctx context.Context, courseID string, p models.Pagination) ([]models.Payment, int, error)
	GetByCoursesBatch(ctx context.Context, courseIDs []string) (map[string][]models.Payment, error)
	GetPaymentsForCalendar(ctx context.Context, tutorID string, from string, to string) (map[string][]models.Payment, error)
	GetAllByTutor(ctx context.Context, tutorID string, limit int) ([]models.Payment, error)
	GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error)
	GetBalance(ctx context.Context, courseID string) (models.CourseBalance, error)
	GetMonthlyIncome(ctx context.Context, tutorID string) (float64, error)
	GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error)
	Delete(ctx context.Context, id string, tutorID string) error
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

func (r *paymentRepository) GetByCoursesBatch(ctx context.Context, courseIDs []string) (map[string][]models.Payment, error) {
	if len(courseIDs) == 0 {
		return map[string][]models.Payment{}, nil
	}
	rows, err := r.conn.Query(ctx,
		`SELECT id, course_id, amount, lessons_count, paid_at
		 FROM payments
		 WHERE course_id = ANY($1)
		 ORDER BY course_id, paid_at ASC`,
		courseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string][]models.Payment{}
	for rows.Next() {
		var p models.Payment
		if err := rows.Scan(&p.ID, &p.CourseID, &p.Amount, &p.LessonsCount, &p.PaidAt); err != nil {
			return nil, err
		}
		result[p.CourseID] = append(result[p.CourseID], p)
	}
	return result, rows.Err()
}

func (r *paymentRepository) GetPaymentsForCalendar(ctx context.Context, tutorID string, from string, to string) (map[string][]models.Payment, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT p.id, p.course_id, p.amount, p.lessons_count, p.paid_at
		 FROM payments p
		 WHERE p.course_id IN (
		   SELECT DISTINCT l.course_id
		   FROM lessons l
		   JOIN courses c ON c.id = l.course_id
		   WHERE c.tutor_id = $1
		     AND l.scheduled_at >= $2::timestamptz
		     AND l.scheduled_at < $3::timestamptz
		 )
		 ORDER BY p.course_id, p.paid_at ASC`,
		tutorID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string][]models.Payment{}
	for rows.Next() {
		var p models.Payment
		if err := rows.Scan(&p.ID, &p.CourseID, &p.Amount, &p.LessonsCount, &p.PaidAt); err != nil {
			return nil, err
		}
		result[p.CourseID] = append(result[p.CourseID], p)
	}
	return result, rows.Err()
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

func (r *paymentRepository) Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error) {
	var payment models.Payment
	err := r.conn.QueryRow(ctx,
		`UPDATE payments SET amount=$1, lessons_count=$2, paid_at=$3
		 WHERE id=$4 AND course_id IN (SELECT id FROM courses WHERE tutor_id=$5)
		 RETURNING id, course_id, amount, lessons_count, paid_at`,
		req.Amount, req.LessonsCount, req.PaidAt, id, tutorID,
	).Scan(&payment.ID, &payment.CourseID, &payment.Amount, &payment.LessonsCount, &payment.PaidAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Payment{}, errors.New("payment not found")
	}
	return payment, err
}

func (r *paymentRepository) Delete(ctx context.Context, id string, tutorID string) error {
	tag, err := r.conn.Exec(ctx,
		`DELETE FROM payments
		 WHERE id = $1
		   AND course_id IN (SELECT id FROM courses WHERE tutor_id = $2)`,
		id, tutorID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("payment not found")
	}
	return nil
}

func (r *paymentRepository) GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error) {
	var total float64
	err := r.conn.QueryRow(ctx,
		`SELECT COALESCE(SUM((c.price_per_cycle::float / c.lessons_per_cycle) * lc.cnt), 0)
		 FROM courses c
		 JOIN (
		     SELECT course_id, COUNT(*) AS cnt
		     FROM lessons
		     WHERE status IN ('scheduled', 'completed', 'missed')
		       AND scheduled_at >= date_trunc('month', NOW())
		       AND scheduled_at <  date_trunc('month', NOW()) + interval '1 month'
		     GROUP BY course_id
		 ) lc ON lc.course_id = c.id
		 WHERE c.tutor_id = $1`,
		tutorID,
	).Scan(&total)
	return total, err
}
