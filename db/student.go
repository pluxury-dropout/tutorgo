package db

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5"
)

func CreateStudent(conn *pgx.Conn, req models.CreateStudentRequest, tutorID string) (models.Student, error) {
	var student models.Student
	err := conn.QueryRow(context.Background(),
		`INSERT INTO students (tutor_id, first_name, last_name, phone, email, notes)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, tutor_id, first_name, last_name, phone, email, notes, active`,
		tutorID, req.FirstName, req.LastName, req.Phone, req.Email, req.Notes,
	).Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)
	return student, err
}
