package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PauseRepository interface {
	Create(ctx context.Context, studentID string, req models.CreatePauseRequest) (models.StudentPause, []string, error)
	ListByStudent(ctx context.Context, studentID string) ([]models.StudentPause, error)
	Delete(ctx context.Context, id, studentID string) (bool, error)
}

type pauseRepository struct {
	pool *pgxpool.Pool
}

func NewPauseRepository(pool *pgxpool.Pool) PauseRepository {
	return &pauseRepository{pool: pool}
}

// Create заводит паузу, отменяет уроки индивидуальных курсов ученика в её
// интервале и сдвигает хвост их правил — одним оператором (спека, п. 6.9).
// Отдельными запросами правило на миг оказалось бы в состоянии «конец сдвинут,
// счётчик нет», а обрыв посередине оставил бы отменённые уроки без продления.
//
// Отмена нужна, иначе автозавершение закроет уроки паузы как проведённые.
// Хвост: ends_on += дней в паузе, max_count += отменённых вхождений правила;
// NULL + n остаётся NULL, поэтому бессрочное правило не меняется само.
// materialized_until откатывается к началу паузы: Materialize идёт от него, и
// без отката хвост за старым концом не появился бы никогда. Дублей не будет —
// (rule_id, occurrence_date) уникален, отменённые держатся тумбстоунами.
// Групповые уроки и правила не трогаются: занятия идут для остальных.
//
// Возвращает id правил со сдвинутым хвостом — их материализует сервис.
//
// ponytail: даты уроков сравниваются в UTC, как в lessonInParticipation. И
// вхождения, ещё не материализованные к моменту заморозки (пауза дальше
// горизонта в 6 месяцев), потом создадутся запланированными — ceiling редкий,
// апгрейд — учитывать паузы в Materialize.
func (r *pauseRepository) Create(ctx context.Context, studentID string, req models.CreatePauseRequest) (models.StudentPause, []string, error) {
	var p models.StudentPause
	var shifted []string
	err := r.pool.QueryRow(ctx,
		`WITH pause AS (
		     INSERT INTO student_pauses (student_id, starts_on, ends_on, reason)
		     VALUES ($1, $2::date, $3::date, $4)
		     RETURNING id, student_id, starts_on, ends_on, reason, created_at
		 ),
		 cancelled AS (
		     UPDATE lessons AS l SET status = 'cancelled'
		       FROM courses c, pause
		      WHERE c.id = l.course_id
		        AND c.student_id = pause.student_id
		        AND l.status = 'scheduled'
		        AND l.scheduled_at::date BETWEEN pause.starts_on AND pause.ends_on
		     RETURNING l.rule_id
		 ),
		 per_rule AS (
		     SELECT rule_id, count(*)::int AS n
		       FROM cancelled
		      WHERE rule_id IS NOT NULL
		      GROUP BY rule_id
		 ),
		 shifted AS (
		     UPDATE recurrence_rules AS r
		        SET ends_on            = r.ends_on + ($3::date - $2::date + 1),
		            max_count          = r.max_count + pr.n,
		            materialized_until = LEAST(r.materialized_until, $2::date)
		       FROM per_rule pr
		      WHERE r.id = pr.rule_id
		     RETURNING r.id::text
		 )
		 SELECT p.id, p.student_id, p.starts_on, p.ends_on, p.reason, p.created_at,
		        ARRAY(SELECT id FROM shifted)
		   FROM pause p`,
		studentID, req.StartsOn, req.EndsOn, req.Reason,
	).Scan(&p.ID, &p.StudentID, &p.StartsOn, &p.EndsOn, &p.Reason, &p.CreatedAt, &shifted)
	return p, shifted, err
}

func (r *pauseRepository) ListByStudent(ctx context.Context, studentID string) ([]models.StudentPause, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, student_id, starts_on, ends_on, reason, created_at
		 FROM student_pauses
		 WHERE student_id = $1
		 ORDER BY starts_on DESC`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pauses := []models.StudentPause{}
	for rows.Next() {
		var p models.StudentPause
		if err := rows.Scan(&p.ID, &p.StudentID, &p.StartsOn, &p.EndsOn, &p.Reason, &p.CreatedAt); err != nil {
			return nil, err
		}
		pauses = append(pauses, p)
	}
	return pauses, rows.Err()
}

// Delete — разморозка: уроки паузы возвращаются в счёт. Отменённые уроки и
// сдвинутый хвост не откатываются — расписание уже пересобрано (спека, п. 6.9).
func (r *pauseRepository) Delete(ctx context.Context, id, studentID string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM student_pauses WHERE id = $1 AND student_id = $2`, id, studentID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
