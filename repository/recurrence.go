package repository

import (
	"context"
	"time"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RecurrenceRepository interface {
	Create(ctx context.Context, rule models.RecurrenceRule) (models.RecurrenceRule, error)
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (models.RecurrenceRule, error)
	DueForMaterialization(ctx context.Context, horizon time.Time) ([]models.RecurrenceRule, error)
	SetMaterializedUntil(ctx context.Context, id string, until time.Time) error
	InsertOccurrences(ctx context.Context, ruleID string, starts []time.Time) (int, error)
	Split(ctx context.Context, ruleID string, at time.Time, timeLocal string, duration int) (models.RecurrenceRule, error)
	UpdateTiming(ctx context.Context, ruleID, timeLocal string, duration int) error
	SetEndsOn(ctx context.Context, ruleID string, endsOn time.Time) error
}

type recurrenceRepository struct {
	pool *pgxpool.Pool
}

func NewRecurrenceRepository(pool *pgxpool.Pool) RecurrenceRepository {
	return &recurrenceRepository{pool: pool}
}

// time_local читаем как text: pgx отдаёт TIME микросекундами, а генератору
// нужны стенные часы «17:00».
const ruleCols = `id, tutor_id, freq, interval_n, byweekday, time_local::text,
                  tz, duration_minutes, starts_on, ends_on, max_count, materialized_until`

func scanRule(row interface{ Scan(...any) error }) (models.RecurrenceRule, error) {
	var r models.RecurrenceRule
	err := row.Scan(&r.ID, &r.TutorID, &r.Freq, &r.IntervalN, &r.ByWeekday, &r.TimeLocal,
		&r.TZ, &r.DurationMinutes, &r.StartsOn, &r.EndsOn, &r.MaxCount, &r.MaterializedUntil)
	return r, err
}

func (r *recurrenceRepository) Create(ctx context.Context, rule models.RecurrenceRule) (models.RecurrenceRule, error) {
	return scanRule(r.pool.QueryRow(ctx,
		`INSERT INTO recurrence_rules
		     (tutor_id, freq, interval_n, byweekday, time_local, tz, duration_minutes,
		      starts_on, ends_on, max_count, materialized_until)
		 VALUES ($1::uuid, $2, $3, $4::smallint[], $5::time, $6, $7, $8::date, $9::date, $10, $11::date)
		 RETURNING `+ruleCols,
		rule.TutorID, rule.Freq, rule.IntervalN, rule.ByWeekday, rule.TimeLocal, rule.TZ,
		rule.DurationMinutes, rule.StartsOn, rule.EndsOn, rule.MaxCount, rule.MaterializedUntil,
	))
}

// Delete нужен на откате: правило без первого вхождения — сирота, шаблона для
// материализации у него нет, и висеть в базе ему незачем.
func (r *recurrenceRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM recurrence_rules WHERE id = $1`, id)
	return err
}

func (r *recurrenceRepository) GetByID(ctx context.Context, id string) (models.RecurrenceRule, error) {
	return scanRule(r.pool.QueryRow(ctx, `SELECT `+ruleCols+` FROM recurrence_rules WHERE id = $1`, id))
}

// DueForMaterialization — активные правила, чей горизонт короче требуемого.
func (r *recurrenceRepository) DueForMaterialization(ctx context.Context, horizon time.Time) ([]models.RecurrenceRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+ruleCols+` FROM recurrence_rules
		 WHERE materialized_until < $1::date
		   AND (ends_on IS NULL OR ends_on > CURRENT_DATE)`, horizon)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := []models.RecurrenceRule{}
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// Split — «это и все следующие»: старое правило закрывается днём раньше
// вхождения, а с него начинается копия с новым временем и длительностью.
// Ветвление, а не правка на месте: прошедшие вхождения должны остаться такими,
// какими были, иначе история занятий начнёт врать.
func (r *recurrenceRepository) Split(ctx context.Context, ruleID string, at time.Time, timeLocal string, duration int) (models.RecurrenceRule, error) {
	if _, err := r.pool.Exec(ctx,
		`UPDATE recurrence_rules SET ends_on = $2::date - 1 WHERE id = $1`, ruleID, at,
	); err != nil {
		return models.RecurrenceRule{}, err
	}

	return scanRule(r.pool.QueryRow(ctx,
		`INSERT INTO recurrence_rules
		     (tutor_id, freq, interval_n, byweekday, time_local, tz, duration_minutes,
		      starts_on, ends_on, max_count, materialized_until)
		 SELECT tutor_id, freq, interval_n, byweekday, $3::time, tz, $4,
		        $2::date, NULL, max_count, $2::date
		 FROM recurrence_rules WHERE id = $1
		 RETURNING `+ruleCols,
		ruleID, at, timeLocal, duration,
	))
}

func (r *recurrenceRepository) SetEndsOn(ctx context.Context, ruleID string, endsOn time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE recurrence_rules SET ends_on = $2::date WHERE id = $1`, ruleID, endsOn)
	return err
}

