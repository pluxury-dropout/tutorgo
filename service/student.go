package service

import (
	"context"
	"fmt"
	"tutorgo/models"
	"tutorgo/repository"
)

type StudentService interface {
	Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error)
	GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Student, int, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Student, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error)
	Delete(ctx context.Context, id string, tutorID string) error
}

type studentService struct {
	repo repository.StudentRepository
}

func NewStudentService(repo repository.StudentRepository) StudentService {
	return &studentService{repo: repo}
}

func (s *studentService) Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error) {
	return s.repo.Create(ctx, req, tutorID)
}

func (s *studentService) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Student, int, error) {
	return s.repo.GetAll(ctx, tutorID, p)
}

func (s *studentService) GetByID(ctx context.Context, id string, tutorID string) (models.Student, error) {
	student, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return models.Student{}, fmt.Errorf("student: %w", ErrNotFound)
	}
	return student, nil
}

func (s *studentService) Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error) {
	if _, err := s.repo.GetByID(ctx, id, tutorID); err != nil {
		return models.Student{}, fmt.Errorf("student: %w", ErrNotFound)
	}
	return s.repo.Update(ctx, id, tutorID, req)
}

func (s *studentService) Delete(ctx context.Context, id string, tutorID string) error {
	if _, err := s.repo.GetByID(ctx, id, tutorID); err != nil {
		return fmt.Errorf("student: %w", ErrNotFound)
	}
	return s.repo.Delete(ctx, id, tutorID)
}
