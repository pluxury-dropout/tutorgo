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
	DeleteByCourse(ctx context.Context, courseID string, tutorID string) error
	DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string) error
	UpdateSeries(ctx context.Context, seriesID string, tutorID string, req models.UpdateSeriesRequest) error
	GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error)
	GetByPeriod(ctx context.Context, courseID string, tutorID string, from string, to string) ([]models.Lesson, error)
	AutoComplete(ctx context.Context) (int64, error)
	ExistsPublic(ctx context.Context, id string) error
	StartRoom(ctx context.Context, lessonID string, tutorID string) error
	EndRoom(ctx context.Context, lessonID string, tutorID string) error
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
		`INSERT INTO lessons (course_id, scheduled_at, duration_minutes, notes, status)
		 VALUES ($1, $2, $3, $4,
		         CASE WHEN $2 + $3 * interval '1 minute' < NOW() THEN 'completed' ELSE 'scheduled' END)
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
			`INSERT INTO lessons (course_id, scheduled_at, duration_minutes, notes, series_id, status)
			 VALUES ($1, $2, $3, $4, $5,
			         CASE WHEN $2 + $3 * interval '1 minute' < NOW() THEN 'completed' ELSE 'scheduled' END)
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

func (r *lessonRepository) ExistsPublic(ctx context.Context, id string) error {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM lessons WHERE id = $1
		AND room_started_at IS NOT NULL)`, id,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("lesson not found")
	}
	return nil
}

func (r *lessonRepository) StartRoom(ctx context.Context, lessonID string, tutorID string) error {
	result, err := r.pool.Exec(ctx,
		`UPDATE lessons
		SET room_started_at = NOW()
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

func (r *lessonRepository) GetRoomStatus(ctx context.Context, lessonID string) (string, error) {
	var startedAt, endedAt *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT room_started_at, room_ended_at FROM lessons WHERE id =$ 1`, lessonID).Scan(&startedAt, endedAt)
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
