package service

import (
	"errors"
	"tutorgo/models"
	"tutorgo/repository"
)

type PaymentService interface {
	Create(req models.CreatePaymentRequest, tutorID string) (models.Payment, error)
	GetByCourse(courseID string, tutorID string) ([]models.Payment, error)
	GetBalance(courseID string, tutorID string) (models.CourseBalance, error)
}

type paymentService struct {
	repo       repository.PaymentRepository
	courseRepo repository.CourseRepository
}

func NewPaymentService(repo repository.PaymentRepository, courseRepo repository.CourseRepository) PaymentService {
	return &paymentService{repo: repo, courseRepo: courseRepo}
}

func (s *paymentService) Create(req models.CreatePaymentRequest, tutorID string) (models.Payment, error) {
	if _, err := s.courseRepo.GetByID(req.CourseID, tutorID); err != nil {
		return models.Payment{}, errors.New("course not found or access denied")
	}
	return s.repo.Create(req)
}

func (s *paymentService) GetByCourse(courseID string, tutorID string) ([]models.Payment, error) {
	if _, err := s.courseRepo.GetByID(courseID, tutorID); err != nil {
		return nil, errors.New("course not found or access denied")
	}
	return s.repo.GetByCourse(courseID)
}

func (s *paymentService) GetBalance(courseID string, tutorID string) (models.CourseBalance, error) {
	if _, err := s.courseRepo.GetByID(courseID, tutorID); err != nil {
		return models.CourseBalance{}, errors.New("course not found or access denied")
	}
	return s.repo.GetBalance(courseID)
}
