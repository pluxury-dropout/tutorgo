package handlers_test

import (
	"errors"
	"log/slog"
	"net/http"
	"testing"
	"tutorgo/handlers"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newCallRouter(svc *mockLessonService) *gin.Engine {
	r := gin.New()
	h := handlers.NewCallHandler(svc, slog.Default(), "http://livekit.test", "key", "secret")
	r.GET("/public/lessons/:id/guest-token", h.GetGuestToken)
	r.GET("/public/lessons/:id/room-status", h.GetRoomStatus)
	auth := r.Group("/")
	auth.Use(withTutorID(testTutorID))
	auth.POST("/lessons/:id/start-room", h.StartRoom)
	auth.POST("/lessons/:id/end-room", h.EndRoom)
	return r
}

func TestStartRoom_Success(t *testing.T) {
	svc := new(mockLessonService)
	r := newCallRouter(svc)

	svc.On("StartRoom", mock.Anything, testLessonID, testTutorID).Return(nil)

	w := makeRequest(t, r, http.MethodPost, "/lessons/"+testLessonID+"/start-room", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestStartRoom_NotFound(t *testing.T) {
	svc := new(mockLessonService)
	r := newCallRouter(svc)

	svc.On("StartRoom", mock.Anything, testLessonID, testTutorID).Return(errors.New("lesson not found"))

	w := makeRequest(t, r, http.MethodPost, "/lessons/"+testLessonID+"/start-room", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestGetGuestToken_LessonNotFound(t *testing.T) {
	svc := new(mockLessonService)
	r := newCallRouter(svc)

	svc.On("ExistsPublic", mock.Anything, testLessonID).Return(errors.New("not found"))

	w := makeRequest(t, r, http.MethodGet, "/public/lessons/"+testLessonID+"/guest-token", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestGetGuestToken_Success(t *testing.T) {
	svc := new(mockLessonService)
	r := newCallRouter(svc)

	svc.On("ExistsPublic", mock.Anything, testLessonID).Return(nil)

	w := makeRequest(t, r, http.MethodGet, "/public/lessons/"+testLessonID+"/guest-token", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestEndRoom_Success(t *testing.T) {
	svc := new(mockLessonService)
	r := newCallRouter(svc)
	svc.On("EndRoom", mock.Anything, testLessonID, testTutorID).Return(nil)
	w := makeRequest(t, r, http.MethodPost, "/lessons/"+testLessonID+"/end-room", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestEndRoom_NotFound(t *testing.T) {
	svc := new(mockLessonService)
	r := newCallRouter(svc)
	svc.On("EndRoom", mock.Anything, testLessonID, testTutorID).Return(errors.New("lesson not found"))
	w := makeRequest(t, r, http.MethodPost, "/lessons/"+testLessonID+"/end-room", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestGetRoomStatus_Active(t *testing.T) {
	svc := new(mockLessonService)
	r := newCallRouter(svc)
	svc.On("GetRoomStatus", mock.Anything, testLessonID).Return("active", nil)
	w := makeRequest(t, r, http.MethodGet, "/public/lessons/"+testLessonID+"/room-status", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	decodeJSON(t, w, &body)
	assert.Equal(t, "active", body["status"])
	svc.AssertExpectations(t)
}

func TestGetRoomStatus_NotFound(t *testing.T) {
	svc := new(mockLessonService)
	r := newCallRouter(svc)
	svc.On("GetRoomStatus", mock.Anything, testLessonID).Return("", errors.New("not found"))
	w := makeRequest(t, r, http.MethodGet, "/public/lessons/"+testLessonID+"/room-status", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}
