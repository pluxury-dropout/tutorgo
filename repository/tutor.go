package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TutorRepository interface {
	Create(req models.CreateTutorRequest, passwordHash string) (models.Tutor, error)
	GetAll() ([]models.Tutor, error)
	GetByID(id string) (models.Tutor, error)
	GetByEmail(email string) (string, string, error)
	GetByPhone(phone string) (string, string, error)
	Update(id string, req models.UpdateTutorRequest) (models.Tutor, error)
	Delete(id string) error
}
type tutorRepository struct {
	conn *pgxpool.Pool
}

func NewTutorRepository(conn *pgxpool.Pool) TutorRepository {
	return &tutorRepository{conn: conn}
}
func (r *tutorRepository) Create(req models.CreateTutorRequest, passwordHash string) (models.Tutor, error) {
	var tutor models.Tutor
	err := r.conn.QueryRow(context.Background(),
		`INSERT INTO tutors (email, password_hash, first_name, last_name, phone)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, email, first_name, last_name, phone`,
		req.Email, passwordHash, req.FirstName, req.LastName, req.Phone,
	).Scan(&tutor.ID, &tutor.Email, &tutor.FirstName, &tutor.LastName, &tutor.Phone)
	return tutor, err
}

func (r *tutorRepository) GetAll() ([]models.Tutor, error) {
	rows, err := r.conn.Query(context.Background(),
		`SELECT id, email, first_name, last_name, phone FROM tutors`)
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

func (r *tutorRepository) GetByID(id string) (models.Tutor, error) {
	var tutor models.Tutor
	err := r.conn.QueryRow(context.Background(),
		`SELECT id, email, first_name, last_name, phone FROM tutors WHERE id = $1`, id,
	).Scan(&tutor.ID, &tutor.Email, &tutor.FirstName, &tutor.LastName, &tutor.Phone)
	return tutor, err
}

func (r *tutorRepository) GetByEmail(email string) (string, string, error) {
	var id, passwordHash string
	err := r.conn.QueryRow(context.Background(),
		`SELECT id, password_hash FROM tutors WHERE email = $1`, email,
	).Scan(&id, &passwordHash)
	return id, passwordHash, err
}

func (r *tutorRepository) GetByPhone(phone string) (string, string, error) {
	var id, passwordHash string
	err := r.conn.QueryRow(context.Background(),
		`SELECT id, password_hash FROM tutors WHERE phone = $1`, phone,
	).Scan(&id, &passwordHash)
	return id, passwordHash, err
}

func (r *tutorRepository) Update(id string, req models.UpdateTutorRequest) (models.Tutor, error) {
	var tutor models.Tutor
	err := r.conn.QueryRow(context.Background(),
		`UPDATE tutors SET email=$1, first_name=$2, last_name=$3, phone=$4
		 WHERE id=$5
		 RETURNING id, email, first_name, last_name, phone`,
		req.Email, req.FirstName, req.LastName, req.Phone, id,
	).Scan(&tutor.ID, &tutor.Email, &tutor.FirstName, &tutor.LastName, &tutor.Phone)
	return tutor, err
}

func (r *tutorRepository) Delete(id string) error {
	_, err := r.conn.Exec(context.Background(),
		`DELETE FROM tutors WHERE id = $1`, id)
	return err
}
