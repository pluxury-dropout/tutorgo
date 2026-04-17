package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LessonRepository interface {
	Create(ctx context.Context, req models.CreateLessonRequest) (models.Lesson, error)
	GetByCourse(ctx context.Context, courseID string) ([]models.Lesson, error)
	GetByID(ctx context.Context, id string) (models.Lesson, error)
	GetByIDForTutor(ctx context.Context, id string, tutorID string) (models.Lesson, error)
	Update(ctx context.Context, id string, req models.UpdateLessonRequest) (models.Lesson, error)
	Delete(ctx context.Context, id string) error
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
		 RETURNING id, course_id, scheduled_at, duration_minutes, status, notes`,
		req.CourseID, req.ScheduledAt, req.DurationMinutes, req.Notes,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes)
	return lesson, err
}

func (r *lessonRepository) GetByCourse(ctx context.Context, courseID string) ([]models.Lesson, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, course_id, scheduled_at, duration_minutes, status, notes
		 FROM lessons WHERE course_id = $1 ORDER BY scheduled_at`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lessons := []models.Lesson{}
	for rows.Next() {
		var lesson models.Lesson
		err := rows.Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes)
		if err != nil {
			return nil, err
		}
		lessons = append(lessons, lesson)
	}
	return lessons, rows.Err()
}

func (r *lessonRepository) GetByID(ctx context.Context, id string) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(ctx,
		`SELECT id, course_id, scheduled_at, duration_minutes, status, notes
		 FROM lessons WHERE id = $1`, id,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes)
	return lesson, err
}

func (r *lessonRepository) GetByIDForTutor(ctx context.Context, id string, tutorID string) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(ctx,
		`SELECT l.id, l.course_id, l.scheduled_at, l.duration_minutes, l.status, l.notes
		 FROM lessons l
		 JOIN courses c ON c.id = l.course_id
		 WHERE l.id = $1 AND c.tutor_id = $2`, id, tutorID,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes)
	return lesson, err
}

func (r *lessonRepository) Update(ctx context.Context, id string, req models.UpdateLessonRequest) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(ctx,
		`UPDATE lessons SET scheduled_at=$1, duration_minutes=$2, status=$3, notes=$4
		 WHERE id=$5
		 RETURNING id, course_id, scheduled_at, duration_minutes, status, notes`,
		req.ScheduledAt, req.DurationMinutes, req.Status, req.Notes, id,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes)
	return lesson, err
}

func (r *lessonRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM lessons WHERE id = $1`, id)
	return err
}

func (r *lessonRepository) GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT l.id, l.course_id, l.scheduled_at, l.duration_minutes, l.status, l.notes,
		        c.subject,
		        CASE WHEN c.student_id IS NOT NULL
		             THEN s.first_name || ' ' || s.last_name
		             ELSE NULL
		        END AS student_name,
		        (c.student_id IS NULL) AS is_group
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
			&cl.Status, &cl.Notes, &cl.Subject, &cl.StudentName, &cl.IsGroup); err != nil {
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
		   AND scheduled_at + (duration_minutes || ' minutes')::interval < NOW()`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
