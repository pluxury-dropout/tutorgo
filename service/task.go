package service

import (
	"context"
	"tutorgo/models"
	"tutorgo/repository"
)

type TaskService interface {
	Create(ctx context.Context, tutorID string, req models.CreateTaskRequest) (models.Task, error)
	GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Task, error)
	Update(ctx context.Context, id, tutorID string, req models.UpdateTaskRequest) (models.Task, error)
	Delete(ctx context.Context, id, tutorID string) error
	ToggleDone(ctx context.Context, id, tutorID string) (models.Task, error)
}

type taskService struct {
	repo repository.TaskRepository
}

func NewTaskService(repo repository.TaskRepository) TaskService {
	return &taskService{repo: repo}
}

func (s *taskService) Create(ctx context.Context, tutorID string, req models.CreateTaskRequest) (models.Task, error) {
	return s.repo.Create(ctx, tutorID, req)
}

func (s *taskService) GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Task, error) {
	return s.repo.GetByRange(ctx, tutorID, from, to)
}

func (s *taskService) Update(ctx context.Context, id, tutorID string, req models.UpdateTaskRequest) (models.Task, error) {
	return s.repo.Update(ctx, id, tutorID, req)
}

func (s *taskService) Delete(ctx context.Context, id, tutorID string) error {
	return s.repo.Delete(ctx, id, tutorID)
}

func (s *taskService) ToggleDone(ctx context.Context, id, tutorID string) (models.Task, error) {
	return s.repo.ToggleDone(ctx, id, tutorID)
}
