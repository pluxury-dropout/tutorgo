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
	GetByID(ctx context.Context, id string, tutorID string) (models.Payment, error)
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

// Одна константа на все выборки платежа — тот же приём, что lessonColumns в
// lesson.go: колонку, добавленную руками в семь запросов, где-нибудь забудут.
// Алиас `p` обязателен и там, где джойна нет.
const paymentColumns = `p.id, p.course_id, p.student_id, p.amount, p.lessons_count, p.paid_at`

// paymentDest — приёмники Scan в порядке paymentColumns; списки дописывают свои
// поля через append.
func paymentDest(p *models.Payment) []any {
	return []any{&p.ID, &p.CourseID, &p.StudentID, &p.Amount, &p.LessonsCount, &p.PaidAt}
}

func (r *paymentRepository) Create(ctx context.Context, req models.CreatePaymentRequest) (models.Payment, error) {
	var payment models.Payment
	err := r.conn.QueryRow(ctx,
		`INSERT INTO payments AS p (course_id, student_id, amount, lessons_count, paid_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+paymentColumns,
		req.CourseID, req.StudentID, req.Amount, req.LessonsCount, req.PaidAt,
	).Scan(paymentDest(&payment)...)
	return payment, err
}

// GetByID — платёж репетитора; чужой неотличим от несуществующего.
func (r *paymentRepository) GetByID(ctx context.Context, id string, tutorID string) (models.Payment, error) {
	var payment models.Payment
	err := r.conn.QueryRow(ctx,
		`SELECT `+paymentColumns+`
		 FROM payments p
		 JOIN courses c ON c.id = p.course_id
		 WHERE p.id = $1 AND c.tutor_id = $2`, id, tutorID,
	).Scan(paymentDest(&payment)...)
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
		`SELECT `+paymentColumns+`, `+studentNameExpr+`
		 FROM payments p
		 LEFT JOIN students s ON s.id = p.student_id
		 WHERE p.course_id = $1
		 ORDER BY p.paid_at DESC
		 LIMIT $2 OFFSET $3`,
		courseID, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	payments := []models.Payment{}
	for rows.Next() {
		var payment models.Payment
		if err := rows.Scan(append(paymentDest(&payment), &payment.StudentName)...); err != nil {
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
		`SELECT `+paymentColumns+` FROM payments p WHERE p.course_id = ANY($1) ORDER BY p.course_id, p.paid_at ASC`,
		courseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string][]models.Payment{}
	for rows.Next() {
		var p models.Payment
		if err := rows.Scan(paymentDest(&p)...); err != nil {
			return nil, err
		}
		result[p.CourseID] = append(result[p.CourseID], p)
	}
	return result, rows.Err()
}

func (r *paymentRepository) GetPaymentsForCalendar(ctx context.Context, tutorID string, from string, to string) (map[string][]models.Payment, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT `+paymentColumns+`
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
		if err := rows.Scan(paymentDest(&p)...); err != nil {
			return nil, err
		}
		result[p.CourseID] = append(result[p.CourseID], p)
	}
	return result, rows.Err()
}

// studentNameExpr — имя адресата платежа; NULL у легаси-платежа группы без
// адресата (спека, п. 3.4). Ожидает LEFT JOIN students s ON s.id = p.student_id.
// TRIM с COALESCE, а не конкатенация напрямую: last_name в схеме nullable, и
// `first_name || ' ' || NULL` даёт NULL — имя пропало бы целиком.
const studentNameExpr = `CASE WHEN p.student_id IS NULL THEN NULL
                              ELSE TRIM(s.first_name || ' ' || COALESCE(s.last_name, ''))
                         END`

func (r *paymentRepository) GetAllByTutor(ctx context.Context, tutorID string, limit int) ([]models.Payment, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT `+paymentColumns+`, c.subject, `+studentNameExpr+`
		 FROM payments p
		 JOIN courses c ON c.id = p.course_id
		 LEFT JOIN students s ON s.id = p.student_id
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
		if err := rows.Scan(append(paymentDest(&p), &p.Subject, &p.StudentName)...); err != nil {
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
		`SELECT `+paymentColumns+`, c.subject, `+studentNameExpr+`
		 FROM payments p
		 JOIN courses c ON c.id = p.course_id
		 LEFT JOIN students s ON s.id = p.student_id
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
		if err := rows.Scan(append(paymentDest(&payment), &payment.Subject, &payment.StudentName)...); err != nil {
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
		`UPDATE payments AS p SET student_id=$1, amount=$2, lessons_count=$3, paid_at=$4
		 WHERE p.id=$5 AND p.course_id IN (SELECT id FROM courses WHERE tutor_id=$6)
		 RETURNING `+paymentColumns,
		req.StudentID, req.Amount, req.LessonsCount, req.PaidAt, id, tutorID,
	).Scan(paymentDest(&payment)...)
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

// GetMonthlyExpected — сумма платежей, ожидаемых к поступлению в текущем месяце.
//
// Ученик платит за цикл на его первом уроке, поэтому ожидаемое поступление
// привязано к дате первого урока каждого ещё не оплаченного цикла. Циклы
// нарезаются так же, как в computeCyclePositions: уроки курса ранжируются по
// scheduled_at, платежи покрывают ранги нарастающим итогом по lessons_count.
// Урок с rank = paid_through+1 открывает первый неоплаченный цикл, дальше
// старты идут с шагом lessons_per_cycle.
//
// Просроченные ожидания (урок цикла уже прошёл, а платежа нет) остаются в сумме:
// деньги ждали в этом месяце и не пришли — долг не должен исчезать из метрики.
//
// ponytail: будущие циклы нарезаются по плановому lessons_per_cycle, тогда как
// прошлые — по фактическим lessons_count платежей. Если ученик регулярно платит
// нестандартными пачками, даты прогноза поплывут. Апгрейд — медиана lessons_count
// последних платежей курса; делать, только если реально разъедется.
func (r *paymentRepository) GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error) {
	var total float64
	err := r.conn.QueryRow(ctx,
		`WITH course_paid AS (
		     SELECT c.id, c.price_per_cycle, c.lessons_per_cycle,
		            COALESCE(SUM(p.lessons_count), 0) AS paid_through
		     FROM courses c
		     LEFT JOIN payments p ON p.course_id = c.id
		     WHERE c.tutor_id = $1 AND c.is_active
		     GROUP BY c.id
		 ),
		 ranked AS (
		     SELECT l.course_id, l.scheduled_at,
		            ROW_NUMBER() OVER (PARTITION BY l.course_id ORDER BY l.scheduled_at) AS rank
		     FROM lessons l
		     JOIN course_paid cp ON cp.id = l.course_id
		     WHERE l.status != 'cancelled'
		 )
		 SELECT COALESCE(SUM(cp.price_per_cycle), 0)
		 FROM ranked r
		 JOIN course_paid cp ON cp.id = r.course_id
		 WHERE r.rank > cp.paid_through
		   AND (r.rank - cp.paid_through - 1) % cp.lessons_per_cycle = 0
		   AND r.scheduled_at >= date_trunc('month', NOW())
		   AND r.scheduled_at <  date_trunc('month', NOW()) + interval '1 month'`,
		tutorID,
	).Scan(&total)
	return total, err
}
