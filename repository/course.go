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
	GetAllArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error)
	Restore(ctx context.Context, id string, tutorID string) error
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
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active`,
		req.StudentID, tutorID, req.Subject, req.PricePerCycle, req.LessonsPerCycle, req.StartedAt, req.EndedAt,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive)
	return course, err
}

func (r *courseRepository) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM courses
		 WHERE tutor_id = $1 AND is_active = TRUE
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')`,
		tutorID, p.Search,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active
		 FROM courses
		 WHERE tutor_id = $1 AND is_active = TRUE
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')
		 ORDER BY started_at DESC
		 LIMIT $3 OFFSET $4`,
		tutorID, p.Search, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	courses := []models.Course{}
	for rows.Next() {
		var course models.Course
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive); err != nil {
			return nil, 0, err
		}
		courses = append(courses, course)
	}
	return courses, total, rows.Err()
}

func (r *courseRepository) GetByID(ctx context.Context, id string, tutorID string) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active
		 FROM courses WHERE id = $1 AND tutor_id = $2`, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive)
	return course, err
}

func (r *courseRepository) GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT c.id, c.student_id, c.tutor_id, c.subject, c.price_per_cycle, c.lessons_per_cycle, c.started_at, c.ended_at, c.is_active
		 FROM courses c
		 WHERE c.tutor_id = $2 AND c.student_id = $1 AND c.is_active = TRUE
		 UNION
		 SELECT c.id, c.student_id, c.tutor_id, c.subject, c.price_per_cycle, c.lessons_per_cycle, c.started_at, c.ended_at, c.is_active
		 FROM courses c
		 JOIN course_enrollments ce ON ce.course_id = c.id
		 WHERE c.tutor_id = $2 AND ce.student_id = $1 AND c.is_active = TRUE
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
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive); err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	return courses, rows.Err()
}

func (r *courseRepository) Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`UPDATE courses SET subject=$1, price_per_cycle=$2, lessons_per_cycle=$3, started_at=$4, ended_at=$5
		 WHERE id=$6 AND tutor_id=$7
		 RETURNING id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active`,
		req.Subject, req.PricePerCycle, req.LessonsPerCycle, req.StartedAt, req.EndedAt, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive)
	return course, err
}

func (r *courseRepository) Delete(ctx context.Context, id string, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE courses SET is_active = FALSE WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}

func (r *courseRepository) GetAllArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM courses
		 WHERE tutor_id = $1 AND is_active = FALSE
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')`,
		tutorID, p.Search,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active
		 FROM courses
		 WHERE tutor_id = $1 AND is_active = FALSE
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')
		 ORDER BY started_at DESC
		 LIMIT $3 OFFSET $4`,
		tutorID, p.Search, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	courses := []models.Course{}
	for rows.Next() {
		var course models.Course
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive); err != nil {
			return nil, 0, err
		}
		courses = append(courses, course)
	}
	return courses, total, rows.Err()
}

func (r *courseRepository) Restore(ctx context.Context, id string, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE courses SET is_active = TRUE WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}
