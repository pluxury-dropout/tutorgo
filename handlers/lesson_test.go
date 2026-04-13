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

func newLessonRouter(svc *mockLessonService, tutorID string) *gin.Engine {
	r := gin.New()
	h := handlers.NewLessonHandler(svc, slog.Default())
	r.Use(withTutorID(tutorID))
	r.GET("/lessons", h.GetByCourse)
	r.POST("/lessons", h.Create)
	r.GET("/lessons/:id", h.GetByID)
	r.PUT("/lessons/:id", h.Update)
	r.DELETE("/lessons/:id", h.Delete)
	return r
}

var testCreateLessonReq = models.CreateLessonRequest{
	CourseID:        testCourseID,
	ScheduledAt:     time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC),
	DurationMinutes: 60,
}

var testUpdateLessonReq = models.UpdateLessonRequest{
	ScheduledAt:     time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC),
	DurationMinutes: 90,
	Status:          "completed",
}

// GetByCourse

func TestLessonGetByCourse_Success(t *testing.T) {
	svc := new(mockLessonService)
	r := newLessonRouter(svc, testTutorID)

	svc.On("GetByCourse", testCourseID, testTutorID).Return([]models.Lesson{testLesson}, nil)

	w := makeRequest(t, r, http.MethodGet, "/lessons?course_id="+testCourseID, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestLessonGetByCourse_MissingCourseID(t *testing.T) {
	svc := new(mockLessonService)
	r := newLessonRouter(svc, testTutorID)

	w := makeRequest(t, r, http.MethodGet, "/lessons", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "GetByCourse")
}

func TestLessonGetByCourse_ServiceError(t *testing.T) {
	svc := new(mockLessonService)
	r := newLessonRouter(svc, testTutorID)

	svc.On("GetByCourse", testCourseID, testTutorID).Return([]models.Lesson{}, errors.New("course not found or access denied"))

	w := makeRequest(t, r, http.MethodGet, "/lessons?course_id="+testCourseID, nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// Create

func TestLessonCreate_Success(t *testing.T) {
	svc := new(mockLessonService)
	r := newLessonRouter(svc, testTutorID)

	svc.On("Create", testCreateLessonReq, testTutorID).Return(testLesson, nil)

	w := makeRequest(t, r, http.MethodPost, "/lessons", testCreateLessonReq)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestLessonCreate_ValidationError(t *testing.T) {
	svc := new(mockLessonService)
	r := newLessonRouter(svc, testTutorID)

	// course_id and scheduled_at are required
	w := makeRequest(t, r, http.MethodPost, "/lessons", map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Create")
}

func TestLessonCreate_ServiceError(t *testing.T) {
	svc := new(mockLessonService)
	r := newLessonRouter(svc, testTutorID)

	svc.On("Create", testCreateLessonReq, testTutorID).Return(models.Lesson{}, errors.New("course not found or access denied"))

	w := makeRequest(t, r, http.MethodPost, "/lessons", testCreateLessonReq)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// GetByID

func TestLessonGetByID_Success(t *testing.T) {
	svc := new(mockLessonService)
	r := newLessonRouter(svc, testTutorID)

	svc.On("GetByID", testLessonID, testTutorID).Return(testLesson, nil)

	w := makeRequest(t, r, http.MethodGet, "/lessons/"+testLessonID, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestLessonGetByID_NotFound(t *testing.T) {
	svc := new(mockLessonService)
	r := newLessonRouter(svc, testTutorID)

	svc.On("GetByID", testLessonID, testTutorID).Return(models.Lesson{}, errors.New("not found"))

	w := makeRequest(t, r, http.MethodGet, "/lessons/"+testLessonID, nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

// Update

func TestLessonUpdate_Success(t *testing.T) {
	svc := new(mockLessonService)
	r := newLessonRouter(svc, testTutorID)

	svc.On("Update", testLessonID, testUpdateLessonReq, testTutorID).Return(testLesson, nil)

	w := makeRequest(t, r, http.MethodPut, "/lessons/"+testLessonID, testUpdateLessonReq)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestLessonUpdate_NotFound(t *testing.T) {
	svc := new(mockLessonService)
	r := newLessonRouter(svc, testTutorID)

	svc.On("Update", testLessonID, testUpdateLessonReq, testTutorID).Return(models.Lesson{}, errors.New("lesson not found or access denied"))

	w := makeRequest(t, r, http.MethodPut, "/lessons/"+testLessonID, testUpdateLessonReq)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// Delete

func TestLessonDelete_Success(t *testing.T) {
	svc := new(mockLessonService)
	r := newLessonRouter(svc, testTutorID)

	svc.On("Delete", testLessonID, testTutorID).Return(nil)

	w := makeRequest(t, r, http.MethodDelete, "/lessons/"+testLessonID, nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestLessonDelete_NotFound(t *testing.T) {
	svc := new(mockLessonService)
	r := newLessonRouter(svc, testTutorID)

	svc.On("Delete", testLessonID, testTutorID).Return(errors.New("lesson not found or access denied"))

	w := makeRequest(t, r, http.MethodDelete, "/lessons/"+testLessonID, nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}
