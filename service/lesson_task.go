package service

import (
	"context"
	"errors"
	"fmt"

	"tutorgo/models"
	"tutorgo/repository"

	"github.com/jackc/pgx/v5"
)

// LessonTaskService — бизнес-логика задач урока. Authz fails-closed: ученик
// касается задачи только через EnrolledInLesson; репетитор — только через
// владение уроком (проверка вшита в SQL репозитория, маппим 0 строк в ErrNotFound).
type LessonTaskService interface {
	ListForTutor(ctx context.Context, lessonID, tutorID string) ([]models.LessonTask, error)
	Create(ctx context.Context, lessonID, tutorID string, req models.CreateLessonTaskRequest) (models.LessonTask, error)
	Update(ctx context.Context, taskID, tutorID string, req models.UpdateLessonTaskRequest) (models.LessonTask, error)
	Delete(ctx context.Context, taskID, tutorID string) error
	ListForStudent(ctx context.Context, lessonID, studentID string) ([]models.LessonTask, error)
	SetDone(ctx context.Context, taskID, studentID string, done bool) error
}

type lessonTaskService struct {
	repo        repository.LessonTaskRepository
	studentRepo repository.StudentRepository
}

func NewLessonTaskService(repo repository.LessonTaskRepository, studentRepo repository.StudentRepository) LessonTaskService {
	return &lessonTaskService{repo: repo, studentRepo: studentRepo}
}

func (s *lessonTaskService) ListForTutor(ctx context.Context, lessonID, tutorID string) ([]models.LessonTask, error) {
	return s.repo.ListByLessonForTutor(ctx, lessonID, tutorID)
}

func (s *lessonTaskService) Create(ctx context.Context, lessonID, tutorID string, req models.CreateLessonTaskRequest) (models.LessonTask, error) {
	task, err := s.repo.Create(ctx, lessonID, tutorID, req)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.LessonTask{}, fmt.Errorf("task: %w", ErrNotFound)
	}
	return task, err
}

func (s *lessonTaskService) Update(ctx context.Context, taskID, tutorID string, req models.UpdateLessonTaskRequest) (models.LessonTask, error) {
	task, err := s.repo.Update(ctx, taskID, tutorID, req)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.LessonTask{}, fmt.Errorf("task: %w", ErrNotFound)
	}
	return task, err
}

func (s *lessonTaskService) Delete(ctx context.Context, taskID, tutorID string) error {
	n, err := s.repo.Delete(ctx, taskID, tutorID)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("task: %w", ErrNotFound)
	}
	return nil
}

func (s *lessonTaskService) ListForStudent(ctx context.Context, lessonID, studentID string) ([]models.LessonTask, error) {
	ok, err := s.studentRepo.EnrolledInLesson(ctx, studentID, lessonID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("task: %w", ErrForbidden)
	}
	return s.repo.ListByLessonForStudent(ctx, lessonID)
}

func (s *lessonTaskService) SetDone(ctx context.Context, taskID, studentID string, done bool) error {
	lessonID, err := s.repo.LessonIDByTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task: %w", ErrNotFound)
	}
	ok, err := s.studentRepo.EnrolledInLesson(ctx, studentID, lessonID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("task: %w", ErrForbidden)
	}
	n, err := s.repo.SetDone(ctx, taskID, done)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("task: %w", ErrNotFound)
	}
	return nil
}
