package service

import (
	"tutorgo/models"
	"tutorgo/repository"
)

type StudentService interface {
	Create(req models.CreateStudentRequest, tutorID string) (models.Student, error)
	GetAll(tutorID string) ([]models.Student, error)
	GetByID(id string, tutorID string) (models.Student, error)
	Update(id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error)
	Delete(id string, tutorID string) error
}

type studentService struct {
	repo repository.StudentRepository
}

func NewStudentService(repo repository.StudentRepository) StudentService {
	return &studentService{repo: repo}
}

func (s *studentService) Create(req models.CreateStudentRequest, tutorID string) (models.Student, error) {
	return s.repo.Create(req, tutorID)
}

func (s *studentService) GetAll(tutorID string) ([]models.Student, error) {
	return s.repo.GetAll(tutorID)
}

func (s *studentService) GetByID(id string, tutorID string) (models.Student, error) {
	return s.repo.GetByID(id, tutorID)
}

func (s *studentService) Update(id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error) {
	return s.repo.Update(id, tutorID, req)
}

func (s *studentService) Delete(id string, tutorID string) error {
	return s.repo.Delete(id, tutorID)
}
