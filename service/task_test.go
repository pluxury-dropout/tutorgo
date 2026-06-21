package service_test

import (
	"context"
	"testing"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockTaskRepo mocks repository.TaskRepository
type mockTaskRepo struct{ mock.Mock }

func (m *mockTaskRepo) Create(ctx context.Context, tutorID string, req models.CreateTaskRequest) (models.Task, error) {
	args := m.Called(ctx, tutorID, req)
	return args.Get(0).(models.Task), args.Error(1)
}

func (m *mockTaskRepo) GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Task, error) {
	args := m.Called(ctx, tutorID, from, to)
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *mockTaskRepo) Update(ctx context.Context, id, tutorID string, req models.UpdateTaskRequest) (models.Task, error) {
	args := m.Called(ctx, id, tutorID, req)
	return args.Get(0).(models.Task), args.Error(1)
}

func (m *mockTaskRepo) Delete(ctx context.Context, id, tutorID string) error {
	args := m.Called(ctx, id, tutorID)
	return args.Error(0)
}

func TestTaskService_Create_DefaultsStatus(t *testing.T) {
	repo := new(mockTaskRepo)
	svc := service.NewTaskService(repo)

	req := models.CreateTaskRequest{Title: "Test task", DurationMinutes: 30}
	expected := models.Task{ID: "1", Status: "not_urgent"}

	repo.On("Create", mock.Anything, "tutor-1", models.CreateTaskRequest{
		Title: "Test task", DurationMinutes: 30, Status: "not_urgent",
	}).Return(expected, nil)

	result, err := svc.Create(context.Background(), "tutor-1", req)
	assert.NoError(t, err)
	assert.Equal(t, "not_urgent", result.Status)
	repo.AssertExpectations(t)
}

func TestTaskService_Create_PreservesExplicitStatus(t *testing.T) {
	repo := new(mockTaskRepo)
	svc := service.NewTaskService(repo)

	req := models.CreateTaskRequest{Title: "Urgent task", DurationMinutes: 30, Status: "urgent"}
	expected := models.Task{ID: "2", Status: "urgent"}

	repo.On("Create", mock.Anything, "tutor-1", req).Return(expected, nil)

	result, err := svc.Create(context.Background(), "tutor-1", req)
	assert.NoError(t, err)
	assert.Equal(t, "urgent", result.Status)
	repo.AssertExpectations(t)
}
