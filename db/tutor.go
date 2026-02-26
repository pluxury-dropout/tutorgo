package db

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5"
)

func CreateTutor(conn *pgx.Conn, req models.CreateTutorRequest) (models.Tutor, error) {
	var tutor models.Tutor
	err := conn.QueryRow(context.Background(),
		`INSERT INTO tutors (email, password_hash, first_name, last_name, phone)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, first_name, last_name, phone`,
		req.Email, req.Password, req.FirstName, req.LastName, req.Phone).Scan(
		&tutor.ID, &tutor.Email, &tutor.FirstName, &tutor.LastName, &tutor.Phone)

	return tutor, err
}

func GetTutors(conn *pgx.Conn) ([]models.Tutor, error) {
	rows, err := conn.Query(context.Background(), `SELECT id, email, first_name, last_name, phone FROM tutors`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tutors []models.Tutor
	for rows.Next() {
		var tutor models.Tutor
		err := rows.Scan(&tutor.ID, &tutor.Email, &tutor.FirstName, &tutor.LastName, &tutor.Phone)
		if err != nil {
			return nil, err
		}
		tutors = append(tutors, tutor)
	}

	return tutors, nil
}

func GetTutorByID(conn *pgx.Conn, id string) (models.Tutor, error) {
	var tutor models.Tutor
	err := conn.QueryRow(context.Background(),
		`SELECT id, email, first_name, last_name, phone FROM tutors WHERE id=$1`, id,
	).Scan(&tutor.ID, &tutor.Email, &tutor.FirstName, &tutor.LastName, &tutor.Phone)
	return tutor, err
}

func DeleteTutor(conn *pgx.Conn, id string) error {
	_, err := conn.Exec(context.Background(),
		`DELETE FROM tutors where id=$1`, id)
	return err
}
