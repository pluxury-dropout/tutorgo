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

var testCreateCourseReq = models.CreateCourseRequest{
	StudentID:      testStudentID,
	Subject:        "Mathematics",
	PricePerLesson: 5000,
	StartedAt:      time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	EndedAt:        time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
}

// GetAll

func TestCourseGetAll_Success(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	svc.On("GetAll", testTutorID).Return([]models.Course{testCourse}, nil)

	w := makeRequest(t, r, http.MethodGet, "/courses", nil)

	assert.Equal(t, http.StatusOK, w.Code)
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

	svc.On("GetAll", testTutorID).Return([]models.Course{}, errors.New("db error"))

	w := makeRequest(t, r, http.MethodGet, "/courses", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// Create

func TestCourseCreate_Success(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	svc.On("Create", testCreateCourseReq, testTutorID).Return(testCourse, nil)

	w := makeRequest(t, r, http.MethodPost, "/courses", testCreateCourseReq)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestCourseCreate_ValidationError(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	// subject is required
	w := makeRequest(t, r, http.MethodPost, "/courses", map[string]interface{}{
		"student_id":       testStudentID,
		"price_per_lesson": 5000,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Create")
}

func TestCourseCreate_ServiceError(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	svc.On("Create", testCreateCourseReq, testTutorID).Return(models.Course{}, errors.New("student not found or access denied"))

	w := makeRequest(t, r, http.MethodPost, "/courses", testCreateCourseReq)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// GetByID

func TestCourseGetByID_Success(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	svc.On("GetByID", testCourseID, testTutorID).Return(testCourse, nil)

	w := makeRequest(t, r, http.MethodGet, "/courses/"+testCourseID, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestCourseGetByID_NotFound(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	svc.On("GetByID", testCourseID, testTutorID).Return(models.Course{}, errors.New("not found"))

	w := makeRequest(t, r, http.MethodGet, "/courses/"+testCourseID, nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

// Delete

func TestCourseDelete_Success(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	svc.On("Delete", testCourseID, testTutorID).Return(nil)

	w := makeRequest(t, r, http.MethodDelete, "/courses/"+testCourseID, nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestCourseDelete_ServiceError(t *testing.T) {
	svc := new(mockCourseService)
	r := newCourseRouter(svc, testTutorID)

	svc.On("Delete", testCourseID, testTutorID).Return(errors.New("cannot delete a course with existing lessons"))

	w := makeRequest(t, r, http.MethodDelete, "/courses/"+testCourseID, nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}
