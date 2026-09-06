package handlers_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"tutorgo/handlers"
	"tutorgo/models"

	"github.com/gin-gonic/gin"
	lkauth "github.com/livekit/protocol/auth"
	livekit "github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/encoding/protojson"
)

// fakeLiveKit — двойник LiveKit RoomService: комнаты в мапе, как их держит
// настоящий сервер. Важно, что состояние живёт ЗДЕСЬ, а не в хендлере: только
// так можно собрать второй хендлер «после редеплоя» поверх тех же комнат.
type fakeLiveKit struct {
	mu    sync.Mutex
	rooms map[string]*livekit.Room
	err   error // подставляется, чтобы проверить реакцию на недоступный LiveKit
}

func newFakeLiveKit() *fakeLiveKit {
	return &fakeLiveKit{rooms: make(map[string]*livekit.Room)}
}

func (f *fakeLiveKit) CreateRoom(_ context.Context, req *livekit.CreateRoomRequest) (*livekit.Room, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	room := &livekit.Room{Name: req.GetName(), Metadata: req.GetMetadata()}
	f.rooms[req.GetName()] = room
	return room, nil
}

func (f *fakeLiveKit) ListRooms(_ context.Context, req *livekit.ListRoomsRequest) (*livekit.ListRoomsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	res := &livekit.ListRoomsResponse{}
	for _, name := range req.GetNames() {
		if room, ok := f.rooms[name]; ok {
			res.Rooms = append(res.Rooms, room)
		}
	}
	return res, nil
}

func (f *fakeLiveKit) DeleteRoom(_ context.Context, req *livekit.DeleteRoomRequest) (*livekit.DeleteRoomResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	delete(f.rooms, req.GetRoom())
	return &livekit.DeleteRoomResponse{}, nil
}

func newCallRouter(svc *mockLessonService) *gin.Engine {
	return newCallRouterWithStudentSvc(svc, new(mockStudentService), new(mockTutorService))
}

func newCallRouterWithStudentSvc(svc *mockLessonService, studentSvc *mockStudentService, tutorSvc *mockTutorService) *gin.Engine {
	return newCallRouterOn(newFakeLiveKit(), testTutorID, svc, studentSvc, tutorSvc)
}

// newCallRouterOn собирает роутер поверх заданного LiveKit и заданного тьютора.
// Отдельный конструктор нужен двум сценариям: «тот же LiveKit, новый процесс»
// и «та же комната, чужой тьютор».
func newCallRouterOn(lk *fakeLiveKit, tutorID string, svc *mockLessonService, studentSvc *mockStudentService, tutorSvc *mockTutorService) *gin.Engine {
	r := gin.New()
	h := handlers.NewCallHandler(svc, slog.Default(), "http://livekit.test", "key", "secret", studentSvc, tutorSvc)
	h.SetRoomAPI(lk)
	r.GET("/public/lessons/:id/room-status", h.GetRoomStatus)
	r.POST("/webhooks/livekit", h.LiveKitWebhook)
	auth := r.Group("/")
	auth.Use(withTutorID(tutorID))
	auth.POST("/lessons/:id/start-room", h.StartRoom)
	auth.POST("/lessons/:id/end-room", h.EndRoom)
	auth.POST("/lessons/:id/room-token", h.GetToken)
	student := r.Group("/")
	student.Use(withStudentID(testStudentID))
	student.POST("/student/lessons/:id/room-token", h.GetStudentToken)
	auth.POST("/calls/quick", h.StartQuickRoom)
	auth.POST("/calls/quick/:id/end", h.EndQuickRoom)
	r.GET("/public/quick/:id/status", h.GetQuickRoomStatus)
	r.GET("/public/quick/:id/guest-token", h.GetQuickGuestToken)
	return r
}

// quickTutorSvc — мок профиля, без которого StartQuickRoom не соберёт имя в токен.
func quickTutorSvc() *mockTutorService {
	svc := new(mockTutorService)
	svc.On("GetByID", mock.Anything, mock.Anything).Return(models.Tutor{ID: testTutorID}, nil).Maybe()
	return svc
}

