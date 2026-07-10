package handlers_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"tutorgo/handlers"

	"github.com/gin-gonic/gin"
	lkauth "github.com/livekit/protocol/auth"
	livekit "github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/encoding/protojson"
)

func newCallRouter(svc *mockLessonService) *gin.Engine {
	return newCallRouterWithStudentSvc(svc, new(mockStudentService))
}

func newCallRouterWithStudentSvc(svc *mockLessonService, studentSvc *mockStudentService) *gin.Engine {
	r := gin.New()
	h := handlers.NewCallHandler(svc, slog.Default(), "http://livekit.test", "key", "secret", studentSvc)
	r.GET("/public/lessons/:id/room-status", h.GetRoomStatus)
	r.POST("/webhooks/livekit", h.LiveKitWebhook)
	auth := r.Group("/")
	auth.Use(withTutorID(testTutorID))
	auth.POST("/lessons/:id/start-room", h.StartRoom)
	auth.POST("/lessons/:id/end-room", h.EndRoom)
	student := r.Group("/")
	student.Use(withStudentID(testStudentID))
	student.POST("/student/lessons/:id/room-token", h.GetStudentToken)
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

// signedWebhookRequest builds a LiveKit webhook POST signed the same way LiveKit
// signs it: protojson body + an Authorization JWT whose sha256 claim matches the
// body. newCallRouter wires the handler with key="key", secret="secret".
func signedWebhookRequest(t *testing.T, event *livekit.WebhookEvent) *http.Request {
	t.Helper()
	body, err := protojson.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	token, err := lkauth.NewAccessToken("key", "secret").
		SetValidFor(5 * time.Minute).
		SetSha256(base64.StdEncoding.EncodeToString(sum[:])).
		ToJWT()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/webhooks/livekit", bytes.NewReader(body))
	req.Header.Set("Authorization", token)
	return req
}

func TestLiveKitWebhook_RoomFinished_EndsRoom(t *testing.T) {
	svc := new(mockLessonService)
	r := newCallRouter(svc)
	svc.On("EndRoomByID", mock.Anything, testLessonID).Return(nil)

	req := signedWebhookRequest(t, &livekit.WebhookEvent{
		Event: webhook.EventRoomFinished,
		Room:  &livekit.Room{Name: "lesson-" + testLessonID},
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t) // proves EndRoomByID was called with the parsed lessonID
}

func TestStudentToken_NotEnrolled_Forbidden(t *testing.T) {
	svc := new(mockLessonService)
	studentSvc := new(mockStudentService)
	r := newCallRouterWithStudentSvc(svc, studentSvc)

	studentSvc.On("EnrolledInLesson", mock.Anything, testStudentID, testLessonID).Return(false, nil)

	w := makeRequest(t, r, http.MethodPost, "/student/lessons/"+testLessonID+"/room-token", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
	studentSvc.AssertExpectations(t)
}

func TestStudentToken_Enrolled_Success(t *testing.T) {
	svc := new(mockLessonService)
	studentSvc := new(mockStudentService)
	r := newCallRouterWithStudentSvc(svc, studentSvc)

	studentSvc.On("EnrolledInLesson", mock.Anything, testStudentID, testLessonID).Return(true, nil)

	w := makeRequest(t, r, http.MethodPost, "/student/lessons/"+testLessonID+"/room-token", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	decodeJSON(t, w, &body)
	assert.NotEmpty(t, body["token"])
	assert.Equal(t, "lesson-"+testLessonID, body["room_name"])
	assert.Equal(t, "http://livekit.test", body["server_url"])

	verifier, err := lkauth.ParseAPIToken(body["token"])
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	_, grants, err := verifier.Verify("secret")
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}
	assert.Equal(t, "student-"+testStudentID, grants.Identity)

	studentSvc.AssertExpectations(t)
}

func TestLiveKitWebhook_BadSignature_Rejected(t *testing.T) {
	svc := new(mockLessonService)
	r := newCallRouter(svc)

	req := signedWebhookRequest(t, &livekit.WebhookEvent{
		Event: webhook.EventRoomFinished,
		Room:  &livekit.Room{Name: "lesson-" + testLessonID},
	})
	req.Header.Set("Authorization", "garbage") // tamper with the signature

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "EndRoomByID", mock.Anything, mock.Anything)
}
