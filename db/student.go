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

func GetStudents(conn *pgx.Conn, tutorID string) ([]models.Student, error) {
	rows, err := conn.Query(context.Background(),
		`SELECT id, tutor_id, first_name, last_name, phone, email, notes, active 
	FROM students WHERE tutor_id = $1`, tutorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var students []models.Student
	for rows.Next() {
		var student models.Student
		err := rows.Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)
		if err != nil {
			return nil, err
		}
		students = append(students, student)
	}
	return students, err

}

func GetStudentByID(conn *pgx.Conn, id string, tutorID string) (models.Student, error) {
	var student models.Student
	err := conn.QueryRow(context.Background(),
		`SELECT id, tutor_id, first_name, last_name, phone, email, notes, active
	FROM students WHERE id=%1 AND tutor_id=%2`, id, tutorID,
	).Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)

	return student, err

}

func UpdateStudent(conn *pgx.Conn, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error) {
	var student models.Student
	err := conn.QueryRow(context.Background(),
		`UPDATE students SET first_name=$1, last_name=$2, phone=$3, email=$4, notes=$5
	WHERE id=$6 AND tutor_id=$7
	RETURNING id, tutor_id, first_name, last_name, phone, email, notes, active`,
		req.FirstName, req.LastName, req.Phone, req.Email, req.Notes, id, tutorID,
	).Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)
	return student, err
}

func DeleteStudent(conn *pgx.Conn, id string, tutorID string) error {
	_, err := conn.Exec(context.Background(),
		`DELETE FROM students WHERE id=$1 AND tutor_id=$2`, id, tutorID)
	return err
}
