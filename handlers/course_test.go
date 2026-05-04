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

func newCourseRouter(svc *mockCourseService, tutorID string) *gin.Engine {
	r := gin.New()
	h := handlers.NewCourseHandler(svc, slog.Default())
	r.Use(withTutorID(tutorID))
	r.GET("/courses", h.GetAll)
	r.POST("/courses", h.Create)
	r.GET("/courses/:id", h.GetByID)
	r.PUT("/courses/:id", h.Update)
	r.DELETE("/courses/:id", h.Delete)
	return r
}

var (
	testEndedAt          = func() *time.Time { t := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC); return &t }()
	testCreateCourseReq  = models.CreateCourseRequest{
		StudentID:      testStudentIDPtr,
		Subject:        "Mathematics",
		PricePerLesson: 5000,
		StartedAt:      time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		EndedAt:        testEndedAt,
	}
)

// GetAll

func TestCourseGetAll_Success(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	p := models.Pagination{Page: 1, Limit: 20}
	svc.On("GetAll", mock.Anything, testTutorID, p).Return([]models.Course{testCourse}, 1, nil)

	w := makeRequest(t, r, http.MethodGet, "/courses?page=1&limit=20", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.PagedResponse[models.Course]
	decodeJSON(t, w, &got)
	assert.Len(t, got.Data, 1)
	assert.Equal(t, 1, got.Total)
	svc.AssertExpectations(t)
}

func TestCourseGetAll_Unauthorized(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, "")

	w := makeRequest(t, r, http.MethodGet, "/courses", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "GetAll")
}

func TestCourseGetAll_ServiceError(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	p := models.Pagination{Page: 1, Limit: 20}
	svc.On("GetAll", mock.Anything, testTutorID, p).Return([]models.Course{}, 0, errors.New("db error"))

	w := makeRequest(t, r, http.MethodGet, "/courses?page=1&limit=20", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// Create

func TestCourseCreate_Success(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	svc.On("Create", mock.Anything, testCreateCourseReq, testTutorID).Return(testCourse, nil)

	w := makeRequest(t, r, http.MethodPost, "/courses", testCreateCourseReq)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestCourseCreate_ValidationError(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	// subject is required
	w := makeRequest(t, r, http.MethodPost, "/courses", map[string]any{
		"student_id":       testStudentID,
		"price_per_lesson": 5000,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Create")
}

func TestCourseCreate_ServiceError(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	svc.On("Create", mock.Anything, testCreateCourseReq, testTutorID).Return(models.Course{}, fmt.Errorf("student: %w", service.ErrNotFound))

	w := makeRequest(t, r, http.MethodPost, "/courses", testCreateCourseReq)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

// GetByID

func TestCourseGetByID_Success(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	svc.On("GetByID", mock.Anything, testCourseID, testTutorID).Return(testCourse, nil)

	w := makeRequest(t, r, http.MethodGet, "/courses/"+testCourseID, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestCourseGetByID_NotFound(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	svc.On("GetByID", mock.Anything, testCourseID, testTutorID).Return(models.Course{}, fmt.Errorf("course: %w", service.ErrNotFound))

	w := makeRequest(t, r, http.MethodGet, "/courses/"+testCourseID, nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

// Delete

func TestCourseDelete_Success(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	svc.On("Delete", mock.Anything, testCourseID, testTutorID).Return(nil)

	w := makeRequest(t, r, http.MethodDelete, "/courses/"+testCourseID, nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestCourseDelete_ServiceError(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	svc.On("Delete", mock.Anything, testCourseID, testTutorID).Return(fmt.Errorf("course has active lessons: %w", service.ErrConflict))

	w := makeRequest(t, r, http.MethodDelete, "/courses/"+testCourseID, nil)

	assert.Equal(t, http.StatusConflict, w.Code)
	svc.AssertExpectations(t)
}
