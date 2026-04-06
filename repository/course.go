package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CourseRepository interface {
	Create(req models.CreateCourseRequest, tutorID string) (models.Course, error)
	GetAll(tutorID string) ([]models.Course, error)
	GetByID(id string, tutorID string) (models.Course, error)
	Update(id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error)
	Delete(id string, tutorID string) error
}

type courseRepository struct {
	conn *pgxpool.Pool
}

func NewCourseRepository(conn *pgxpool.Pool) CourseRepository {
	return &courseRepository{conn: conn}
}

func (r *courseRepository) Create(req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(context.Background(),
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_lesson, started_at, ended_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at`,
		req.StudentID, tutorID, req.Subject, req.PricePerLesson, req.StartedAt, req.EndedAt,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt)
	return course, err
}

func (r *courseRepository) GetAll(tutorID string) ([]models.Course, error) {
	rows, err := r.conn.Query(context.Background(),
		`SELECT id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at
		 FROM courses WHERE tutor_id = $1`, tutorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var courses []models.Course
	for rows.Next() {
		var course models.Course
		err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt)
		if err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	return courses, nil
}

func (r *courseRepository) GetByID(id string, tutorID string) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(context.Background(),
		`SELECT id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at
		 FROM courses WHERE id = $1 AND tutor_id = $2`, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt)
	return course, err
}

func (r *courseRepository) Update(id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(context.Background(),
		`UPDATE courses SET subject=$1, price_per_lesson=$2, started_at=$3, ended_at=$4
		 WHERE id=$5 AND tutor_id=$6
		 RETURNING id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at`,
		req.Subject, req.PricePerLesson, req.StartedAt, req.EndedAt, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt)
	return course, err
}

func (r *courseRepository) Delete(id string, tutorID string) error {
	_, err := r.conn.Exec(context.Background(),
		`DELETE FROM courses WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}
