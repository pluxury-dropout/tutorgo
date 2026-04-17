package service

import (
	"context"
	"errors"
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
		return models.CourseEnrollment{}, errors.New("course not found or access denied")
	}
	if course.StudentID != nil {
		return models.CourseEnrollment{}, errors.New("cannot enroll students in an individual course")
	}
	if _, err := s.studentRepo.GetByID(ctx, req.StudentID, tutorID); err != nil {
		return models.CourseEnrollment{}, errors.New("student not found or access denied")
	}
	return s.repo.Add(ctx, courseID, req.StudentID)
}

func (s *enrollmentService) Remove(ctx context.Context, courseID string, studentID string, tutorID string) error {
	course, err := s.courseRepo.GetByID(ctx, courseID, tutorID)
	if err != nil {
		return errors.New("course not found or access denied")
	}
	if course.StudentID != nil {
		return errors.New("cannot modify enrollments for an individual course")
	}
	return s.repo.Remove(ctx, courseID, studentID)
}

func (s *enrollmentService) GetByCourse(ctx context.Context, courseID string, tutorID string) ([]models.CourseEnrollment, error) {
	if _, err := s.courseRepo.GetByID(ctx, courseID, tutorID); err != nil {
		return nil, errors.New("course not found or access denied")
	}
	return s.repo.GetByCourse(ctx, courseID)
}
