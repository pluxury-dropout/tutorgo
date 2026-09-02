package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"tutorgo/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LessonRepository interface {
	Create(ctx context.Context, req models.CreateLessonRequest) (models.Lesson, error)
	CreateBulk(ctx context.Context, req models.CreateBulkLessonRequest) ([]models.Lesson, error)
	GetByCourse(ctx context.Context, courseID string) ([]models.Lesson, error)
	GetByCoursePaged(ctx context.Context, courseID string, p models.Pagination) ([]models.Lesson, int, error)
	GetByID(ctx context.Context, id string) (models.Lesson, error)
	GetByIDForTutor(ctx context.Context, id string, tutorID string) (models.Lesson, error)
	Update(ctx context.Context, id string, req models.UpdateLessonRequest) (models.Lesson, error)
	Delete(ctx context.Context, id string) error
	Cancel(ctx context.Context, id string) error
	ReassignToRule(ctx context.Context, lessonID, ruleID string, occurrenceDate time.Time) error
	DeleteFutureByRule(ctx context.Context, ruleID string, after time.Time) error
	DeleteByCourse(ctx context.Context, courseID string, tutorID string) error
	DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string, toDate *string) error
	UpdateSeries(ctx context.Context, seriesID string, tutorID string, req models.UpdateSeriesRequest) error
	GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error)
	GetAllLessonsForCycles(ctx context.Context, tutorID string) ([]models.CalendarLesson, error)
	GetByPeriod(ctx context.Context, courseID string, tutorID string, from string, to string) ([]models.Lesson, error)
	AutoComplete(ctx context.Context) (int64, error)
	StartRoom(ctx context.Context, lessonID string, tutorID string) error
	EndRoom(ctx context.Context, lessonID string, tutorID string) error
	EndRoomByID(ctx context.Context, lessonID string) error
	GetRoomStatus(ctx context.Context, lessonID string) (string, error)
	GetRanksForCourses(ctx context.Context, courseIDs []string) (map[string]map[string]int, error)
}

type lessonRepository struct {
	pool *pgxpool.Pool
}

func NewLessonRepository(pool *pgxpool.Pool) LessonRepository {
	return &lessonRepository{pool: pool}
}

func (r *lessonRepository) Create(ctx context.Context, req models.CreateLessonRequest) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(ctx,
		`INSERT INTO lessons (course_id, scheduled_at, duration_minutes, notes, status, rule_id, occurrence_date)
		 VALUES ($1, $2, $3, $4,
		         CASE WHEN $2::timestamptz + $3::integer * interval '1 minute' < NOW() THEN 'completed' ELSE 'scheduled' END,
		         NULLIF($5, '')::uuid, $6::date)
		 RETURNING id, course_id, scheduled_at, duration_minutes, status, notes, series_id`,
		req.CourseID, req.ScheduledAt, req.DurationMinutes, req.Notes, req.RuleID, req.OccurrenceDate,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes, &lesson.SeriesID)
	return lesson, err
}

func (r *lessonRepository) CreateBulk(ctx context.Context, req models.CreateBulkLessonRequest) ([]models.Lesson, error) {
	seriesID := uuid.New().String()

	batch := &pgx.Batch{}
	for _, sa := range req.ScheduledAts {
		batch.Queue(
			`INSERT INTO lessons (course_id, scheduled_at, duration_minutes, notes, series_id, status)
			 VALUES ($1, $2, $3, $4, $5,
			         CASE WHEN $2::timestamptz + $3::integer * interval '1 minute' < NOW() THEN 'completed' ELSE 'scheduled' END)
			 RETURNING id, course_id, scheduled_at, duration_minutes, status, notes, series_id`,
			req.CourseID, sa, req.DurationMinutes, req.Notes, seriesID,
		)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	var lessons []models.Lesson
	for range req.ScheduledAts {
		var l models.Lesson
		if err := br.QueryRow().Scan(&l.ID, &l.CourseID, &l.ScheduledAt, &l.DurationMinutes, &l.Status, &l.Notes, &l.SeriesID); err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}
	return lessons, nil
}

func (r *lessonRepository) GetByCourse(ctx context.Context, courseID string) ([]models.Lesson, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, course_id, scheduled_at, duration_minutes, status, notes, series_id
		 FROM lessons WHERE course_id = $1 ORDER BY scheduled_at`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lessons := []models.Lesson{}
	for rows.Next() {
		var lesson models.Lesson
		if err := rows.Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes, &lesson.SeriesID); err != nil {
			return nil, err
		}
		lessons = append(lessons, lesson)
	}
	return lessons, rows.Err()
}

func (r *lessonRepository) GetByCoursePaged(ctx context.Context, courseID string, p models.Pagination) ([]models.Lesson, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lessons WHERE course_id = $1`, courseID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, course_id, scheduled_at, duration_minutes, status, notes, series_id
		 FROM lessons WHERE course_id = $1
		 ORDER BY scheduled_at DESC
		 LIMIT $2 OFFSET $3`,
		courseID, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	lessons := []models.Lesson{}
	for rows.Next() {
		var l models.Lesson
		if err := rows.Scan(&l.ID, &l.CourseID, &l.ScheduledAt, &l.DurationMinutes, &l.Status, &l.Notes, &l.SeriesID); err != nil {
			return nil, 0, err
		}
		lessons = append(lessons, l)
	}
	return lessons, total, rows.Err()
}

func (r *lessonRepository) GetByID(ctx context.Context, id string) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(ctx,
		`SELECT id, course_id, scheduled_at, duration_minutes, status, notes, series_id
		 FROM lessons WHERE id = $1`, id,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes, &lesson.SeriesID)
	return lesson, err
}

// Cancel вместо Delete для вхождения серии: удали строку целиком — и ночная
// материализация создаст урок заново, потому что дата снова свободна.
// is_override заодно защищает отмену от «изменить все следующие».
func (r *lessonRepository) Cancel(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lessons SET status = 'cancelled', is_override = TRUE WHERE id = $1`, id)
	return err
}

