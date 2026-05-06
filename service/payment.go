package service

import (
	"context"
	"fmt"
	"tutorgo/models"
	"tutorgo/repository"
)

type PaymentService interface {
	Create(ctx context.Context, req models.CreatePaymentRequest, tutorID string) (models.Payment, error)
	GetByCourse(ctx context.Context, courseID string, tutorID string, p models.Pagination) ([]models.Payment, int, error)
	GetAllByTutor(ctx context.Context, tutorID string, limit int) ([]models.Payment, error)
	GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error)
	GetBalance(ctx context.Context, courseID string, tutorID string) (models.CourseBalance, error)
	GetMonthlyIncome(ctx context.Context, tutorID string) (float64, error)
}

type paymentService struct {
	repo       repository.PaymentRepository
	courseRepo repository.CourseRepository
}

func NewPaymentService(repo repository.PaymentRepository, courseRepo repository.CourseRepository) PaymentService {
	return &paymentService{repo: repo, courseRepo: courseRepo}
}

func (s *paymentService) Create(ctx context.Context, req models.CreatePaymentRequest, tutorID string) (models.Payment, error) {
	if _, err := s.courseRepo.GetByID(ctx, req.CourseID, tutorID); err != nil {
		return models.Payment{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.Create(ctx, req)
}

func (s *paymentService) GetByCourse(ctx context.Context, courseID string, tutorID string, p models.Pagination) ([]models.Payment, int, error) {
	if _, err := s.courseRepo.GetByID(ctx, courseID, tutorID); err != nil {
		return nil, 0, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.GetByCourse(ctx, courseID, p)
}

func (s *paymentService) GetAllByTutor(ctx context.Context, tutorID string, limit int) ([]models.Payment, error) {
	return s.repo.GetAllByTutor(ctx, tutorID, limit)
}

func (s *paymentService) GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error) {
	return s.repo.GetAllByTutorPaged(ctx, tutorID, p)
}

func (s *paymentService) GetBalance(ctx context.Context, courseID string, tutorID string) (models.CourseBalance, error) {
	if _, err := s.courseRepo.GetByID(ctx, courseID, tutorID); err != nil {
		return models.CourseBalance{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.GetBalance(ctx, courseID)
}

func (s *paymentService) GetMonthlyIncome(ctx context.Context, tutorID string) (float64, error) {
	return s.repo.GetMonthlyIncome(ctx, tutorID)
}
