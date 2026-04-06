package service_test

import (
	"errors"
	"testing"
	"time"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockPaymentRepo struct {
	mock.Mock
}

func (m *mockPaymentRepo) Create(req models.CreatePaymentRequest) (models.Payment, error) {
	args := m.Called(req)
	return args.Get(0).(models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetByCourse(courseID string) ([]models.Payment, error) {
	args := m.Called(courseID)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetBalance(courseID string) (int, error) {
	args := m.Called(courseID)
	return args.Int(0), args.Error(1)
}

func TestCreate_Success(t *testing.T) {
	repo := new(mockPaymentRepo)
	svc := service.NewPaymentService(repo)
	req := models.CreatePaymentRequest{
		CourseID:     "123",
		Amount:       5000,
		LessonsCount: 12,
		PaidAt:       time.Date(2001, time.September, 11, 0, 0, 0, 0, time.UTC),
	}

	expected := models.Payment{
		ID:           "1",
		CourseID:     "123",
		Amount:       5000,
		LessonsCount: 12,
		PaidAt:       time.Date(2001, time.September, 11, 0, 0, 0, 0, time.UTC),
	}
	repo.On("Create", req).Return(expected, nil)

	payment, err := svc.Create(req)

	assert.NoError(t, err)
	assert.Equal(t, expected, payment)
	repo.AssertExpectations(t)
}

func TestCreate_Error(t *testing.T) {
	repo := new(mockPaymentRepo)
	svc := service.NewPaymentService(repo)

	req := models.CreatePaymentRequest{
		CourseID:     "123",
		Amount:       5000,
		LessonsCount: 12,
		PaidAt:       time.Date(2001, time.September, 11, 0, 0, 0, 0, time.UTC),
	}
	repo.On("Create", req).Return(models.Payment{}, errors.New("failed to create payment"))
	payment, err := svc.Create(req)

	assert.Error(t, err)
	assert.Empty(t, payment)
	repo.AssertExpectations(t)
}