// ReassignToRule переводит вхождение в другое правило — используется при
// «это и все следующие», где урок становится первым вхождением новой ветки.
func (r *lessonRepository) ReassignToRule(ctx context.Context, lessonID, ruleID string, occurrenceDate time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lessons SET rule_id = $2::uuid, occurrence_date = $3::date, is_override = FALSE
		 WHERE id = $1`, lessonID, ruleID, occurrenceDate)
	return err
}

// DeleteFutureByRule убирает будущие вхождения правила, кроме вручную
// перенесённых: is_override — это то, что не даёт «изменить все следующие»
// затереть урок, который репетитор уже подвинул руками.
func (r *lessonRepository) DeleteFutureByRule(ctx context.Context, ruleID string, after time.Time) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM lessons
		 WHERE rule_id = $1::uuid AND occurrence_date > $2::date
		   AND is_override = FALSE AND status = 'scheduled'`, ruleID, after)
	return err
}

func (r *lessonRepository) GetByIDForTutor(ctx context.Context, id string, tutorID string) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(ctx,
		`SELECT l.id, l.course_id, l.scheduled_at, l.duration_minutes, l.status, l.notes, l.series_id,
		        l.rule_id, l.occurrence_date
		 FROM lessons l
		 JOIN courses c ON c.id = l.course_id
		 WHERE l.id = $1 AND c.tutor_id = $2`, id, tutorID,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes, &lesson.SeriesID,
		&lesson.RuleID, &lesson.OccurrenceDate)
	return lesson, err
}

func (r *lessonRepository) Update(ctx context.Context, id string, req models.UpdateLessonRequest) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(ctx,
		`UPDATE lessons SET scheduled_at=$1, duration_minutes=$2, status=$3, notes=$4
		 WHERE id=$5
		 RETURNING id, course_id, scheduled_at, duration_minutes, status, notes, series_id`,
		req.ScheduledAt, req.DurationMinutes, req.Status, req.Notes, id,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes, &lesson.SeriesID)
	return lesson, err
}

func (r *lessonRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM lessons WHERE id = $1`, id)
	return err
}

func (r *lessonRepository) DeleteByCourse(ctx context.Context, courseID string, tutorID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM lessons
		 USING courses
		 WHERE lessons.course_id = $1
		   AND lessons.course_id = courses.id
		   AND courses.tutor_id = $2`,
		courseID, tutorID)
	return err
}

