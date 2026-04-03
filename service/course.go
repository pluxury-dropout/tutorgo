package service

import (
	"tutorgo/models"
	"tutorgo/repository"
)

type CourseService interface {
	Create(req models.CreateCourseRequest, tutorID string) (models.Course, error)
	GetAll(tutorID string) ([]models.Course, error)
	GetByID(id string, tutorID string) (models.Course, error)
	Update(id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error)
	Delete(id string, tutorID string) error
}

type courseService struct {
	repo repository.CourseRepository
}

func NewCourseService(repo repository.CourseRepository) CourseService {
	return &courseService{repo: repo}
}

func (s *courseService) Create(req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	return s.repo.Create(req, tutorID)
}

func (s *courseService) GetAll(tutorID string) ([]models.Course, error) {
	return s.repo.GetAll(tutorID)
}

func (s *courseService) GetByID(id string, tutorID string) (models.Course, error) {
	return s.repo.GetByID(id, tutorID)
}

func (s *courseService) Update(id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	return s.repo.Update(id, tutorID, req)
}

func (s *courseService) Delete(id string, tutorID string) error {
	return s.repo.Delete(id, tutorID)
}
