package handlers_test

import (
	"errors"
	"net/http"
	"testing"
	"time"
	"tutorgo/handlers"
	"tutorgo/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"log/slog"
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

	svc.On("GetByCourse", testCourseID, testTutorID).Return([]models.Payment{testPayment}, nil)

	w := makeRequest(t, r, http.MethodGet, "/payments?course_id="+testCourseID, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestPaymentGetAll_MissingCourseID(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	w := makeRequest(t, r, http.MethodGet, "/payments", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "GetByCourse")
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

	svc.On("GetByCourse", testCourseID, testTutorID).Return([]models.Payment{}, errors.New("db error"))

	w := makeRequest(t, r, http.MethodGet, "/payments?course_id="+testCourseID, nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// Create

func TestPaymentCreate_Success(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	svc.On("Create", testCreatePaymentReq, testTutorID).Return(testPayment, nil)

	w := makeRequest(t, r, http.MethodPost, "/payments", testCreatePaymentReq)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestPaymentCreate_ValidationError(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	// amount is required
	w := makeRequest(t, r, http.MethodPost, "/payments", map[string]interface{}{
		"course_id":     testCourseID,
		"lessons_count": 10,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Create")
}

func TestPaymentCreate_ServiceError(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	svc.On("Create", testCreatePaymentReq, testTutorID).Return(models.Payment{}, errors.New("course not found or access denied"))

	w := makeRequest(t, r, http.MethodPost, "/payments", testCreatePaymentReq)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// GetBalance

func TestPaymentGetBalance_Success(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	expected := models.CourseBalance{LessonsPaid: 10, LessonsCompleted: 3, LessonsRemaining: 7}
	svc.On("GetBalance", testCourseID, testTutorID).Return(expected, nil)

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

	svc.On("GetBalance", testCourseID, testTutorID).Return(models.CourseBalance{}, errors.New("course not found or access denied"))

	w := makeRequest(t, r, http.MethodGet, "/payments/balance?course_id="+testCourseID, nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}
