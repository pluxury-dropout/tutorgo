package service_test

import (
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

func (m *mockTutorRepo) Create(req models.CreateTutorRequest, passwordHash string) (models.Tutor, error) {
	args := m.Called(req, passwordHash)
	return args.Get(0).(models.Tutor), args.Error(1)
}

func (m *mockTutorRepo) GetAll() ([]models.Tutor, error) {
	args := m.Called()
	return args.Get(0).([]models.Tutor), args.Error(1)
}

func (m *mockTutorRepo) GetByID(id string) (models.Tutor, error) {
	args := m.Called(id)
	return args.Get(0).(models.Tutor), args.Error(1)
}
func (m *mockTutorRepo) GetByEmail(email string) (string, string, error) {
	args := m.Called(email)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *mockTutorRepo) GetByPhone(phone string) (string, string, error) {
	args := m.Called(phone)
	return args.String(0), args.String(1), args.Error(2)
}
func (m *mockTutorRepo) Update(id string, req models.UpdateTutorRequest) (models.Tutor, error) {
	args := m.Called(id, req)
	return args.Get(0).(models.Tutor), args.Error(1)
}
func (m *mockTutorRepo) Delete(id string) error {
	args := m.Called(id)
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

	repo.On("Create", req, "hashedPassTest").Return(expected, nil)
	tutor, err := svc.Create(req, "hashedPassTest")
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

	repo.On("Create", req, "hashedPassTest").Return(models.Tutor{}, errors.New("failed to create new tutor"))
	tutor, err := svc.Create(req, "hashedPassTest")
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
	repo.On("GetAll").Return(expected, nil)
	tutors, err := svc.GetAll()
	assert.NoError(t, err)
	assert.Equal(t, expected, tutors)
	repo.AssertExpectations(t)

}
func TestGetAllTutors_Error(t *testing.T) {
	repo := new(mockTutorRepo)
	svc := service.NewTutorService(repo)

	repo.On("GetAll").Return([]models.Tutor{}, errors.New("db error"))

	tutors, err := svc.GetAll()

	assert.Error(t, err)
	assert.Empty(t, tutors)
	repo.AssertExpectations(t)
}

func TestDeleteTutor_Success(t *testing.T) {
	repo := new(mockTutorRepo)
	svc := service.NewTutorService(repo)

	repo.On("Delete", "tutor-1").Return(nil)

	err := svc.Delete("tutor-1")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