// startQuickRoom поднимает пробную комнату и отдаёт её id: гостевой токен
// выдаётся только на комнату, существующую в LiveKit.
func startQuickRoom(t *testing.T, r *gin.Engine) string {
	t.Helper()
	w := makeRequest(t, r, http.MethodPost, "/calls/quick", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("start quick room: got %d", w.Code)
	}
	var body map[string]string
	decodeJSON(t, w, &body)
	return body["room_id"]
}

func quickGuestName(t *testing.T, r *gin.Engine, roomID, query string) string {
	t.Helper()
	w := makeRequest(t, r, http.MethodGet, "/public/quick/"+roomID+"/guest-token"+query, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("guest token: got %d", w.Code)
	}
	var body map[string]string
	decodeJSON(t, w, &body)
	verifier, err := lkauth.ParseAPIToken(body["token"])
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	_, grants, err := verifier.Verify("secret")
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}
	return grants.Name
}

// Регрессия, ради которой состояние переехало в LiveKit: пробная комната
// пережила редеплой. Второй роутер — это новый процесс поверх того же LiveKit;
// раньше он терял мапу, и гость получал 404 «room not found or ended» посреди
// идущего урока.
func TestQuickRoom_SurvivesProcessRestart(t *testing.T) {
	lk := newFakeLiveKit()
	before := newCallRouterOn(lk, testTutorID, new(mockLessonService), new(mockStudentService), quickTutorSvc())
	roomID := startQuickRoom(t, before)

	after := newCallRouterOn(lk, testTutorID, new(mockLessonService), new(mockStudentService), quickTutorSvc())

	w := makeRequest(t, after, http.MethodGet, "/public/quick/"+roomID+"/status", nil)
	assert.Equal(t, http.StatusOK, w.Code, "статус комнаты не должен зависеть от процесса")
	var status map[string]string
	decodeJSON(t, w, &status)
	assert.Equal(t, "active", status["status"])

	w = makeRequest(t, after, http.MethodGet, "/public/quick/"+roomID+"/guest-token?name=Гость", nil)
	assert.Equal(t, http.StatusOK, w.Code, "гость должен входить в комнату и после рестарта")

	// Завершение тоже: раньше EndQuickRoom отдавал 404 и комната оставалась в LiveKit.
	w = makeRequest(t, after, http.MethodPost, "/calls/quick/"+roomID+"/end", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	w = makeRequest(t, after, http.MethodGet, "/public/quick/"+roomID+"/status", nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "после завершения — ended")
}

// Недоступный LiveKit — это не «урок закончился». Фронт приравнивает 404 к концу
// урока и уводит гостя на финальный экран без возврата, поэтому сбой обязан
// приходить как 503: на нём гость остаётся ждать и продолжает поллить.
func TestQuickRoom_LiveKitDown_IsNotEnded(t *testing.T) {
	lk := newFakeLiveKit()
	r := newCallRouterOn(lk, testTutorID, new(mockLessonService), new(mockStudentService), quickTutorSvc())
	roomID := startQuickRoom(t, r)

	lk.err = errors.New("livekit unreachable")

	w := makeRequest(t, r, http.MethodGet, "/public/quick/"+roomID+"/status", nil)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	w = makeRequest(t, r, http.MethodGet, "/public/quick/"+roomID+"/guest-token?name=Гость", nil)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// Владелец комнаты теперь берётся из metadata в LiveKit, а не из мапы — проверяем,
// что проверка прав от этого не потерялась.
func TestEndQuickRoom_ForeignTutor_Forbidden(t *testing.T) {
	lk := newFakeLiveKit()
	owner := newCallRouterOn(lk, testTutorID, new(mockLessonService), new(mockStudentService), quickTutorSvc())
	roomID := startQuickRoom(t, owner)

	const otherTutorID = "33333333-3333-3333-3333-333333333333"
	stranger := newCallRouterOn(lk, otherTutorID, new(mockLessonService), new(mockStudentService), quickTutorSvc())

	w := makeRequest(t, stranger, http.MethodPost, "/calls/quick/"+roomID+"/end", nil)
	assert.Equal(t, http.StatusForbidden, w.Code)

	w = makeRequest(t, owner, http.MethodGet, "/public/quick/"+roomID+"/status", nil)
	assert.Equal(t, http.StatusOK, w.Code, "чужой запрос не должен был закрыть комнату")
}

// Без canUpdateOwnMetadata LiveKit отклоняет setMetadata, которым препод
// анонсирует открытую доску, — гость, зашедший позже, её не увидит.
func TestQuickRoomToken_AllowsMetadataUpdate(t *testing.T) {
	tutorSvc := new(mockTutorService)
	tutorSvc.On("GetByID", mock.Anything, testTutorID).Return(models.Tutor{ID: testTutorID}, nil)
	r := newCallRouterWithStudentSvc(new(mockLessonService), new(mockStudentService), tutorSvc)

	w := makeRequest(t, r, http.MethodPost, "/calls/quick", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	decodeJSON(t, w, &body)

	verifier, err := lkauth.ParseAPIToken(body["token"])
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	_, grants, err := verifier.Verify("secret")
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}
	if assert.NotNil(t, grants.Video.CanUpdateOwnMetadata) {
		assert.True(t, *grants.Video.CanUpdateOwnMetadata)
	}
}

func TestQuickGuestToken_CarriesTypedName(t *testing.T) {
	tutorSvc := new(mockTutorService)
	tutorSvc.On("GetByID", mock.Anything, testTutorID).Return(models.Tutor{ID: testTutorID}, nil)
	r := newCallRouterWithStudentSvc(new(mockLessonService), new(mockStudentService), tutorSvc)
	roomID := startQuickRoom(t, r)

	assert.Equal(t, "Айгерим", quickGuestName(t, r, roomID, "?name=%D0%90%D0%B9%D0%B3%D0%B5%D1%80%D0%B8%D0%BC"))
	assert.Equal(t, "Ученик", quickGuestName(t, r, roomID, ""), "без имени остаётся роль")
	assert.Equal(t, "Ученик", quickGuestName(t, r, roomID, "?name=%20%20"), "пробелы — не имя")
	assert.Len(t, []rune(quickGuestName(t, r, roomID, "?name="+strings.Repeat("a", 100))), 40, "длину режем")
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
	r := newCallRouterWithStudentSvc(svc, studentSvc, new(mockTutorService))

	studentSvc.On("EnrolledInLesson", mock.Anything, testStudentID, testLessonID).Return(false, nil)

	w := makeRequest(t, r, http.MethodPost, "/student/lessons/"+testLessonID+"/room-token", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
	studentSvc.AssertExpectations(t)
}

func TestStudentToken_Enrolled_Success(t *testing.T) {
	svc := new(mockLessonService)
	studentSvc := new(mockStudentService)
	r := newCallRouterWithStudentSvc(svc, studentSvc, new(mockTutorService))

	studentSvc.On("EnrolledInLesson", mock.Anything, testStudentID, testLessonID).Return(true, nil)
	studentSvc.On("GetProfile", mock.Anything, testStudentID).
		Return(models.StudentProfile{ID: testStudentID, FirstName: "Иван", LastName: "Петров"}, nil)

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
	// Имя в токене — то, что панель участников покажет сразу при подключении,
	// не дожидаясь первого курсора с доски.
	assert.Equal(t, "Иван Петров", grants.Name)

	studentSvc.AssertExpectations(t)
}

func TestTutorToken_CarriesRealName(t *testing.T) {
	svc := new(mockLessonService)
	tutorSvc := new(mockTutorService)
	r := newCallRouterWithStudentSvc(svc, new(mockStudentService), tutorSvc)

	svc.On("GetByID", mock.Anything, testLessonID, testTutorID).Return(models.Lesson{ID: testLessonID}, nil)
	tutorSvc.On("GetByID", mock.Anything, testTutorID).
		Return(models.Tutor{ID: testTutorID, FirstName: "Мария", LastName: "Соколова"}, nil)

	w := makeRequest(t, r, http.MethodPost, "/lessons/"+testLessonID+"/room-token", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	decodeJSON(t, w, &body)

	verifier, err := lkauth.ParseAPIToken(body["token"])
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	_, grants, err := verifier.Verify("secret")
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}
	assert.Equal(t, "tutor-"+testTutorID, grants.Identity)
	assert.Equal(t, "Мария Соколова", grants.Name)

	tutorSvc.AssertExpectations(t)
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
