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

type mockTutorRepo struct {
	mock.Mock
}

func (m *mockTutorRepo) Create(ctx context.Context, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error) {
	args := m.Called(ctx, req, passwordHash)
	return args.Get(0).(models.Tutor), args.Error(1)
}

func (m *mockTutorRepo) GetAll(ctx context.Context) ([]models.Tutor, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Tutor), args.Error(1)
}

func (m *mockTutorRepo) GetByID(ctx context.Context, id string) (models.Tutor, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.Tutor), args.Error(1)
}
func (m *mockTutorRepo) GetByEmail(ctx context.Context, email string) (string, string, error) {
	args := m.Called(ctx, email)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *mockTutorRepo) GetByPhone(ctx context.Context, phone string) (string, string, error) {
	args := m.Called(ctx, phone)
	return args.String(0), args.String(1), args.Error(2)
}
func (m *mockTutorRepo) Update(ctx context.Context, id string, req models.UpdateTutorRequest) (models.Tutor, error) {
	args := m.Called(ctx, id, req)
	return args.Get(0).(models.Tutor), args.Error(1)
}
func (m *mockTutorRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestCreateTutor_Success(t *testing.T) {
	repo := new(mockTutorRepo)
	svc := service.NewTutorService(repo)

	req := models.CreateTutorRequest{
		FirstName: "Zhanibek",
		LastName:  "Gabitov",
	}
	expected := models.Tutor{
		ID:        "1",
		FirstName: "Zhanibek",
		LastName:  "Gabitov",
	}

	repo.On("Create", mock.Anything, req, "hashedPassTest").Return(expected, nil)
	tutor, err := svc.Create(context.Background(), req, "hashedPassTest")
	assert.NoError(t, err)
	assert.Equal(t, expected, tutor)
	repo.AssertExpectations(t)
}

func TestCreateTutor_Error(t *testing.T) {
	repo := new(mockTutorRepo)
	svc := service.NewTutorService(repo)

	req := models.CreateTutorRequest{
		FirstName: "Zhanibek",
		LastName:  "Gabitov",
	}

	repo.On("Create", mock.Anything, req, "hashedPassTest").Return(models.Tutor{}, errors.New("failed to create new tutor"))
	tutor, err := svc.Create(context.Background(), req, "hashedPassTest")
	assert.Error(t, err)
	assert.Empty(t, tutor)
	repo.AssertExpectations(t)

}

func TestGetAllTutors_Success(t *testing.T) {
	repo := new(mockTutorRepo)
	svc := service.NewTutorService(repo)

	expected := []models.Tutor{
		{ID: "1",
			FirstName: "Zhanibek",
			LastName:  "Gabitov",
		},
		{
			ID:        "2",
			FirstName: "Diana",
			LastName:  "Arslanova",
		},
	}
	repo.On("GetAll", mock.Anything).Return(expected, nil)
	tutors, err := svc.GetAll(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, expected, tutors)
	repo.AssertExpectations(t)

}
func TestGetAllTutors_Error(t *testing.T) {
	repo := new(mockTutorRepo)
	svc := service.NewTutorService(repo)

	repo.On("GetAll", mock.Anything).Return([]models.Tutor{}, errors.New("db error"))

	tutors, err := svc.GetAll(context.Background())

	assert.Error(t, err)
	assert.Empty(t, tutors)
	repo.AssertExpectations(t)
}

func TestDeleteTutor_Success(t *testing.T) {
	repo := new(mockTutorRepo)
	svc := service.NewTutorService(repo)

	repo.On("Delete", mock.Anything, "tutor-1").Return(nil)

	err := svc.Delete(context.Background(), "tutor-1")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
