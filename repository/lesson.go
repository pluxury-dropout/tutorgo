package repository

import (
	"context"
	"fmt"
	"strings"
	"tutorgo/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LessonRepository interface {
	Create(ctx context.Context, req models.CreateLessonRequest) (models.Lesson, error)
	CreateBulk(ctx context.Context, req models.CreateBulkLessonRequest) ([]models.Lesson, error)
	GetByCourse(ctx context.Context, courseID string) ([]models.Lesson, error)
	GetByID(ctx context.Context, id string) (models.Lesson, error)
	GetByIDForTutor(ctx context.Context, id string, tutorID string) (models.Lesson, error)
	Update(ctx context.Context, id string, req models.UpdateLessonRequest) (models.Lesson, error)
	Delete(ctx context.Context, id string) error
	DeleteByCourse(ctx context.Context, courseID string, tutorID string) error
	DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string) error
	UpdateSeries(ctx context.Context, seriesID string, tutorID string, req models.UpdateSeriesRequest) error
	GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error)
	AutoComplete(ctx context.Context) (int64, error)
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
		`INSERT INTO lessons (course_id, scheduled_at, duration_minutes, notes)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, course_id, scheduled_at, duration_minutes, status, notes, series_id`,
		req.CourseID, req.ScheduledAt, req.DurationMinutes, req.Notes,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes, &lesson.SeriesID)
	return lesson, err
}

func (r *lessonRepository) CreateBulk(ctx context.Context, req models.CreateBulkLessonRequest) ([]models.Lesson, error) {
	seriesID := uuid.New().String()

	batch := &pgx.Batch{}
	for _, sa := range req.ScheduledAts {
		batch.Queue(
			`INSERT INTO lessons (course_id, scheduled_at, duration_minutes, notes, series_id)
			 VALUES ($1, $2, $3, $4, $5)
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

func (r *lessonRepository) GetByID(ctx context.Context, id string) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(ctx,
		`SELECT id, course_id, scheduled_at, duration_minutes, status, notes, series_id
		 FROM lessons WHERE id = $1`, id,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes, &lesson.SeriesID)
	return lesson, err
}

func (r *lessonRepository) GetByIDForTutor(ctx context.Context, id string, tutorID string) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(ctx,
		`SELECT l.id, l.course_id, l.scheduled_at, l.duration_minutes, l.status, l.notes, l.series_id
		 FROM lessons l
		 JOIN courses c ON c.id = l.course_id
		 WHERE l.id = $1 AND c.tutor_id = $2`, id, tutorID,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes, &lesson.SeriesID)
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

func (r *lessonRepository) DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string) error {
	args := []interface{}{seriesID, tutorID}
	fromClause := ""
	if fromDate != nil {
		fromClause = fmt.Sprintf("AND lessons.scheduled_at >= $%d::timestamptz", len(args)+1)
		args = append(args, *fromDate)
	}

	query := fmt.Sprintf(`
		DELETE FROM lessons
		USING courses
		WHERE lessons.series_id = $1
		  AND lessons.course_id = courses.id
		  AND courses.tutor_id = $2
		  %s`, fromClause)

	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

func (r *lessonRepository) UpdateSeries(ctx context.Context, seriesID string, tutorID string, req models.UpdateSeriesRequest) error {
	setParts := []string{}
	args := []interface{}{seriesID, tutorID}

	if req.NewTime != nil {
		idx := len(args) + 1
		setParts = append(setParts, fmt.Sprintf("scheduled_at = date_trunc('day', lessons.scheduled_at) + $%d::time", idx))
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
		`SELECT l.id, l.course_id, l.scheduled_at, l.duration_minutes, l.status, l.notes,
		        c.subject,
		        CASE WHEN c.student_id IS NOT NULL
		             THEN CASE WHEN s.last_name = '' THEN s.first_name ELSE s.first_name || ' ' || s.last_name END
		             ELSE NULL
		        END AS student_name,
		        (c.student_id IS NULL) AS is_group,
		        l.series_id
		 FROM lessons l
		 JOIN courses c ON c.id = l.course_id
		 LEFT JOIN students s ON s.id = c.student_id
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
			&cl.Status, &cl.Notes, &cl.Subject, &cl.StudentName, &cl.IsGroup, &cl.SeriesID); err != nil {
			return nil, err
		}
		lessons = append(lessons, cl)
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
