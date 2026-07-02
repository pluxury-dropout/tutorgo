package service

import (
	"context"
	"tutorgo/models"
	"tutorgo/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TutorService interface {
	Create(ctx context.Context, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error)
	Register(ctx context.Context, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error)
	GetAll(ctx context.Context) ([]models.Tutor, error)
	GetByID(ctx context.Context, id string) (models.Tutor, error)
	GetByEmail(ctx context.Context, email string) (string, string, error)
	GetByPhone(ctx context.Context, phone string) (string, string, error)
	Update(ctx context.Context, id string, req models.UpdateTutorRequest) (models.Tutor, error)
	Delete(ctx context.Context, id string) error
	GetPasswordHash(ctx context.Context, id string) (string, error)
	UpdatePassword(ctx context.Context, id string, hash string) error
}

type tutorService struct {
	repo    repository.TutorRepository
	subRepo repository.SubscriptionRepository
	pool    *pgxpool.Pool
}

func NewTutorService(repo repository.TutorRepository, subRepo repository.SubscriptionRepository, pool *pgxpool.Pool) TutorService {
	return &tutorService{repo: repo, subRepo: subRepo, pool: pool}
}

func (s *tutorService) Create(ctx context.Context, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error) {
	return s.repo.Create(ctx, req, passwordHash)
}

func (s *tutorService) Register(ctx context.Context, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Tutor{}, err
	}
	defer tx.Rollback(ctx) // no-op после Commit

	tutor, err := s.repo.CreateTx(ctx, tx, req, passwordHash)
	if err != nil {
		return models.Tutor{}, err
	}
	if err := s.subRepo.CreateTrialTx(ctx, tx, tutor.ID); err != nil {
		return models.Tutor{}, err
	}
	return tutor, tx.Commit(ctx)
}

func (s *tutorService) GetAll(ctx context.Context) ([]models.Tutor, error) {
	return s.repo.GetAll(ctx)
}

func (s *tutorService) GetByID(ctx context.Context, id string) (models.Tutor, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *tutorService) GetByEmail(ctx context.Context, email string) (string, string, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *tutorService) GetByPhone(ctx context.Context, phone string) (string, string, error) {
	return s.repo.GetByPhone(ctx, phone)
}

func (s *tutorService) Update(ctx context.Context, id string, req models.UpdateTutorRequest) (models.Tutor, error) {
	return s.repo.Update(ctx, id, req)
}

func (s *tutorService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *tutorService) GetPasswordHash(ctx context.Context, id string) (string, error) {
	return s.repo.GetPasswordHash(ctx, id)
}

func (s *tutorService) UpdatePassword(ctx context.Context, id string, hash string) error {
	return s.repo.UpdatePassword(ctx, id, hash)
}
