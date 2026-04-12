package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LessonRepository interface {
	Create(req models.CreateLessonRequest) (models.Lesson, error)
	GetByCourse(courseID string) ([]models.Lesson, error)
	GetByID(id string) (models.Lesson, error)
	GetByIDForTutor(id string, tutorID string) (models.Lesson, error)
	Update(id string, req models.UpdateLessonRequest) (models.Lesson, error)
	Delete(id string) error
}

type lessonRepository struct {
	pool *pgxpool.Pool
}

func NewLessonRepository(pool *pgxpool.Pool) LessonRepository {
	return &lessonRepository{pool: pool}
}

func (r *lessonRepository) Create(req models.CreateLessonRequest) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO lessons (course_id, scheduled_at, duration_minutes, notes)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, course_id, scheduled_at, duration_minutes, status, notes`,
		req.CourseID, req.ScheduledAt, req.DurationMinutes, req.Notes,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes)
	return lesson, err
}

func (r *lessonRepository) GetByCourse(courseID string) ([]models.Lesson, error) {
	rows, err := r.pool.Query(context.Background(),
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
	return lessons, nil
}

func (r *lessonRepository) GetByID(id string) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(context.Background(),
		`SELECT id, course_id, scheduled_at, duration_minutes, status, notes
		 FROM lessons WHERE id = $1`, id,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes)
	return lesson, err
}

func (r *lessonRepository) GetByIDForTutor(id string, tutorID string) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(context.Background(),
		`SELECT l.id, l.course_id, l.scheduled_at, l.duration_minutes, l.status, l.notes
		 FROM lessons l
		 JOIN courses с ON c.id = l.course_id
		 WHERE l.id = $1 AND c.tutor_id = $2`, id, tutorID,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes)
	return lesson, err
}

func (r *lessonRepository) Update(id string, req models.UpdateLessonRequest) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(context.Background(),
		`UPDATE lessons SET scheduled_at=$1, duration_minutes=$2, status=$3, notes=$4
		 WHERE id=$5
		 RETURNING id, course_id, scheduled_at, duration_minutes, status, notes`,
		req.ScheduledAt, req.DurationMinutes, req.Status, req.Notes, id,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes)
	return lesson, err
}

func (r *lessonRepository) Delete(id string) error {
	_, err := r.pool.Exec(context.Background(),
		`DELETE FROM lessons WHERE id = $1`, id)
	return err
}
