package service

import (
	"tutorgo/models"
	"tutorgo/repository"
)

type PaymentService interface {
	Create(req models.CreatePaymentRequest) (models.Payment, error)
	GetByCourse(courseID string) ([]models.Payment, error)
	GetBalance(courseID string) (int, error)
}

type paymentService struct {
	repo repository.PaymentRepository
}

func NewPaymentService(repo repository.PaymentRepository) PaymentService {
	return &paymentService{repo: repo}
}

func (s *paymentService) Create(req models.CreatePaymentRequest) (models.Payment, error) {
	return s.repo.Create(req)
}

func (s *paymentService) GetByCourse(courseID string) ([]models.Payment, error) {
	return s.repo.GetByCourse(courseID)
}

func (s *paymentService) GetBalance(courseID string) (int, error) {
	return s.repo.GetBalance(courseID)
}
