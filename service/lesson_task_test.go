package service_test

import (
	"context"
	"testing"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockLessonTaskRepo mocks repository.LessonTaskRepository
type mockLessonTaskRepo struct{ mock.Mock }

func (m *mockLessonTaskRepo) ListByLessonForTutor(ctx context.Context, lessonID, tutorID string) ([]models.LessonTask, error) {
	args := m.Called(ctx, lessonID, tutorID)
	return args.Get(0).([]models.LessonTask), args.Error(1)
}
func (m *mockLessonTaskRepo) Create(ctx context.Context, lessonID, tutorID string, req models.CreateLessonTaskRequest) (models.LessonTask, error) {
	args := m.Called(ctx, lessonID, tutorID, req)
	return args.Get(0).(models.LessonTask), args.Error(1)
}
func (m *mockLessonTaskRepo) Update(ctx context.Context, taskID, tutorID string, req models.UpdateLessonTaskRequest) (models.LessonTask, error) {
	args := m.Called(ctx, taskID, tutorID, req)
	return args.Get(0).(models.LessonTask), args.Error(1)
}
func (m *mockLessonTaskRepo) Delete(ctx context.Context, taskID, tutorID string) (int64, error) {
	args := m.Called(ctx, taskID, tutorID)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockLessonTaskRepo) ListByLessonForStudent(ctx context.Context, lessonID string) ([]models.LessonTask, error) {
	args := m.Called(ctx, lessonID)
	return args.Get(0).([]models.LessonTask), args.Error(1)
}
func (m *mockLessonTaskRepo) SetDone(ctx context.Context, taskID string, done bool) (int64, error) {
	args := m.Called(ctx, taskID, done)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockLessonTaskRepo) LessonIDByTask(ctx context.Context, taskID string) (string, error) {
	args := m.Called(ctx, taskID)
	return args.String(0), args.Error(1)
}

func TestTask_StudentSetDone_NotEnrolled(t *testing.T) {
	repo := new(mockLessonTaskRepo)
	stuRepo := new(mockStudentRepo)
	svc := service.NewLessonTaskService(repo, stuRepo)

	repo.On("LessonIDByTask", mock.Anything, "task-1").Return("lesson-1", nil)
	stuRepo.On("EnrolledInLesson", mock.Anything, "stu-1", "lesson-1").Return(false, nil)

	err := svc.SetDone(context.Background(), "task-1", "stu-1", true)

	assert.ErrorIs(t, err, service.ErrForbidden)
	repo.AssertNotCalled(t, "SetDone")
	repo.AssertExpectations(t)
	stuRepo.AssertExpectations(t)
}

func TestTask_StudentSetDone_OK(t *testing.T) {
	repo := new(mockLessonTaskRepo)
	stuRepo := new(mockStudentRepo)
	svc := service.NewLessonTaskService(repo, stuRepo)

	repo.On("LessonIDByTask", mock.Anything, "task-1").Return("lesson-1", nil)
	stuRepo.On("EnrolledInLesson", mock.Anything, "stu-1", "lesson-1").Return(true, nil)
	repo.On("SetDone", mock.Anything, "task-1", true).Return(int64(1), nil)

	err := svc.SetDone(context.Background(), "task-1", "stu-1", true)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
	stuRepo.AssertExpectations(t)
}

func TestTask_StudentList_NotEnrolled(t *testing.T) {
	repo := new(mockLessonTaskRepo)
	stuRepo := new(mockStudentRepo)
	svc := service.NewLessonTaskService(repo, stuRepo)

	stuRepo.On("EnrolledInLesson", mock.Anything, "stu-1", "lesson-1").Return(false, nil)

	tasks, err := svc.ListForStudent(context.Background(), "lesson-1", "stu-1")

	assert.ErrorIs(t, err, service.ErrForbidden)
	assert.Nil(t, tasks)
	repo.AssertNotCalled(t, "ListByLessonForStudent")
	stuRepo.AssertExpectations(t)
}

func TestTask_TutorCreate_NotOwned(t *testing.T) {
	repo := new(mockLessonTaskRepo)
	stuRepo := new(mockStudentRepo)
	svc := service.NewLessonTaskService(repo, stuRepo)

	req := models.CreateLessonTaskRequest{Title: "HW"}
	repo.On("Create", mock.Anything, "lesson-1", "tutor-1", req).
		Return(models.LessonTask{}, pgx.ErrNoRows)

	task, err := svc.Create(context.Background(), "lesson-1", "tutor-1", req)

	assert.ErrorIs(t, err, service.ErrNotFound)
	assert.Empty(t, task)
	repo.AssertExpectations(t)
}
