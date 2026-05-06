package handlers_test

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"tutorgo/handlers"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newStudentRouter(svc *mockStudentService, tutorID string) *gin.Engine {
	r := gin.New()
	h := handlers.NewStudentHandler(svc, slog.Default())
	r.Use(withTutorID(tutorID))
	r.GET("/students", h.GetAll)
	r.POST("/students", h.Create)
	r.GET("/students/:id", h.GetByID)
	r.PUT("/students/:id", h.Update)
	r.DELETE("/students/:id", h.Delete)
	return r
}

// GetAll

func TestStudentGetAll_Success(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	p := models.Pagination{Page: 1, Limit: 20}
	expected := []models.Student{testStudent}
	svc.On("GetAll", mock.Anything, testTutorID, p).Return(expected, 1, nil)

	w := makeRequest(t, r, http.MethodGet, "/students?page=1&limit=20", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.PagedResponse[models.Student]
	decodeJSON(t, w, &got)
	assert.Len(t, got.Data, 1)
	assert.Equal(t, 1, got.Total)
	svc.AssertExpectations(t)
}

func TestStudentGetAll_Unauthorized(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, "")

	w := makeRequest(t, r, http.MethodGet, "/students", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "GetAll")
}

func TestStudentGetAll_ServiceError(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	p := models.Pagination{Page: 1, Limit: 20}
	svc.On("GetAll", mock.Anything, testTutorID, p).Return([]models.Student{}, 0, errors.New("db error"))

	w := makeRequest(t, r, http.MethodGet, "/students?page=1&limit=20", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestStudentGetAll_WithSearch(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	p := models.Pagination{Page: 1, Limit: 20, Search: "Aiya"}
	expected := []models.Student{testStudent}
	svc.On("GetAll", mock.Anything, testTutorID, p).Return(expected, 1, nil)

	w := makeRequest(t, r, http.MethodGet, "/students?page=1&limit=20&search=Aiya", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.PagedResponse[models.Student]
	decodeJSON(t, w, &got)
	assert.Len(t, got.Data, 1)
	assert.Equal(t, 1, got.Total)
	svc.AssertExpectations(t)
}

// Create

func TestStudentCreate_Success(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	req := models.CreateStudentRequest{FirstName: "Aiya", LastName: "Bekova"}
	svc.On("Create", mock.Anything, req, testTutorID).Return(testStudent, nil)

	w := makeRequest(t, r, http.MethodPost, "/students", req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got models.Student
	decodeJSON(t, w, &got)
	assert.Equal(t, testStudent.ID, got.ID)
	svc.AssertExpectations(t)
}

func TestStudentCreate_InvalidJSON(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	w := makeRequest(t, r, http.MethodPost, "/students", "not-json")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Create")
}

func TestStudentCreate_ValidationError(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	// first_name is required with min=2
	w := makeRequest(t, r, http.MethodPost, "/students", map[string]string{"first_name": "A"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Create")
}

func TestStudentCreate_ServiceError(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	req := models.CreateStudentRequest{FirstName: "Aiya", LastName: "Bekova"}
	svc.On("Create", mock.Anything, req, testTutorID).Return(models.Student{}, errors.New("db error"))

	w := makeRequest(t, r, http.MethodPost, "/students", req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// GetByID

func TestStudentGetByID_Success(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	svc.On("GetByID", mock.Anything, testStudentID, testTutorID).Return(testStudent, nil)

	w := makeRequest(t, r, http.MethodGet, "/students/"+testStudentID, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.Student
	decodeJSON(t, w, &got)
	assert.Equal(t, testStudentID, got.ID)
	svc.AssertExpectations(t)
}

func TestStudentGetByID_NotFound(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	svc.On("GetByID", mock.Anything, testStudentID, testTutorID).Return(models.Student{}, fmt.Errorf("student: %w", service.ErrNotFound))

	w := makeRequest(t, r, http.MethodGet, "/students/"+testStudentID, nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

// Update

func TestStudentUpdate_Success(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	req := models.UpdateStudentRequest{FirstName: "Aiya", LastName: "Bekova"}
	updated := models.Student{ID: testStudentID, FirstName: "Aiya", LastName: "Bekova"}
	svc.On("Update", mock.Anything, testStudentID, testTutorID, req).Return(updated, nil)

	w := makeRequest(t, r, http.MethodPut, "/students/"+testStudentID, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestStudentUpdate_ServiceError(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	req := models.UpdateStudentRequest{FirstName: "Aiya", LastName: "Bekova"}
	svc.On("Update", mock.Anything, testStudentID, testTutorID, req).Return(models.Student{}, errors.New("db error"))

	w := makeRequest(t, r, http.MethodPut, "/students/"+testStudentID, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// Delete

func TestStudentDelete_Success(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	svc.On("Delete", mock.Anything, testStudentID, testTutorID).Return(nil)

	w := makeRequest(t, r, http.MethodDelete, "/students/"+testStudentID, nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestStudentDelete_ServiceError(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	svc.On("Delete", mock.Anything, testStudentID, testTutorID).Return(errors.New("db error"))

	w := makeRequest(t, r, http.MethodDelete, "/students/"+testStudentID, nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// Body Size Limit

func TestStudentCreate_BodyTooLarge(t *testing.T) {
	svc := new(mockStudentService)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10)
		c.Next()
	})
	h := handlers.NewStudentHandler(svc, slog.Default())
	r.Use(withTutorID(testTutorID))
	r.POST("/students", h.Create)

	body := strings.NewReader(`{"first_name":"Aiya","last_name":"Bekova"}`)
	req := httptest.NewRequest(http.MethodPost, "/students", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error", "response should contain error field")
	svc.AssertNotCalled(t, "Create")
}
