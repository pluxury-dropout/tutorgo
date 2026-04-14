package service_test

import (
	"context"
	"errors"
	"testing"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockStudentRepo struct {
	mock.Mock
}

func (m *mockStudentRepo) Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error) {
	args := m.Called(ctx, req, tutorID)
	return args.Get(0).(models.Student), args.Error(1)
}

func (m *mockStudentRepo) GetAll(ctx context.Context, tutorID string) ([]models.Student, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).([]models.Student), args.Error(1)
}

func (m *mockStudentRepo) GetByID(ctx context.Context, id string, tutorID string) (models.Student, error) {
	args := m.Called(ctx, id, tutorID)
	return args.Get(0).(models.Student), args.Error(1)
}

func (m *mockStudentRepo) Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error) {
	args := m.Called(ctx, id, tutorID, req)
	return args.Get(0).(models.Student), args.Error(1)
}

func (m *mockStudentRepo) Delete(ctx context.Context, id string, tutorID string) error {
	args := m.Called(ctx, id, tutorID)
	return args.Error(0)
}

// Тесты
func TestGetAllStudents_Success(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo)

	expected := []models.Student{
		{ID: "1", FirstName: "Aiya", LastName: "Bekova", TutorID: "tutor-1"},
		{ID: "2", FirstName: "Zhanibek", LastName: "Gabitov", TutorID: "tutor-1"},
	}

	repo.On("GetAll", mock.Anything, "tutor-1").Return(expected, nil)

	students, err := svc.GetAll(context.Background(), "tutor-1")

	assert.NoError(t, err)
	assert.Equal(t, expected, students)
	repo.AssertExpectations(t)
}

func TestGetAllStudents_Error(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo)

	repo.On("GetAll", mock.Anything, "tutor-1").Return([]models.Student{}, errors.New("db error"))

	students, err := svc.GetAll(context.Background(), "tutor-1")

	assert.Error(t, err)
	assert.Empty(t, students)
	repo.AssertExpectations(t)
}

func TestCreateStudent_Success(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo)

	req := models.CreateStudentRequest{
		FirstName: "Aiya",
		LastName:  "Bekova",
	}
	expected := models.Student{ID: "1", FirstName: "Aiya", LastName: "Bekova", TutorID: "tutor-1"}

	repo.On("Create", mock.Anything, req, "tutor-1").Return(expected, nil)

	student, err := svc.Create(context.Background(), req, "tutor-1")

	assert.NoError(t, err)
	assert.Equal(t, expected, student)
	repo.AssertExpectations(t)
}

func TestCreateStudent_Error(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo)

	req := models.CreateStudentRequest{
		FirstName: "Aiya",
		LastName:  "Bekova",
	}
	repo.On("Create", mock.Anything, req, "tutor-1").Return(models.Student{}, errors.New("failed to create new student"))
	student, err := svc.Create(context.Background(), req, "tutor-1")

	assert.Error(t, err)
	assert.Empty(t, student)
	repo.AssertExpectations(t)
}

func TestDeleteStudent_Success(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo)

	repo.On("Delete", mock.Anything, "student-1", "tutor-1").Return(nil)

	err := svc.Delete(context.Background(), "student-1", "tutor-1")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
