package service

import (
	"tutorgo/models"
	"tutorgo/repository"
)

type LessonService interface {
	Create(req models.CreateLessonRequest) (models.Lesson, error)
	GetByCourse(courseID string) ([]models.Lesson, error)
	GetByID(id string) (models.Lesson, error)
	Update(id string, req models.UpdateLessonRequest) (models.Lesson, error)
	Delete(id string) error
}

type lessonService struct {
	repo repository.LessonRepository
}

func NewLessonService(repo repository.LessonRepository) LessonService {
	return &lessonService{repo: repo}
}

func (s *lessonService) Create(req models.CreateLessonRequest) (models.Lesson, error) {
	return s.repo.Create(req)
}

func (s *lessonService) GetByCourse(courseID string) ([]models.Lesson, error) {
	return s.repo.GetByCourse(courseID)
}

func (s *lessonService) GetByID(id string) (models.Lesson, error) {
	return s.repo.GetByID(id)
}

func (s *lessonService) Update(id string, req models.UpdateLessonRequest) (models.Lesson, error) {
	return s.repo.Update(id, req)
}

func (s *lessonService) Delete(id string) error {
	return s.repo.Delete(id)
}
