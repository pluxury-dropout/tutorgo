package service

import (
	"context"
	"fmt"
	"tutorgo/models"
	"tutorgo/repository"
)

type EnrollmentService interface {
	Add(ctx context.Context, courseID string, req models.EnrollStudentRequest, tutorID string) (models.CourseEnrollment, error)
	AddBulk(ctx context.Context, courseID string, req models.EnrollStudentsBulkRequest, tutorID string) ([]models.CourseEnrollment, error)
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
	student, err := s.studentRepo.GetByID(ctx, req.StudentID, tutorID)
	if err != nil {
		return models.CourseEnrollment{}, fmt.Errorf("student: %w", ErrNotFound)
	}
	// Пикеры архивных не показывают — это на случай вкладки, открытой до архивации.
	if !student.Active {
		return models.CourseEnrollment{}, fmt.Errorf("student archived: %w", ErrBadRequest)
	}
	return s.repo.Add(ctx, courseID, req.StudentID)
}

// AddBulk отличается от Add не только числом: владение учениками проверяет сам
// запрос (см. repository.AddBulk), поэтому N обращений к studentRepo тут нет.
func (s *enrollmentService) AddBulk(ctx context.Context, courseID string, req models.EnrollStudentsBulkRequest, tutorID string) ([]models.CourseEnrollment, error) {
	course, err := s.courseRepo.GetByID(ctx, courseID, tutorID)
	if err != nil {
		return nil, fmt.Errorf("course: %w", ErrNotFound)
	}
	if course.StudentID != nil {
		return nil, fmt.Errorf("individual course: %w", ErrForbidden)
	}
	return s.repo.AddBulk(ctx, courseID, req.StudentIDs, tutorID)
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
