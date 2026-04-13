package handlers_test

import (
	"errors"
	"net/http"
	"testing"
	"tutorgo/handlers"
	"tutorgo/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"log/slog"
)

func newTutorRouter(svc *mockTutorService, tutorID string) *gin.Engine {
	r := gin.New()
	h := handlers.NewTutorHandler(svc, slog.Default())
	r.Use(withTutorID(tutorID))
	r.GET("/tutors/:id", h.GetByID)
	r.PUT("/tutors/:id", h.Update)
	r.DELETE("/tutors/:id", h.Delete)
	return r
}

// GetByID

func TestTutorGetByID_Success(t *testing.T) {
	svc := new(mockTutorService)
	r := newTutorRouter(svc, testTutorID)

	svc.On("GetByID", testTutorID).Return(testTutor, nil)

	w := makeRequest(t, r, http.MethodGet, "/tutors/"+testTutorID, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.Tutor
	decodeJSON(t, w, &got)
	assert.Equal(t, testTutorID, got.ID)
	svc.AssertExpectations(t)
}

func TestTutorGetByID_Forbidden(t *testing.T) {
	svc := new(mockTutorService)
	r := newTutorRouter(svc, testTutorID)

	// Requesting someone else's profile
	w := makeRequest(t, r, http.MethodGet, "/tutors/other-tutor-id", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertNotCalled(t, "GetByID")
}

func TestTutorGetByID_NotFound(t *testing.T) {
	svc := new(mockTutorService)
	r := newTutorRouter(svc, testTutorID)

	svc.On("GetByID", testTutorID).Return(models.Tutor{}, errors.New("not found"))

	w := makeRequest(t, r, http.MethodGet, "/tutors/"+testTutorID, nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

// Update

func TestTutorUpdate_Success(t *testing.T) {
	svc := new(mockTutorService)
	r := newTutorRouter(svc, testTutorID)

	req := models.UpdateTutorRequest{
		Email:     "new@example.com",
		FirstName: "Amir",
		LastName:  "Bekov",
	}
	svc.On("Update", testTutorID, req).Return(testTutor, nil)

	w := makeRequest(t, r, http.MethodPut, "/tutors/"+testTutorID, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestTutorUpdate_Forbidden(t *testing.T) {
	svc := new(mockTutorService)
	r := newTutorRouter(svc, testTutorID)

	req := models.UpdateTutorRequest{Email: "new@example.com", FirstName: "Amir", LastName: "Bekov"}
	w := makeRequest(t, r, http.MethodPut, "/tutors/other-tutor-id", req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertNotCalled(t, "Update")
}

func TestTutorUpdate_ValidationError(t *testing.T) {
	svc := new(mockTutorService)
	r := newTutorRouter(svc, testTutorID)

	// email is required
	w := makeRequest(t, r, http.MethodPut, "/tutors/"+testTutorID, map[string]string{"first_name": "Amir"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Update")
}

// Delete

func TestTutorDelete_Success(t *testing.T) {
	svc := new(mockTutorService)
	r := newTutorRouter(svc, testTutorID)

	svc.On("Delete", testTutorID).Return(nil)

	w := makeRequest(t, r, http.MethodDelete, "/tutors/"+testTutorID, nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestTutorDelete_Forbidden(t *testing.T) {
	svc := new(mockTutorService)
	r := newTutorRouter(svc, testTutorID)

	w := makeRequest(t, r, http.MethodDelete, "/tutors/other-tutor-id", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertNotCalled(t, "Delete")
}

func TestTutorDelete_ServiceError(t *testing.T) {
	svc := new(mockTutorService)
	r := newTutorRouter(svc, testTutorID)

	svc.On("Delete", testTutorID).Return(errors.New("db error"))

	w := makeRequest(t, r, http.MethodDelete, "/tutors/"+testTutorID, nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}