func (r *lessonRepository) DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string, toDate *string) error {
	args := []interface{}{seriesID, tutorID}
	fromClause := ""
	toClause := ""
	if fromDate != nil {
		fromClause = fmt.Sprintf("AND lessons.scheduled_at >= $%d::timestamptz", len(args)+1)
		args = append(args, *fromDate)
	}
	if toDate != nil {
		toClause = fmt.Sprintf("AND lessons.scheduled_at <= $%d::timestamptz", len(args)+1)
		args = append(args, *toDate)
	}

	query := fmt.Sprintf(`
		DELETE FROM lessons
		USING courses
		WHERE lessons.series_id = $1
		  AND lessons.course_id = courses.id
		  AND courses.tutor_id = $2
		  %s
		  %s`, fromClause, toClause)

	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

func (r *lessonRepository) UpdateSeries(ctx context.Context, seriesID string, tutorID string, req models.UpdateSeriesRequest) error {
	setParts := []string{}
	args := []interface{}{seriesID, tutorID}

	if req.NewTime != nil {
		idx := len(args) + 1
		setParts = append(setParts, fmt.Sprintf("scheduled_at = date_trunc('day', lessons.scheduled_at) + $%d::interval", idx))
		args = append(args, *req.NewTime)
	}
	if req.DurationMinutes != nil {
		idx := len(args) + 1
		setParts = append(setParts, fmt.Sprintf("duration_minutes = $%d", idx))
		args = append(args, *req.DurationMinutes)
	}
	if req.Notes != nil {
		idx := len(args) + 1
		setParts = append(setParts, fmt.Sprintf("notes = $%d", idx))
		args = append(args, *req.Notes)
	}

	if len(setParts) == 0 {
		return nil
	}

	fromClause := ""
	if req.FromDate != nil {
		idx := len(args) + 1
		fromClause = fmt.Sprintf("AND lessons.scheduled_at >= $%d::timestamptz", idx)
		args = append(args, *req.FromDate)
	}

	query := fmt.Sprintf(`
		UPDATE lessons
		SET %s
		FROM courses
		WHERE lessons.series_id = $1
		  AND lessons.course_id = courses.id
		  AND courses.tutor_id = $2
		  %s`,
		strings.Join(setParts, ", "), fromClause)

	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

func (r *lessonRepository) GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error) {
	rows, err := r.pool.Query(ctx,
		// ranked CTE computes global lesson rank inside the DB — eliminates a separate GetRanksForCourses round-trip.
		// cal_courses is materialized so the subquery runs once and is reused by ranked.
		`WITH cal_courses AS MATERIALIZED (
		   SELECT DISTINCT l.course_id
		   FROM lessons l
		   JOIN courses c ON c.id = l.course_id
		   WHERE c.tutor_id = $1
		     AND l.scheduled_at >= $2::timestamptz
		     AND l.scheduled_at < $3::timestamptz
		 ),
		 ranked AS (
		   SELECT l.id,
		          ROW_NUMBER() OVER (PARTITION BY l.course_id ORDER BY l.scheduled_at)::int AS rank
		   FROM lessons l
		   WHERE l.course_id IN (SELECT course_id FROM cal_courses)
		     AND l.status != 'cancelled'
		 )
		 SELECT l.id, l.course_id, l.scheduled_at, l.duration_minutes, l.status, l.notes,
		        c.subject,
		        CASE WHEN c.student_id IS NOT NULL
		             THEN CASE WHEN s.last_name = '' THEN s.first_name ELSE s.first_name || ' ' || s.last_name END
		             ELSE NULL
		        END AS student_name,
		        (c.student_id IS NULL) AS is_group,
		        l.series_id,
		        r.rank
		 FROM lessons l
		 JOIN courses c ON c.id = l.course_id
		 LEFT JOIN students s ON s.id = c.student_id
		 LEFT JOIN ranked r ON r.id = l.id
		 WHERE c.tutor_id = $1
		   AND l.scheduled_at >= $2::timestamptz
		   AND l.scheduled_at < $3::timestamptz
		 ORDER BY l.scheduled_at`,
		tutorID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []models.CalendarLesson
	for rows.Next() {
		var cl models.CalendarLesson
		if err := rows.Scan(&cl.ID, &cl.CourseID, &cl.ScheduledAt, &cl.DurationMinutes,
			&cl.Status, &cl.Notes, &cl.Subject, &cl.StudentName, &cl.IsGroup, &cl.SeriesID, &cl.Rank); err != nil {
			return nil, err
		}
		lessons = append(lessons, cl)
	}
	return lessons, rows.Err()
}

func (r *lessonRepository) GetByPeriod(ctx context.Context, courseID string, tutorID string, from string, to string) ([]models.Lesson, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT l.id, l.course_id, l.scheduled_at, l.duration_minutes, l.status, l.notes, l.series_id
		 FROM lessons l
		 JOIN courses c ON c.id = l.course_id
		 WHERE l.course_id = $1
		   AND c.tutor_id = $2
		   AND l.scheduled_at >= $3::timestamptz
		   AND l.scheduled_at < $4::timestamptz
		 ORDER BY l.scheduled_at ASC`,
		courseID, tutorID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lessons := []models.Lesson{}
	for rows.Next() {
		var l models.Lesson
		if err := rows.Scan(&l.ID, &l.CourseID, &l.ScheduledAt, &l.DurationMinutes, &l.Status, &l.Notes, &l.SeriesID); err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}
	return lessons, rows.Err()
}

func (r *lessonRepository) AutoComplete(ctx context.Context) (int64, error) {
	result, err := r.pool.Exec(ctx,
		`UPDATE lessons SET status = 'completed'
		 WHERE status = 'scheduled'
		   AND scheduled_at + duration_minutes * interval '1 minute' < NOW()`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r *lessonRepository) StartRoom(ctx context.Context, lessonID string, tutorID string) error {
	result, err := r.pool.Exec(ctx,
		`UPDATE lessons
		SET room_started_at = NOW(), room_ended_at = NULL
		FROM courses
		WHERE lessons.id = $1
		  AND lessons.course_id = courses.id
		  AND courses.tutor_id = $2`,
		lessonID, tutorID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("lesson not found")
	}
	return nil
}

func (r *lessonRepository) EndRoom(ctx context.Context, lessonID string, tutorID string) error {
	result, err := r.pool.Exec(ctx,
		`UPDATE lessons
		SET room_ended_at = NOW()
		FROM courses
		WHERE lessons.id = $1
		  AND lessons.course_id = courses.id
		  AND courses.tutor_id = $2`,
		lessonID, tutorID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("lesson not found")
	}
	return nil
}

// EndRoomByID marks a room ended without a tutor scope — used by the LiveKit
// webhook, which is authenticated by signature, not by a logged-in tutor.
// Idempotent: only the first call (room started, not yet ended) writes; a
// re-delivered webhook or an already-ended room is a no-op, not an error.
func (r *lessonRepository) EndRoomByID(ctx context.Context, lessonID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lessons
		SET room_ended_at = NOW()
		WHERE id = $1
		  AND room_started_at IS NOT NULL
		  AND room_ended_at IS NULL`,
		lessonID)
	return err
}

func (r *lessonRepository) GetRoomStatus(ctx context.Context, lessonID string) (string, error) {
	var startedAt, endedAt *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT room_started_at, room_ended_at FROM lessons WHERE id = $1`, lessonID).Scan(&startedAt, &endedAt)
	if err != nil {
		return "", err
	}
	if endedAt != nil {
		return "ended", nil
	}
	if startedAt != nil {
		return "active", nil
	}
	return "waiting", nil
}

func (r *lessonRepository) GetAllLessonsForCycles(ctx context.Context, tutorID string) ([]models.CalendarLesson, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT l.id, l.course_id, l.scheduled_at, l.status,
		        c.subject,
		        CASE WHEN c.student_id IS NOT NULL
		             THEN CASE WHEN s.last_name = '' THEN s.first_name ELSE s.first_name || ' ' || s.last_name END
		             ELSE NULL
		        END AS student_name,
		        (c.student_id IS NULL) AS is_group,
		        ROW_NUMBER() OVER (PARTITION BY l.course_id ORDER BY l.scheduled_at)::int AS rank
		 FROM lessons l
		 JOIN courses c ON c.id = l.course_id
		 LEFT JOIN students s ON s.id = c.student_id
		 WHERE c.tutor_id = $1
		   AND l.status != 'cancelled'
		 ORDER BY l.course_id, l.scheduled_at`,
		tutorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []models.CalendarLesson
	for rows.Next() {
		var cl models.CalendarLesson
		if err := rows.Scan(
			&cl.ID, &cl.CourseID, &cl.ScheduledAt, &cl.Status,
			&cl.Subject, &cl.StudentName, &cl.IsGroup, &cl.Rank,
		); err != nil {
			return nil, err
		}
		lessons = append(lessons, cl)
	}
	return lessons, rows.Err()
}

func (r *lessonRepository) GetRanksForCourses(ctx context.Context, courseIDs []string) (map[string]map[string]int, error) {
	if len(courseIDs) == 0 {
		return map[string]map[string]int{}, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, course_id,
		        ROW_NUMBER() OVER (PARTITION BY course_id ORDER BY scheduled_at)::int AS rank
		 FROM lessons
		 WHERE course_id = ANY($1)
		   AND status != 'cancelled'`,
		courseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]map[string]int{}
	for rows.Next() {
		var lessonID, courseID string
		var rank int
		if err := rows.Scan(&lessonID, &courseID, &rank); err != nil {
			return nil, err
		}
		if result[courseID] == nil {
			result[courseID] = map[string]int{}
		}
		result[courseID][lessonID] = rank
	}
	return result, rows.Err()
}
