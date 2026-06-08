package service

import (
	"context"
	"fmt"
	"tutorgo/models"
	"tutorgo/repository"
)

type CourseService interface {
	Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error)
	GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Course, error)
	GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error)
	Delete(ctx context.Context, id string, tutorID string) error
	GetArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error)
	Restore(ctx context.Context, id string, tutorID string) error
}

type courseService struct {
	repo        repository.CourseRepository
	studentRepo repository.StudentRepository
}

func NewCourseService(repo repository.CourseRepository, studentRepo repository.StudentRepository) CourseService {
	return &courseService{repo: repo, studentRepo: studentRepo}
}

func (s *courseService) Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	if req.StudentID != nil {
		_, err := s.studentRepo.GetByID(ctx, *req.StudentID, tutorID)
		if err != nil {
			return models.Course{}, fmt.Errorf("student: %w", ErrNotFound)
		}
	}
	return s.repo.Create(ctx, req, tutorID)
}

func (s *courseService) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	return s.repo.GetAll(ctx, tutorID, p)
}

func (s *courseService) GetByID(ctx context.Context, id string, tutorID string) (models.Course, error) {
	course, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return models.Course{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	return course, nil
}

func (s *courseService) GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error) {
	if _, err := s.studentRepo.GetByID(ctx, studentID, tutorID); err != nil {
		return nil, fmt.Errorf("student: %w", ErrNotFound)
	}
	return s.repo.GetByStudent(ctx, studentID, tutorID)
}

func (s *courseService) Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	_, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return models.Course{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.Update(ctx, id, tutorID, req)
}

func (s *courseService) Delete(ctx context.Context, id string, tutorID string) error {
	_, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.Delete(ctx, id, tutorID)
}

func (s *courseService) GetArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	return s.repo.GetAllArchived(ctx, tutorID, p)
}

func (s *courseService) Restore(ctx context.Context, id string, tutorID string) error {
	_, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.Restore(ctx, id, tutorID)
}
