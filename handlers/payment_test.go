package handlers_test

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"
	"tutorgo/handlers"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newPaymentRouter(svc *mockPaymentService, tutorID string) *gin.Engine {
	r := gin.New()
	h := handlers.NewPaymentHandler(svc, slog.Default())
	r.Use(withTutorID(tutorID))
	r.GET("/payments", h.GetAll)
	r.POST("/payments", h.Create)
	r.GET("/payments/balance", h.GetBalance)
	return r
}

var testCreatePaymentReq = models.CreatePaymentRequest{
	CourseID:     testCourseID,
	Amount:       5000,
	LessonsCount: 10,
	PaidAt:       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
}

// GetAll

func TestPaymentGetAll_Success(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	p := models.Pagination{Page: 1, Limit: 20}
	svc.On("GetByCourse", mock.Anything, testCourseID, testTutorID, p).Return([]models.Payment{testPayment}, 1, nil)

	w := makeRequest(t, r, http.MethodGet, "/payments?course_id="+testCourseID+"&page=1&limit=20", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.PagedResponse[models.Payment]
	decodeJSON(t, w, &got)
	assert.Len(t, got.Data, 1)
	assert.Equal(t, 1, got.Total)
	svc.AssertExpectations(t)
}

func TestPaymentGetAll_AllTutor(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	p := models.Pagination{Page: 1, Limit: 20}
	svc.On("GetAllByTutorPaged", mock.Anything, testTutorID, p).Return([]models.Payment{testPayment}, 1, nil)

	w := makeRequest(t, r, http.MethodGet, "/payments?page=1&limit=20", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.PagedResponse[models.Payment]
	decodeJSON(t, w, &got)
	assert.Len(t, got.Data, 1)
	assert.Equal(t, 1, got.Total)
	svc.AssertExpectations(t)
}

func TestPaymentGetAll_Unauthorized(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, "")

	w := makeRequest(t, r, http.MethodGet, "/payments?course_id="+testCourseID, nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "GetByCourse")
}

func TestPaymentGetAll_ServiceError(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	p := models.Pagination{Page: 1, Limit: 20}
	svc.On("GetByCourse", mock.Anything, testCourseID, testTutorID, p).Return([]models.Payment{}, 0, errors.New("db error"))

	w := makeRequest(t, r, http.MethodGet, "/payments?course_id="+testCourseID+"&page=1&limit=20", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// Create

func TestPaymentCreate_Success(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	svc.On("Create", mock.Anything, testCreatePaymentReq, testTutorID).Return(testPayment, nil)

	w := makeRequest(t, r, http.MethodPost, "/payments", testCreatePaymentReq)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestPaymentCreate_ValidationError(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	// amount is required
	w := makeRequest(t, r, http.MethodPost, "/payments", map[string]any{
		"course_id":     testCourseID,
		"lessons_count": 10,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Create")
}

func TestPaymentCreate_ServiceError(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	svc.On("Create", mock.Anything, testCreatePaymentReq, testTutorID).Return(models.Payment{}, fmt.Errorf("course: %w", service.ErrNotFound))

	w := makeRequest(t, r, http.MethodPost, "/payments", testCreatePaymentReq)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

// GetBalance

func TestPaymentGetBalance_Success(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	expected := models.CourseBalance{LessonsPaid: 10, LessonsCompleted: 3, LessonsRemaining: 7}
	svc.On("GetBalance", mock.Anything, testCourseID, testTutorID).Return(expected, nil)

	w := makeRequest(t, r, http.MethodGet, "/payments/balance?course_id="+testCourseID, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.CourseBalance
	decodeJSON(t, w, &got)
	assert.Equal(t, expected, got)
	svc.AssertExpectations(t)
}

func TestPaymentGetBalance_MissingCourseID(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	w := makeRequest(t, r, http.MethodGet, "/payments/balance", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "GetBalance")
}

func TestPaymentGetBalance_ServiceError(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	svc.On("GetBalance", mock.Anything, testCourseID, testTutorID).Return(models.CourseBalance{}, fmt.Errorf("course: %w", service.ErrNotFound))

	w := makeRequest(t, r, http.MethodGet, "/payments/balance?course_id="+testCourseID, nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}
