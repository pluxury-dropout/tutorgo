package service

import (
	"tutorgo/models"
	"tutorgo/repository"
)

type TutorService interface {
	Create(req models.CreateTutorRequest, passwordHash string) (models.Tutor, error)
	GetAll() ([]models.Tutor, error)
	GetByID(id string) (models.Tutor, error)
	GetByEmail(email string) (string, string, error)
	Update(id string, req models.UpdateTutorRequest) (models.Tutor, error)
	Delete(id string) error
}

type tutorService struct {
	repo repository.TutorRepository
}

func NewTutorService(repo repository.TutorRepository) TutorService {
	return &tutorService{repo: repo}
}

func (s *tutorService) Create(req models.CreateTutorRequest, passwordHash string) (models.Tutor, error) {
	return s.repo.Create(req, passwordHash)
}

func (s *tutorService) GetAll() ([]models.Tutor, error) {
	return s.repo.GetAll()
}

func (s *tutorService) GetByID(id string) (models.Tutor, error) {
	return s.repo.GetByID(id)
}

func (s *tutorService) GetByEmail(email string) (string, string, error) {
	return s.repo.GetByEmail(email)
}

func (s *tutorService) Update(id string, req models.UpdateTutorRequest) (models.Tutor, error) {
	return s.repo.Update(id, req)
}

func (s *tutorService) Delete(id string) error {
	return s.repo.Delete(id)
}
