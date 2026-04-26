package service

import (
	"context"
	"errors"
	"tutorgo/models"
	"tutorgo/repository"
)

type CourseService interface {
	Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error)
	GetAll(ctx context.Context, tutorID string) ([]models.Course, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Course, error)
	GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error)
	Delete(ctx context.Context, id string, tutorID string) error
}

type courseService struct {
	repo        repository.CourseRepository
	studentRepo repository.StudentRepository
	lessonRepo  repository.LessonRepository
}

func NewCourseService(repo repository.CourseRepository, studentRepo repository.StudentRepository, lessonRepo repository.LessonRepository) CourseService {
	return &courseService{repo: repo, studentRepo: studentRepo, lessonRepo: lessonRepo}
}

func (s *courseService) Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	if req.StudentID != nil {
		_, err := s.studentRepo.GetByID(ctx, *req.StudentID, tutorID)
		if err != nil {
			return models.Course{}, errors.New("student not found or access denied")
		}
	}
	return s.repo.Create(ctx, req, tutorID)
}

func (s *courseService) GetAll(ctx context.Context, tutorID string) ([]models.Course, error) {
	return s.repo.GetAll(ctx, tutorID)
}

func (s *courseService) GetByID(ctx context.Context, id string, tutorID string) (models.Course, error) {
	return s.repo.GetByID(ctx, id, tutorID)
}

func (s *courseService) GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error) {
	if _, err := s.studentRepo.GetByID(ctx, studentID, tutorID); err != nil {
		return nil, errors.New("student not found or access denied")
	}
	return s.repo.GetByStudent(ctx, studentID, tutorID)
}

func (s *courseService) Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	_, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return models.Course{}, errors.New("course not found or access denied")
	}
	return s.repo.Update(ctx, id, tutorID, req)
}

func (s *courseService) Delete(ctx context.Context, id string, tutorID string) error {
	_, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return errors.New("course not found or access denied")
	}

	lessons, err := s.lessonRepo.GetByCourse(ctx, id)
	if err != nil {
		return err
	}
	if len(lessons) > 0 {
		return errors.New("cannot delete a course with existing lessons")
	}
	return s.repo.Delete(ctx, id, tutorID)
}
