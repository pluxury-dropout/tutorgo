package service

import (
	"errors"
	"tutorgo/models"
	"tutorgo/repository"
)

type LessonService interface {
	Create(req models.CreateLessonRequest, tutorID string) (models.Lesson, error)
	GetByCourse(courseID string, tutorID string) ([]models.Lesson, error)
	GetByID(id string, tutorID string) (models.Lesson, error)
	Update(id string, req models.UpdateLessonRequest, tutorID string) (models.Lesson, error)
	Delete(id string, tutorID string) error
}

type lessonService struct {
	repo       repository.LessonRepository
	courseRepo repository.CourseRepository
}

func NewLessonService(repo repository.LessonRepository, courseRepo repository.CourseRepository) LessonService {
	return &lessonService{repo: repo, courseRepo: courseRepo}
}

func (s *lessonService) Create(req models.CreateLessonRequest, tutorID string) (models.Lesson, error) {
	_, err := s.courseRepo.GetByID(req.CourseID, tutorID)
	if err != nil {
		return models.Lesson{}, errors.New("course not found or access denied")
	}
	return s.repo.Create(req)
}

func (s *lessonService) GetByCourse(courseID string, tutorID string) ([]models.Lesson, error) {
	_, err := s.courseRepo.GetByID(courseID, tutorID)
	if err != nil {
		return nil, errors.New("course not found or access denied")
	}
	return s.repo.GetByCourse(courseID)
}

func (s *lessonService) GetByID(id string, tutorID string) (models.Lesson, error) {
	lesson, err := s.repo.GetByID(id)
	if err != nil {
		return models.Lesson{}, err
	}
	_, err = s.courseRepo.GetByID(lesson.CourseID, tutorID)
	if err != nil {
		return models.Lesson{}, errors.New("lesson not found or access denied")
	}
	return lesson, nil
}

func (s *lessonService) Update(id string, req models.UpdateLessonRequest, tutorID string) (models.Lesson, error) {
	lesson, err := s.repo.GetByID(id)
	if err != nil {
		return models.Lesson{}, err
	}
	_, err = s.courseRepo.GetByID(lesson.CourseID, tutorID)
	if err != nil {
		return models.Lesson{}, errors.New("lesson not found or access denied")
	}
	return s.repo.Update(id, req)
}

func (s *lessonService) Delete(id string, tutorID string) error {
	lesson, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	_, err = s.courseRepo.GetByID(lesson.CourseID, tutorID)
	if err != nil {
		return errors.New("lesson not found or access denied")
	}
	return s.repo.Delete(id)
}
