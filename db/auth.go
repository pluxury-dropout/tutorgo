package db

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5"
)

func RegisterTutor(conn *pgx.Conn, req models.RegisterRequest, passwordHash string) (models.Tutor, error) {
	var tutor models.Tutor
	err := conn.QueryRow(context.Background(),
		`INSERT INTO tutors (email, password_hash, first_name, last_name, phone)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, email, first_name, last_name, phone`,
		req.Email, passwordHash, req.FirstName, req.LastName, req.Phone,
	).Scan(&tutor.ID, &tutor.Email, &tutor.FirstName, &tutor.LastName, &tutor.Phone)
	return tutor, err
}

func GetTutorByEmail(conn *pgx.Conn, email string) (string, string, error) {
	var id, passwordHash string
	err := conn.QueryRow(context.Background(),
		`SELECT id, password_hash FROM tutors WHERE email = $1`, email,
	).Scan(&id, &passwordHash)
	return id, passwordHash, err
}

func GetTutorByPhone(conn *pgx.Conn, phone string) (string, string, error) {
	var id, passwordHash string
	err := conn.QueryRow(context.Background(),
		`SELECT id, password_hash FROM tutors where phone=$1`, phone).Scan(&id, &passwordHash)
	return id, passwordHash, err
}
