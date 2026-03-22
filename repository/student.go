package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StudentRepository interface {
	Create(req models.CreateStudentRequest, tutorID string) (models.Student, error)
	GetAll(tutorID string) ([]models.Student, error)
	GetByID(id string, tutorID string) (models.Student, error)
	Update(id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error)
	Delete(id string, tutorID string) error
}

type studentRepository struct {
	conn *pgxpool.Pool
}

func NewStudentRepository(conn *pgxpool.Pool) StudentRepository {
	return &studentRepository{conn: conn}
}

func (r *studentRepository) Create(req models.CreateStudentRequest, tutorID string) (models.Student, error) {
	var student models.Student
	err := r.conn.QueryRow(context.Background(),
		`INSERT INTO students (tutor_id, first_name, last_name, phone, email, notes)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, tutor_id, first_name, last_name, phone, email, notes, active`,
		tutorID, req.FirstName, req.LastName, req.Phone, req.Email, req.Notes,
	).Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)
	return student, err
}

func (r *studentRepository) GetAll(tutorID string) ([]models.Student, error) {
	rows, err := r.conn.Query(context.Background(),
		`SELECT id, tutor_id, first_name, last_name, phone, email, notes, active
		 FROM students WHERE tutor_id = $1`, tutorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	students := []models.Student{}
	for rows.Next() {
		var student models.Student
		err := rows.Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)
		if err != nil {
			return nil, err
		}
		students = append(students, student)
	}
	return students, nil
}

func (r *studentRepository) GetByID(id string, tutorID string) (models.Student, error) {
	var student models.Student
	err := r.conn.QueryRow(context.Background(),
		`SELECT id, tutor_id, first_name, last_name, phone, email, notes, active
		 FROM students WHERE id = $1 AND tutor_id = $2`, id, tutorID,
	).Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)
	return student, err
}

func (r *studentRepository) Update(id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error) {
	var student models.Student
	err := r.conn.QueryRow(context.Background(),
		`UPDATE students SET first_name=$1, last_name=$2, phone=$3, email=$4, notes=$5
		 WHERE id=$6 AND tutor_id=$7
		 RETURNING id, tutor_id, first_name, last_name, phone, email, notes, active`,
		req.FirstName, req.LastName, req.Phone, req.Email, req.Notes, id, tutorID,
	).Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)
	return student, err
}

func (r *studentRepository) Delete(id string, tutorID string) error {
	_, err := r.conn.Exec(context.Background(),
		`DELETE FROM students WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}