func (r *recurrenceRepository) UpdateTiming(ctx context.Context, ruleID, timeLocal string, duration int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE recurrence_rules SET time_local = $2::time, duration_minutes = $3 WHERE id = $1`,
		ruleID, timeLocal, duration)
	return err
}

func (r *recurrenceRepository) SetMaterializedUntil(ctx context.Context, id string, until time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE recurrence_rules SET materialized_until = $2::date WHERE id = $1`, id, until)
	return err
}

// InsertOccurrences создаёт недостающие вхождения правила. Шаблон — первая
// строка серии: у урока из неё берутся курс, длительность и заметки, у события
// — заголовок, вид и место. Поэтому отдельного «типа правила» в схеме нет:
// правило про уроки просто не найдёт шаблона в events, и наоборот.
//
// occurrence_date считается в зоне правила: в UTC урок 1 сентября 00:30 по
// Алматы был бы записан на 31 августа, и «одно вхождение в день» поехало бы.
func (r *recurrenceRepository) InsertOccurrences(ctx context.Context, ruleID string, starts []time.Time) (int, error) {
	lessons, err := r.pool.Exec(ctx,
		`INSERT INTO lessons (course_id, scheduled_at, duration_minutes, notes, status, rule_id, occurrence_date)
		 SELECT t.course_id, d.at, t.duration_minutes, t.notes,
		        CASE WHEN d.at + t.duration_minutes * interval '1 minute' < NOW() THEN 'completed' ELSE 'scheduled' END,
		        $1::uuid, (d.at AT TIME ZONE r.tz)::date
		 FROM recurrence_rules r
		 JOIN LATERAL (
		     SELECT course_id, duration_minutes, notes FROM lessons
		     WHERE rule_id = $1::uuid ORDER BY scheduled_at LIMIT 1
		 ) t ON TRUE
		 CROSS JOIN unnest($2::timestamptz[]) AS d(at)
		 WHERE r.id = $1::uuid
		 ON CONFLICT DO NOTHING`,
		ruleID, starts,
	)
	if err != nil {
		return 0, err
	}

	events, err := r.pool.Exec(ctx,
		`INSERT INTO events (tutor_id, title, kind, starts_at, duration_minutes, color, location, notes, rule_id, occurrence_date)
		 SELECT t.tutor_id, t.title, t.kind, d.at, t.duration_minutes, t.color, t.location, t.notes,
		        $1::uuid, (d.at AT TIME ZONE r.tz)::date
		 FROM recurrence_rules r
		 JOIN LATERAL (
		     SELECT tutor_id, title, kind, duration_minutes, color, location, notes FROM events
		     WHERE rule_id = $1::uuid ORDER BY starts_at LIMIT 1
		 ) t ON TRUE
		 CROSS JOIN unnest($2::timestamptz[]) AS d(at)
		 WHERE r.id = $1::uuid
		 ON CONFLICT DO NOTHING`,
		ruleID, starts,
	)
	if err != nil {
		return 0, err
	}
	return int(lessons.RowsAffected() + events.RowsAffected()), nil
}
