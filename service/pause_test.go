package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockPauseRepo struct{ mock.Mock }

func (m *mockPauseRepo) Create(ctx context.Context, studentID string, req models.CreatePauseRequest) (models.StudentPause, []string, error) {
	args := m.Called(ctx, studentID, req)
	return args.Get(0).(models.StudentPause), args.Get(1).([]string), args.Error(2)
}

func (m *mockPauseRepo) ListByStudent(ctx context.Context, studentID string) ([]models.StudentPause, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).([]models.StudentPause), args.Error(1)
}

func (m *mockPauseRepo) Delete(ctx context.Context, id, studentID string) (bool, error) {
	args := m.Called(ctx, id, studentID)
	return args.Bool(0), args.Error(1)
}

type mockMaterializer struct{ mock.Mock }

func (m *mockMaterializer) Materialize(ctx context.Context, ruleID string, horizon time.Time) (int, error) {
	args := m.Called(ctx, ruleID, horizon)
	return args.Int(0), args.Error(1)
}

var pauseDay = time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)

func TestPauseCreate_EndsBeforeStartsRejected(t *testing.T) {
	repo, students := new(mockPauseRepo), new(mockStudentRepo)
	svc := service.NewPauseService(repo, students, new(mockMaterializer))
	students.On("GetByID", mock.Anything, "stu-1", "tutor-1").Return(models.Student{ID: "stu-1"}, nil)

	_, err := svc.Create(context.Background(), "stu-1", "tutor-1",
		models.CreatePauseRequest{StartsOn: pauseDay, EndsOn: pauseDay.AddDate(0, 0, -1)})

	assert.ErrorIs(t, err, service.ErrBadRequest)
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

func TestPauseCreate_ForeignStudentNotFound(t *testing.T) {
	repo, students := new(mockPauseRepo), new(mockStudentRepo)
	svc := service.NewPauseService(repo, students, new(mockMaterializer))
	students.On("GetByID", mock.Anything, "stu-1", "tutor-1").Return(models.Student{}, errors.New("no rows"))

	_, err := svc.Create(context.Background(), "stu-1", "tutor-1",
		models.CreatePauseRequest{StartsOn: pauseDay, EndsOn: pauseDay})

	assert.ErrorIs(t, err, service.ErrNotFound)
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

// Каждое правило со сдвинутым хвостом материализуется сразу; сбой
// материализации паузу не отменяет — ночная ExtendAll догонит (спека, п. 6.9).
func TestPauseCreate_MaterializesShiftedRules(t *testing.T) {
	repo, students, rules := new(mockPauseRepo), new(mockStudentRepo), new(mockMaterializer)
	svc := service.NewPauseService(repo, students, rules)
	req := models.CreatePauseRequest{StartsOn: pauseDay, EndsOn: pauseDay.AddDate(0, 0, 13)}
	students.On("GetByID", mock.Anything, "stu-1", "tutor-1").Return(models.Student{ID: "stu-1"}, nil)
	repo.On("Create", mock.Anything, "stu-1", req).Return(models.StudentPause{ID: "pause-1"}, []string{"r1", "r2"}, nil)
	rules.On("Materialize", mock.Anything, "r1", mock.Anything).Return(2, nil)
	rules.On("Materialize", mock.Anything, "r2", mock.Anything).Return(0, errors.New("boom"))

	pause, err := svc.Create(context.Background(), "stu-1", "tutor-1", req)

	assert.NoError(t, err)
	assert.Equal(t, "pause-1", pause.ID)
	rules.AssertExpectations(t)
}

func TestPauseDelete_MissingIsNotFound(t *testing.T) {
	repo, students := new(mockPauseRepo), new(mockStudentRepo)
	svc := service.NewPauseService(repo, students, new(mockMaterializer))
	students.On("GetByID", mock.Anything, "stu-1", "tutor-1").Return(models.Student{ID: "stu-1"}, nil)
	repo.On("Delete", mock.Anything, "pause-1", "stu-1").Return(false, nil)

	err := svc.Delete(context.Background(), "pause-1", "stu-1", "tutor-1")

	assert.ErrorIs(t, err, service.ErrNotFound)
}
