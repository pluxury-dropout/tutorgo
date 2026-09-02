package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type EnrollmentRepository interface {
	Add(ctx context.Context, courseID string, studentID string) (models.CourseEnrollment, error)
	AddBulk(ctx context.Context, courseID string, studentIDs []string, tutorID string) ([]models.CourseEnrollment, error)
	Remove(ctx context.Context, courseID string, studentID string) error
	GetByCourse(ctx context.Context, courseID string) ([]models.CourseEnrollment, error)
}

type enrollmentRepository struct {
	pool *pgxpool.Pool
}

func NewEnrollmentRepository(pool *pgxpool.Pool) EnrollmentRepository {
	return &enrollmentRepository{pool: pool}
}

func (r *enrollmentRepository) Add(ctx context.Context, courseID string, studentID string) (models.CourseEnrollment, error) {
	var e models.CourseEnrollment
	err := r.pool.QueryRow(ctx,
		`INSERT INTO course_enrollments (course_id, student_id)
		 VALUES ($1, $2)
		 RETURNING id, course_id, student_id`,
		courseID, studentID,
	).Scan(&e.ID, &e.CourseID, &e.StudentID)
	return e, err
}

// AddBulk записывает в группу сразу нескольких учеников. Вставка идёт SELECT'ом
// из students — чужие ученики отсеиваются там же, отдельной проверкой владения
// по одному запросу на каждого. Повторная запись уже состоящего в группе
// молча пропускается: собрать группу дважды — не ошибка пользователя.
func (r *enrollmentRepository) AddBulk(ctx context.Context, courseID string, studentIDs []string, tutorID string) ([]models.CourseEnrollment, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO course_enrollments (course_id, student_id)
		 SELECT $1::uuid, s.id
		 FROM students s
		 WHERE s.tutor_id = $2::uuid AND s.id = ANY($3::uuid[])
		 ON CONFLICT (course_id, student_id) DO NOTHING
		 RETURNING id, course_id, student_id`,
		courseID, tutorID, studentIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	enrollments := []models.CourseEnrollment{}
	for rows.Next() {
		var e models.CourseEnrollment
		if err := rows.Scan(&e.ID, &e.CourseID, &e.StudentID); err != nil {
			return nil, err
		}
		enrollments = append(enrollments, e)
	}
	return enrollments, rows.Err()
}

func (r *enrollmentRepository) Remove(ctx context.Context, courseID string, studentID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM course_enrollments WHERE course_id = $1 AND student_id = $2`,
		courseID, studentID)
	return err
}

func (r *enrollmentRepository) GetByCourse(ctx context.Context, courseID string) ([]models.CourseEnrollment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT ce.id, ce.course_id, ce.student_id, s.first_name, s.last_name
		 FROM course_enrollments ce
		 JOIN students s ON s.id = ce.student_id
		 WHERE ce.course_id = $1`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var enrollments []models.CourseEnrollment
	for rows.Next() {
		var e models.CourseEnrollment
		if err := rows.Scan(&e.ID, &e.CourseID, &e.StudentID, &e.StudentFirstName, &e.StudentLastName); err != nil {
			return nil, err
		}
		enrollments = append(enrollments, e)
	}
	return enrollments, rows.Err()
}
