package service

import (
	"context"
	"errors"
	"tutorgo/models"
	"tutorgo/repository"
)

type PaymentService interface {
	Create(ctx context.Context, req models.CreatePaymentRequest, tutorID string) (models.Payment, error)
	GetByCourse(ctx context.Context, courseID string, tutorID string) ([]models.Payment, error)
	GetBalance(ctx context.Context, courseID string, tutorID string) (models.CourseBalance, error)
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
		return models.Payment{}, errors.New("course not found or access denied")
	}
	return s.repo.Create(ctx, req)
}

func (s *paymentService) GetByCourse(ctx context.Context, courseID string, tutorID string) ([]models.Payment, error) {
	if _, err := s.courseRepo.GetByID(ctx, courseID, tutorID); err != nil {
		return nil, errors.New("course not found or access denied")
	}
	return s.repo.GetByCourse(ctx, courseID)
}

func (s *paymentService) GetBalance(ctx context.Context, courseID string, tutorID string) (models.CourseBalance, error) {
	if _, err := s.courseRepo.GetByID(ctx, courseID, tutorID); err != nil {
		return models.CourseBalance{}, errors.New("course not found or access denied")
	}
	return s.repo.GetBalance(ctx, courseID)
}
