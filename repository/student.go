package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StudentRepository interface {
	Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error)
	GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Student, int, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Student, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error)
	Delete(ctx context.Context, id string, tutorID string) error
}

type studentRepository struct {
	conn *pgxpool.Pool
}

func NewStudentRepository(conn *pgxpool.Pool) StudentRepository {
	return &studentRepository{conn: conn}
}

func (r *studentRepository) Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error) {
	var student models.Student
	err := r.conn.QueryRow(ctx,
		`INSERT INTO students (tutor_id, first_name, last_name, phone, email, notes)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, tutor_id, first_name, last_name, phone, email, notes, active`,
		tutorID, req.FirstName, req.LastName, req.Phone, req.Email, req.Notes,
	).Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)
	return student, err
}

func (r *studentRepository) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Student, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM students WHERE tutor_id = $1`, tutorID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, tutor_id, first_name, last_name, phone, email, notes, active
		 FROM students WHERE tutor_id = $1
		 ORDER BY first_name, last_name
		 LIMIT $2 OFFSET $3`,
		tutorID, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	students := []models.Student{}
	for rows.Next() {
		var student models.Student
		if err := rows.Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active); err != nil {
			return nil, 0, err
		}
		students = append(students, student)
	}
	return students, total, rows.Err()
}

func (r *studentRepository) GetByID(ctx context.Context, id string, tutorID string) (models.Student, error) {
	var student models.Student
	err := r.conn.QueryRow(ctx,
		`SELECT id, tutor_id, first_name, last_name, phone, email, notes, active
		 FROM students WHERE id = $1 AND tutor_id = $2`, id, tutorID,
	).Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)
	return student, err
}

func (r *studentRepository) Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error) {
	var student models.Student
	err := r.conn.QueryRow(ctx,
		`UPDATE students SET first_name=$1, last_name=$2, phone=$3, email=$4, notes=$5
		 WHERE id=$6 AND tutor_id=$7
		 RETURNING id, tutor_id, first_name, last_name, phone, email, notes, active`,
		req.FirstName, req.LastName, req.Phone, req.Email, req.Notes, id, tutorID,
	).Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)
	return student, err
}

func (r *studentRepository) Delete(ctx context.Context, id string, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM students WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}
