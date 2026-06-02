package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshTokenRepository interface {
	Create(ctx context.Context, tutorID, token string, expiresAt time.Time) error
	GetByToken(ctx context.Context, token string) (tutorID string, expiresAt time.Time, err error)
	DeleteByToken(ctx context.Context, token string) error
	DeleteAllByTutorID(ctx context.Context, tutorID string) error
}

type refreshTokenRepository struct {
	conn *pgxpool.Pool
}

func NewRefreshTokenRepository(conn *pgxpool.Pool) RefreshTokenRepository {
	return &refreshTokenRepository{conn: conn}
}

func (r *refreshTokenRepository) Create(ctx context.Context, tutorID, token string, expiresAt time.Time) error {
	_, err := r.conn.Exec(ctx,
		`INSERT INTO refresh_tokens (tutor_id, token, expires_at) VALUES ($1, $2, $3)`,
		tutorID, token, expiresAt,
	)
	return err
}

func (r *refreshTokenRepository) GetByToken(ctx context.Context, token string) (string, time.Time, error) {
	var tutorID string
	var expiresAt time.Time
	err := r.conn.QueryRow(ctx,
		`SELECT tutor_id, expires_at FROM refresh_tokens WHERE token = $1`,
		token,
	).Scan(&tutorID, &expiresAt)
	return tutorID, expiresAt, err
}

func (r *refreshTokenRepository) DeleteByToken(ctx context.Context, token string) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE token = $1`, token)
	return err
}

func (r *refreshTokenRepository) DeleteAllByTutorID(ctx context.Context, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE tutor_id = $1`, tutorID)
	return err
}
