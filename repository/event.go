package repository

import (
	"context"
	"errors"
	"time"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrEventNotFound = errors.New("event not found")

const eventColumns = `id, tutor_id, title, kind, starts_at, duration_minutes, color, location, notes,
                      rule_id, occurrence_date`

type EventRepository interface {
	Create(ctx context.Context, tutorID string, req models.CreateEventRequest) (models.Event, error)
	GetByID(ctx context.Context, id, tutorID string) (models.Event, error)
	GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Event, error)
	Update(ctx context.Context, id, tutorID string, req models.UpdateEventRequest) (models.Event, error)
	Delete(ctx context.Context, id, tutorID string) error
	Cancel(ctx context.Context, id, tutorID string) error
	MarkOverride(ctx context.Context, id, tutorID string) error
	ReassignToRule(ctx context.Context, eventID, ruleID string, occurrenceDate time.Time) error
	DeleteFutureByRule(ctx context.Context, ruleID string, after time.Time) error
	// GetOccupiedInRange отдаёт всё, что занимает время в пересечении с [from, to):
	// уроки в статусе scheduled и все события. Задачи занятостью не считаются.
	GetOccupiedInRange(ctx context.Context, tutorID, from, to string, excludeType string, excludeID *string) ([]models.CalendarItem, error)
}

type eventRepository struct {
	conn *pgxpool.Pool
}

func NewEventRepository(conn *pgxpool.Pool) EventRepository {
	return &eventRepository{conn: conn}
}

func scanEvent(row interface{ Scan(...any) error }) (models.Event, error) {
	var e models.Event
	err := row.Scan(&e.ID, &e.TutorID, &e.Title, &e.Kind, &e.StartsAt, &e.DurationMinutes,
		&e.Color, &e.Location, &e.Notes, &e.RuleID, &e.OccurrenceDate)
	return e, err
}

func (r *eventRepository) Create(ctx context.Context, tutorID string, req models.CreateEventRequest) (models.Event, error) {
	return scanEvent(r.conn.QueryRow(ctx,
		`INSERT INTO events (tutor_id, title, kind, starts_at, duration_minutes, color, location, notes, rule_id, occurrence_date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, '')::uuid, $10::date)
		 RETURNING `+eventColumns,
		tutorID, req.Title, req.Kind, req.StartsAt, req.DurationMinutes,
		req.Color, req.Location, req.Notes, req.RuleID, req.OccurrenceDate,
	))
}

func (r *eventRepository) GetByID(ctx context.Context, id, tutorID string) (models.Event, error) {
	return scanEvent(r.conn.QueryRow(ctx,
		`SELECT `+eventColumns+` FROM events WHERE id = $1 AND tutor_id = $2`, id, tutorID,
	))
}

// Cancel — «удалить только это вхождение». Строка остаётся тумбстоуном: она
// занимает дату в уникальном индексе (rule_id, occurrence_date), и ночная
// материализация не создаёт событие заново.
func (r *eventRepository) Cancel(ctx context.Context, id, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE events SET cancelled = TRUE, is_override = TRUE WHERE id = $1 AND tutor_id = $2`,
		id, tutorID)
	return err
}

// MarkOverride помечает вхождение вручную правленным — та же защита от
// «это и все следующие», что и у отмены, но для переноса и правки.
func (r *eventRepository) MarkOverride(ctx context.Context, id, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE events SET is_override = TRUE WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}

func (r *eventRepository) ReassignToRule(ctx context.Context, eventID, ruleID string, occurrenceDate time.Time) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE events SET rule_id = $2::uuid, occurrence_date = $3::date, is_override = FALSE
		 WHERE id = $1`, eventID, ruleID, occurrenceDate)
	return err
}

// DeleteFutureByRule убирает будущие вхождения, кроме вручную перенесённых и
// уже отменённых: и те, и другие — осознанные решения пользователя.
func (r *eventRepository) DeleteFutureByRule(ctx context.Context, ruleID string, after time.Time) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM events
		 WHERE rule_id = $1::uuid AND occurrence_date > $2::date
		   AND is_override = FALSE AND NOT cancelled`, ruleID, after)
	return err
}

func (r *eventRepository) GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Event, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT `+eventColumns+`
		 FROM events
		 WHERE tutor_id = $1 AND starts_at >= $2::timestamptz AND starts_at < $3::timestamptz
		   AND NOT cancelled
		 ORDER BY starts_at`,
		tutorID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []models.Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *eventRepository) Update(ctx context.Context, id, tutorID string, req models.UpdateEventRequest) (models.Event, error) {
	return scanEvent(r.conn.QueryRow(ctx,
		`UPDATE events
		 SET title=$1, kind=$2, starts_at=$3, duration_minutes=$4,
		     color=$5, location=$6, notes=$7, updated_at=NOW()
		 WHERE id=$8 AND tutor_id=$9
		 RETURNING `+eventColumns,
		req.Title, req.Kind, req.StartsAt, req.DurationMinutes,
		req.Color, req.Location, req.Notes, id, tutorID,
	))
}

func (r *eventRepository) Delete(ctx context.Context, id, tutorID string) error {
	tag, err := r.conn.Exec(ctx, `DELETE FROM events WHERE id=$1 AND tutor_id=$2`, id, tutorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrEventNotFound
	}
	return nil
}

func (r *eventRepository) GetOccupiedInRange(ctx context.Context, tutorID, from, to string, excludeType string, excludeID *string) ([]models.CalendarItem, error) {
	// Пересечение полуинтервалов: начало раньше конца слота И конец позже начала слота.
	// Исключение самого себя ($4/$5) нужно при переносе: иначе элемент конфликтует с собой.
	rows, err := r.conn.Query(ctx,
		`SELECT 'lesson' AS type, l.id::text,
		        CASE WHEN c.student_id IS NULL OR s.first_name IS NULL THEN c.subject
		             ELSE c.subject || ' — ' || s.first_name
		        END,
		        l.scheduled_at, l.duration_minutes
		 FROM lessons l
		 JOIN courses c ON c.id = l.course_id
		 LEFT JOIN students s ON s.id = c.student_id
		 WHERE c.tutor_id = $1
		   AND l.status = 'scheduled'
		   AND l.scheduled_at < $3::timestamptz
		   AND l.scheduled_at + l.duration_minutes * interval '1 minute' > $2::timestamptz
		   AND ($4 <> 'lesson' OR l.id IS DISTINCT FROM $5::uuid)
		 UNION ALL
		 SELECT 'event' AS type, e.id::text, e.title, e.starts_at, e.duration_minutes
		 FROM events e
		 WHERE e.tutor_id = $1
		   AND NOT e.cancelled
		   AND e.starts_at < $3::timestamptz
		   AND e.starts_at + e.duration_minutes * interval '1 minute' > $2::timestamptz
		   AND ($4 <> 'event' OR e.id IS DISTINCT FROM $5::uuid)
		 ORDER BY 4`,
		tutorID, from, to, excludeType, excludeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.CalendarItem
	for rows.Next() {
		var it models.CalendarItem
		if err := rows.Scan(&it.Type, &it.ID, &it.Title, &it.StartsAt, &it.DurationMinutes); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}
