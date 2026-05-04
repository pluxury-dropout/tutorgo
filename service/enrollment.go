package service

import (
	"context"
	"fmt"
	"tutorgo/models"
	"tutorgo/repository"
)

type EnrollmentService interface {
	Add(ctx context.Context, courseID string, req models.EnrollStudentRequest, tutorID string) (models.CourseEnrollment, error)
	Remove(ctx context.Context, courseID string, studentID string, tutorID string) error
	GetByCourse(ctx context.Context, courseID string, tutorID string) ([]models.CourseEnrollment, error)
}

type enrollmentService struct {
	repo        repository.EnrollmentRepository
	courseRepo  repository.CourseRepository
	studentRepo repository.StudentRepository
}

func NewEnrollmentService(repo repository.EnrollmentRepository, courseRepo repository.CourseRepository, studentRepo repository.StudentRepository) EnrollmentService {
	return &enrollmentService{repo: repo, courseRepo: courseRepo, studentRepo: studentRepo}
}

func (s *enrollmentService) Add(ctx context.Context, courseID string, req models.EnrollStudentRequest, tutorID string) (models.CourseEnrollment, error) {
	course, err := s.courseRepo.GetByID(ctx, courseID, tutorID)
	if err != nil {
		return models.CourseEnrollment{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	if course.StudentID != nil {
		return models.CourseEnrollment{}, fmt.Errorf("individual course: %w", ErrForbidden)
	}
	if _, err := s.studentRepo.GetByID(ctx, req.StudentID, tutorID); err != nil {
		return models.CourseEnrollment{}, fmt.Errorf("student: %w", ErrNotFound)
	}
	return s.repo.Add(ctx, courseID, req.StudentID)
}

func (s *enrollmentService) Remove(ctx context.Context, courseID string, studentID string, tutorID string) error {
	course, err := s.courseRepo.GetByID(ctx, courseID, tutorID)
	if err != nil {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	if course.StudentID != nil {
		return fmt.Errorf("individual course: %w", ErrForbidden)
	}
	return s.repo.Remove(ctx, courseID, studentID)
}

func (s *enrollmentService) GetByCourse(ctx context.Context, courseID string, tutorID string) ([]models.CourseEnrollment, error) {
	if _, err := s.courseRepo.GetByID(ctx, courseID, tutorID); err != nil {
		return nil, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.GetByCourse(ctx, courseID)
}
