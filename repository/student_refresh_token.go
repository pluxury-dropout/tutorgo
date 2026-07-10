package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StudentRefreshTokenRepository interface {
	Create(ctx context.Context, studentID, token string, expiresAt time.Time) error
	GetByToken(ctx context.Context, token string) (studentID string, expiresAt time.Time, err error)
	DeleteByToken(ctx context.Context, token string) error
}

type studentRefreshTokenRepository struct{ conn *pgxpool.Pool }

func NewStudentRefreshTokenRepository(conn *pgxpool.Pool) StudentRefreshTokenRepository {
	return &studentRefreshTokenRepository{conn: conn}
}

func (r *studentRefreshTokenRepository) Create(ctx context.Context, studentID, token string, expiresAt time.Time) error {
	_, err := r.conn.Exec(ctx,
		`INSERT INTO student_refresh_tokens (student_id, token, expires_at) VALUES ($1,$2,$3)`,
		studentID, token, expiresAt)
	return err
}

func (r *studentRefreshTokenRepository) GetByToken(ctx context.Context, token string) (string, time.Time, error) {
	var id string
	var exp time.Time
	err := r.conn.QueryRow(ctx,
		`SELECT student_id, expires_at FROM student_refresh_tokens WHERE token=$1`, token,
	).Scan(&id, &exp)
	return id, exp, err
}

func (r *studentRefreshTokenRepository) DeleteByToken(ctx context.Context, token string) error {
	_, err := r.conn.Exec(ctx, `DELETE FROM student_refresh_tokens WHERE token=$1`, token)
	return err
}
