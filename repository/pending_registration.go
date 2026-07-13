package repository

import (
	"context"

	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PendingRegistrationRepository interface {
	Upsert(ctx context.Context, p models.PendingRegistration) error
	GetByEmail(ctx context.Context, email string) (models.PendingRegistration, error)
	IncrementAttempts(ctx context.Context, email string) error
	Delete(ctx context.Context, email string) error
	DeleteExpired(ctx context.Context) (int64, error)
}

type pendingRegistrationRepository struct {
	conn *pgxpool.Pool
}

func NewPendingRegistrationRepository(conn *pgxpool.Pool) PendingRegistrationRepository {
	return &pendingRegistrationRepository{conn: conn}
}

func (r *pendingRegistrationRepository) Upsert(ctx context.Context, p models.PendingRegistration) error {
	_, err := r.conn.Exec(ctx, `
		INSERT INTO pending_registrations
			(email, password_hash, first_name, last_name, phone, code_hash, attempts, resend_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8)
		ON CONFLICT (email) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			first_name    = EXCLUDED.first_name,
			last_name     = EXCLUDED.last_name,
			phone         = EXCLUDED.phone,
			code_hash     = EXCLUDED.code_hash,
			attempts      = 0,
			resend_at     = EXCLUDED.resend_at,
			expires_at    = EXCLUDED.expires_at`,
		p.Email, p.PasswordHash, p.FirstName, p.LastName, p.Phone, p.CodeHash, p.ResendAt, p.ExpiresAt,
	)
	return err
}

func (r *pendingRegistrationRepository) GetByEmail(ctx context.Context, email string) (models.PendingRegistration, error) {
	var p models.PendingRegistration
	err := r.conn.QueryRow(ctx, `
		SELECT email, password_hash, first_name, last_name, phone, code_hash, attempts, resend_at, expires_at
		FROM pending_registrations WHERE email = $1`, email,
	).Scan(&p.Email, &p.PasswordHash, &p.FirstName, &p.LastName, &p.Phone, &p.CodeHash, &p.Attempts, &p.ResendAt, &p.ExpiresAt)
	return p, err
}

func (r *pendingRegistrationRepository) IncrementAttempts(ctx context.Context, email string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE pending_registrations SET attempts = attempts + 1 WHERE email = $1`, email)
	return err
}

func (r *pendingRegistrationRepository) Delete(ctx context.Context, email string) error {
	_, err := r.conn.Exec(ctx, `DELETE FROM pending_registrations WHERE email = $1`, email)
	return err
}

func (r *pendingRegistrationRepository) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.conn.Exec(ctx, `DELETE FROM pending_registrations WHERE expires_at < now()`)
	return tag.RowsAffected(), err
}
