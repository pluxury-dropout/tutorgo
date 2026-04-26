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

type mockPaymentRepo struct {
	mock.Mock
}

func (m *mockPaymentRepo) Create(ctx context.Context, req models.CreatePaymentRequest) (models.Payment, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetByCourse(ctx context.Context, courseID string) ([]models.Payment, error) {
	args := m.Called(ctx, courseID)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetAllByTutor(ctx context.Context, tutorID string, limit int) ([]models.Payment, error) {
	args := m.Called(ctx, tutorID, limit)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetBalance(ctx context.Context, courseID string) (models.CourseBalance, error) {
	args := m.Called(ctx, courseID)
	return args.Get(0).(models.CourseBalance), args.Error(1)
}

func (m *mockPaymentRepo) GetMonthlyIncome(ctx context.Context, tutorID string) (float64, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).(float64), args.Error(1)
}

var (
	tutorID  = "tutor-uuid-1"
	courseID = "course-uuid-1"

	paymentReq = models.CreatePaymentRequest{
		CourseID:     courseID,
		Amount:       5000,
		LessonsCount: 12,
		PaidAt:       time.Date(2001, time.September, 11, 0, 0, 0, 0, time.UTC),
	}

	expectedPayment = models.Payment{
		ID:           "payment-uuid-1",
		CourseID:     courseID,
		Amount:       5000,
		LessonsCount: 12,
		PaidAt:       time.Date(2001, time.September, 11, 0, 0, 0, 0, time.UTC),
	}

	expectedCourse = models.Course{
		ID:      courseID,
		TutorID: tutorID,
	}
)

func newPaymentSvc(payRepo *mockPaymentRepo, courseRepo *mockCourseRepo) service.PaymentService {
	return service.NewPaymentService(payRepo, courseRepo)
}

// Create

func TestPaymentCreate_Success(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	payRepo.On("Create", mock.Anything, paymentReq).Return(expectedPayment, nil)

	payment, err := svc.Create(context.Background(), paymentReq, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expectedPayment, payment)
	courseRepo.AssertExpectations(t)
	payRepo.AssertExpectations(t)
}

func TestPaymentCreate_CourseNotFound(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	payment, err := svc.Create(context.Background(), paymentReq, tutorID)

	assert.Error(t, err)
	assert.EqualError(t, err, "course not found or access denied")
	assert.Empty(t, payment)
	payRepo.AssertNotCalled(t, "Create")
	courseRepo.AssertExpectations(t)
}

func TestPaymentCreate_RepoError(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	payRepo.On("Create", mock.Anything, paymentReq).Return(models.Payment{}, errors.New("db error"))

	payment, err := svc.Create(context.Background(), paymentReq, tutorID)

	assert.Error(t, err)
	assert.Empty(t, payment)
	courseRepo.AssertExpectations(t)
	payRepo.AssertExpectations(t)
}

// GetByCourse

func TestPaymentGetByCourse_Success(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo)

	expected := []models.Payment{expectedPayment}
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	payRepo.On("GetByCourse", mock.Anything, courseID).Return(expected, nil)

	payments, err := svc.GetByCourse(context.Background(), courseID, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expected, payments)
	courseRepo.AssertExpectations(t)
	payRepo.AssertExpectations(t)
}

func TestPaymentGetByCourse_CourseNotFound(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	payments, err := svc.GetByCourse(context.Background(), courseID, tutorID)

	assert.Error(t, err)
	assert.EqualError(t, err, "course not found or access denied")
	assert.Nil(t, payments)
	payRepo.AssertNotCalled(t, "GetByCourse")
	courseRepo.AssertExpectations(t)
}

// GetBalance

func TestPaymentGetBalance_Success(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo)

	expected := models.CourseBalance{
		LessonsPaid:      10,
		LessonsCompleted: 3,
		LessonsRemaining: 7,
	}
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	payRepo.On("GetBalance", mock.Anything, courseID).Return(expected, nil)

	balance, err := svc.GetBalance(context.Background(), courseID, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expected, balance)
	courseRepo.AssertExpectations(t)
	payRepo.AssertExpectations(t)
}

func TestPaymentGetBalance_CourseNotFound(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	balance, err := svc.GetBalance(context.Background(), courseID, tutorID)

	assert.Error(t, err)
	assert.EqualError(t, err, "course not found or access denied")
	assert.Empty(t, balance)
	payRepo.AssertNotCalled(t, "GetBalance")
	courseRepo.AssertExpectations(t)
}
