package service

import (
	"context"
	"fmt"
	"tutorgo/models"
	"tutorgo/repository"
)

type StudentService interface {
	Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error)
	GetAll(ctx context.Context, tutorID string) ([]models.Student, error)
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

func (s *studentService) GetAll(ctx context.Context, tutorID string) ([]models.Student, error) {
	return s.repo.GetAll(ctx, tutorID)
}

func (s *studentService) GetByID(ctx context.Context, id string, tutorID string) (models.Student, error) {
	student, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return models.Student{}, fmt.Errorf("student: %w", ErrNotFound)
	}
	return student, nil
}

func (s *studentService) Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error) {
	student, err := s.repo.Update(ctx, id, tutorID, req)
	if err != nil {
		return models.Student{}, fmt.Errorf("student: %w", ErrNotFound)
	}
	return student, nil
}

func (s *studentService) Delete(ctx context.Context, id string, tutorID string) error {
	if err := s.repo.Delete(ctx, id, tutorID); err != nil {
		return fmt.Errorf("student: %w", ErrNotFound)
	}
	return nil
}
