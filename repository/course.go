package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CourseRepository interface {
	Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error)
	GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Course, error)
	GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error)
	Delete(ctx context.Context, id string, tutorID string) error
}

type courseRepository struct {
	conn *pgxpool.Pool
}

func NewCourseRepository(conn *pgxpool.Pool) CourseRepository {
	return &courseRepository{conn: conn}
}

func (r *courseRepository) Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_lesson, started_at, ended_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at`,
		req.StudentID, tutorID, req.Subject, req.PricePerLesson, req.StartedAt, req.EndedAt,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt)
	return course, err
}

func (r *courseRepository) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM courses
		 WHERE tutor_id = $1
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')`,
		tutorID, p.Search,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at
		 FROM courses
		 WHERE tutor_id = $1
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')
		 ORDER BY started_at DESC
		 LIMIT $3 OFFSET $4`,
		tutorID, p.Search, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var courses []models.Course
	for rows.Next() {
		var course models.Course
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt); err != nil {
			return nil, 0, err
		}
		courses = append(courses, course)
	}
	return courses, total, rows.Err()
}

func (r *courseRepository) GetByID(ctx context.Context, id string, tutorID string) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at
		 FROM courses WHERE id = $1 AND tutor_id = $2`, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt)
	return course, err
}

func (r *courseRepository) GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at
		 FROM courses
		 WHERE tutor_id = $2 AND student_id = $1
		 UNION
		 SELECT c.id, c.student_id, c.tutor_id, c.subject, c.price_per_lesson, c.started_at, c.ended_at
		 FROM courses c
		 JOIN course_enrollments ce ON ce.course_id = c.id
		 WHERE c.tutor_id = $2 AND ce.student_id = $1
		 ORDER BY started_at DESC`,
		studentID, tutorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var courses []models.Course
	for rows.Next() {
		var course models.Course
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt); err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	return courses, rows.Err()
}

func (r *courseRepository) Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`UPDATE courses SET subject=$1, price_per_lesson=$2, started_at=$3, ended_at=$4
		 WHERE id=$5 AND tutor_id=$6
		 RETURNING id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at`,
		req.Subject, req.PricePerLesson, req.StartedAt, req.EndedAt, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt)
	return course, err
}

func (r *courseRepository) Delete(ctx context.Context, id string, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM courses WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}
