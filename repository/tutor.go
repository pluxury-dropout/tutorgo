package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TutorRepository interface {
	Create(ctx context.Context, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error)
	GetAll(ctx context.Context) ([]models.Tutor, error)
	GetByID(ctx context.Context, id string) (models.Tutor, error)
	GetByEmail(ctx context.Context, email string) (string, string, error)
	GetByPhone(ctx context.Context, phone string) (string, string, error)
	Update(ctx context.Context, id string, req models.UpdateTutorRequest) (models.Tutor, error)
	Delete(ctx context.Context, id string) error
}
type tutorRepository struct {
	conn *pgxpool.Pool
}

func NewTutorRepository(conn *pgxpool.Pool) TutorRepository {
	return &tutorRepository{conn: conn}
}
func (r *tutorRepository) Create(ctx context.Context, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error) {
	var tutor models.Tutor
	err := r.conn.QueryRow(ctx,
		`INSERT INTO tutors (email, password_hash, first_name, last_name, phone)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, email, first_name, last_name, phone`,
		req.Email, passwordHash, req.FirstName, req.LastName, req.Phone,
	).Scan(&tutor.ID, &tutor.Email, &tutor.FirstName, &tutor.LastName, &tutor.Phone)
	return tutor, err
}

func (r *tutorRepository) GetAll(ctx context.Context) ([]models.Tutor, error) {
	rows, err := r.conn.Query(ctx,
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
	return tutors, rows.Err()
}

func (r *tutorRepository) GetByID(ctx context.Context, id string) (models.Tutor, error) {
	var tutor models.Tutor
	err := r.conn.QueryRow(ctx,
		`SELECT id, email, first_name, last_name, phone FROM tutors WHERE id = $1`, id,
	).Scan(&tutor.ID, &tutor.Email, &tutor.FirstName, &tutor.LastName, &tutor.Phone)
	return tutor, err
}

func (r *tutorRepository) GetByEmail(ctx context.Context, email string) (string, string, error) {
	var id, passwordHash string
	err := r.conn.QueryRow(ctx,
		`SELECT id, password_hash FROM tutors WHERE email = $1`, email,
	).Scan(&id, &passwordHash)
	return id, passwordHash, err
}

func (r *tutorRepository) GetByPhone(ctx context.Context, phone string) (string, string, error) {
	var id, passwordHash string
	err := r.conn.QueryRow(ctx,
		`SELECT id, password_hash FROM tutors WHERE phone = $1`, phone,
	).Scan(&id, &passwordHash)
	return id, passwordHash, err
}

func (r *tutorRepository) Update(ctx context.Context, id string, req models.UpdateTutorRequest) (models.Tutor, error) {
	var tutor models.Tutor
	err := r.conn.QueryRow(ctx,
		`UPDATE tutors SET email=$1, first_name=$2, last_name=$3, phone=$4
		 WHERE id=$5
		 RETURNING id, email, first_name, last_name, phone`,
		req.Email, req.FirstName, req.LastName, req.Phone, id,
	).Scan(&tutor.ID, &tutor.Email, &tutor.FirstName, &tutor.LastName, &tutor.Phone)
	return tutor, err
}

func (r *tutorRepository) Delete(ctx context.Context, id string) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM tutors WHERE id = $1`, id)
	return err
}
