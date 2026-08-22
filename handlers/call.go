package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"tutorgo/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	lkauth "github.com/livekit/protocol/auth"
	livekit "github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/webhook"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

type quickRoom struct {
	tutorID string
}

type CallHandler struct {
	lessonService  service.LessonService
	studentService service.StudentService
	tutorService   service.TutorService
	log            *slog.Logger
	livekitURL     string
	apiKey         string
	apiSecret      string
	roomClient     *lksdk.RoomServiceClient

	quickMu    sync.RWMutex
	quickRooms map[string]*quickRoom
}

func NewCallHandler(svc service.LessonService, log *slog.Logger, url, key, secret string, studentSvc service.StudentService, tutorSvc service.TutorService) *CallHandler {
	var roomClient *lksdk.RoomServiceClient
	if key != "" {
		roomClient = lksdk.NewRoomServiceClient(url, key, secret)
	}
	return &CallHandler{
		lessonService:  svc,
		studentService: studentSvc,
		tutorService:   tutorSvc,
		log:            log,
		livekitURL:     url,
		apiKey:         key,
		apiSecret:      secret,
		roomClient:     roomClient,
		quickRooms:     make(map[string]*quickRoom),
	}
}

// displayName склеивает имя для LiveKit-токена. Имя из токена LiveKit
// раздаёт всем участникам в момент join'а — без него панель участников до
// первого движения мыши показывает роль («Ученик», «Репетитор»): настоящее имя
// иначе доезжает только с cursor-сообщением доски. Профиль не прочитался или
// пуст — остаётся роль.
func displayName(first, last, role string) string {
	if full := strings.TrimSpace(first + " " + last); full != "" {
		return full
	}
	return role
}

// guestDisplayName — подпись анонимного гостя пробного урока. Ввод публичный и
// показывается другим участникам, поэтому режем по длине; пустой → роль.
func guestDisplayName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "Ученик"
	}
	if r := []rune(name); len(r) > 40 {
		return string(r[:40])
	}
	return name
}

// tutorName — имя репетитора для токена; ошибку глотаем, звонок важнее подписи.
func (h *CallHandler) tutorName(c *gin.Context, tutorID string) string {
	if h.tutorService == nil {
		return "Репетитор"
	}
	t, err := h.tutorService.GetByID(c.Request.Context(), tutorID)
	if err != nil {
		return "Репетитор"
	}
	return displayName(t.FirstName, t.LastName, "Репетитор")
}

// POST /lessons/:id/room-token — защищённый, только для репетитора
func (h *CallHandler) GetToken(c *gin.Context) {
	if h.apiKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "video calls not configured"})
		return
	}

	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	lessonID := c.Param("id")
	_, err := h.lessonService.GetByID(c.Request.Context(), lessonID, tutorID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "lesson not found"})
		return
	}

	roomName := "lesson-" + lessonID
	canPublish := true
	canSubscribe := true
	canUpdateMeta := true
	at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
	grant := &lkauth.VideoGrant{
		RoomJoin:             true,
		Room:                 roomName,
		CanPublish:           &canPublish,
		CanSubscribe:         &canSubscribe,
		CanUpdateOwnMetadata: &canUpdateMeta,
	}
	at.SetVideoGrant(grant).
		SetIdentity("tutor-" + tutorID).
		SetName(h.tutorName(c, tutorID)).
		SetValidFor(3 * time.Hour)

	token, err := at.ToJWT()
	if err != nil {
		h.log.Error("Failed to generate tutor token", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"room_name":  roomName,
		"server_url": h.livekitURL,
	})
}

// POST /student/lessons/:id/room-token — только для записанного залогиненного ученика
func (h *CallHandler) GetStudentToken(c *gin.Context) {
	if h.apiKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "video calls not configured"})
		return
	}
	studentID := c.GetString("studentID")
	if studentID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	lessonID := c.Param("id")
	ok, err := h.studentService.EnrolledInLesson(c.Request.Context(), studentID, lessonID)
	if err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "not enrolled in this lesson"})
		return
	}
	roomName := "lesson-" + lessonID
	name := "Ученик"
	if p, err := h.studentService.GetProfile(c.Request.Context(), studentID); err == nil {
		name = displayName(p.FirstName, p.LastName, name)
	}
	canPublish, canSubscribe := true, true
	at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
	at.SetVideoGrant(&lkauth.VideoGrant{
		RoomJoin: true, Room: roomName,
		CanPublish: &canPublish, CanSubscribe: &canSubscribe,
	}).SetIdentity("student-" + studentID).SetName(name).SetValidFor(3 * time.Hour)
	token, err := at.ToJWT()
	if err != nil {
		h.log.Error("Failed to generate student token", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "room_name": roomName, "server_url": h.livekitURL})
}

func (h *CallHandler) StartRoom(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	lessonID := c.Param("id")
	err := h.lessonService.StartRoom(c.Request.Context(), lessonID, tutorID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "lesson not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "room started"})
}

// POST /lessons/:id/end-room
func (h *CallHandler) EndRoom(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	lessonID := c.Param("id")
	if err := h.lessonService.EndRoom(c.Request.Context(), lessonID, tutorID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "lesson not found"})
		return
	}
	if h.roomClient != nil {
		roomName := "lesson-" + lessonID
		_, _ = h.roomClient.DeleteRoom(c.Request.Context(), &livekit.DeleteRoomRequest{Room: roomName})
	}
	c.JSON(http.StatusOK, gin.H{"message": "room ended"})
}

// POST /webhooks/livekit — public; LiveKit posts room lifecycle events here.
// No JWT: the request is authenticated by its signature, verified with our
// LiveKit API key/secret. LiveKit fires "room_finished" once a room has been
// empty for its empty_timeout (e.g. after the last tab closes), which is how a
// call that was never explicitly ended still gets closed in our DB.
func (h *CallHandler) LiveKitWebhook(c *gin.Context) {
	if h.apiKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "video calls not configured"})
		return
	}

	event, err := webhook.ReceiveWebhookEvent(c.Request, lkauth.NewSimpleKeyProvider(h.apiKey, h.apiSecret))
	if err != nil {
		h.log.Warn("Rejected LiveKit webhook", slog.String("error", err.Error()))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid webhook"})
		return
	}

	// We only act on a room shutting down. Scheduled-lesson rooms are named
	// "lesson-<lessonID>"; quick rooms ("quick-") have no DB row, so skip them.
	if event.GetEvent() != webhook.EventRoomFinished {
		c.Status(http.StatusOK)
		return
	}
	lessonID, ok := strings.CutPrefix(event.GetRoom().GetName(), "lesson-")
	if !ok {
		c.Status(http.StatusOK)
		return
	}
	if err := h.lessonService.EndRoomByID(c.Request.Context(), lessonID); err != nil {
		h.log.Error("Failed to end room from webhook",
			slog.String("lessonID", lessonID), slog.String("error", err.Error()))
		c.Status(http.StatusInternalServerError) // 5xx → LiveKit redelivers; EndRoomByID is idempotent
		return
	}
	c.Status(http.StatusOK)
}

// GET /public/lessons/:id/room-status
func (h *CallHandler) GetRoomStatus(c *gin.Context) {
	lessonID := c.Param("id")
	status, err := h.lessonService.GetRoomStatus(c.Request.Context(), lessonID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "lesson not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

// POST /calls/quick
func (h *CallHandler) StartQuickRoom(c *gin.Context) {
	if h.apiKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "video calls not configured"})
		return
	}
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	roomID := uuid.New().String()
	roomName := "quick-" + roomID

	canPublish := true
	canSubscribe := true
	// Тем же грантом, что и на обычном уроке: открытие доски препод анонсирует
	// через setMetadata, а LiveKit без canUpdateOwnMetadata его отклоняет —
	// гость, зашедший после анонса, доски не увидит.
	canUpdateMeta := true
	at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
	grant := &lkauth.VideoGrant{
		RoomJoin:             true,
		Room:                 roomName,
		CanPublish:           &canPublish,
		CanSubscribe:         &canSubscribe,
		CanUpdateOwnMetadata: &canUpdateMeta,
	}
	at.SetVideoGrant(grant).
		SetIdentity("tutor-" + tutorID).
		SetName(h.tutorName(c, tutorID)).
		SetValidFor(3 * time.Hour)

	token, err := at.ToJWT()
	if err != nil {
		h.log.Error("Failed to generate quick room token", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// Only insert if token generation succeeded
	h.quickMu.Lock()
	h.quickRooms[roomID] = &quickRoom{tutorID: tutorID}
	h.quickMu.Unlock()

	// Auto-evict after token validity window (3h)
	time.AfterFunc(3*time.Hour, func() {
		h.quickMu.Lock()
		delete(h.quickRooms, roomID)
		h.quickMu.Unlock()
	})

	c.JSON(http.StatusOK, gin.H{
		"room_id":    roomID,
		"token":      token,
		"server_url": h.livekitURL,
	})
}

// POST /calls/quick/:id/end
func (h *CallHandler) EndQuickRoom(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	roomID := c.Param("id")

	h.quickMu.Lock()
	room, ok := h.quickRooms[roomID]
	if ok {
		if room.tutorID != tutorID {
			h.quickMu.Unlock()
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		delete(h.quickRooms, roomID)
	}
	h.quickMu.Unlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	if h.roomClient != nil {
		roomName := "quick-" + roomID
		_, _ = h.roomClient.DeleteRoom(c.Request.Context(), &livekit.DeleteRoomRequest{Room: roomName})
	}

	c.JSON(http.StatusOK, gin.H{"message": "room ended"})
}

// GET /public/quick/:id/status
func (h *CallHandler) GetQuickRoomStatus(c *gin.Context) {
	roomID := c.Param("id")

	h.quickMu.RLock()
	_, ok := h.quickRooms[roomID]
	h.quickMu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"status": "ended"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "active"})
}

// GET /public/quick/:id/guest-token
func (h *CallHandler) GetQuickGuestToken(c *gin.Context) {
	if h.apiKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "video calls not configured"})
		return
	}

	roomID := c.Param("id")

	h.quickMu.RLock()
	_, ok := h.quickRooms[roomID]
	h.quickMu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found or ended"})
		return
	}

	roomName := "quick-" + roomID
	canPublish := true
	canSubscribe := true
	identity := fmt.Sprintf("guest-%d", time.Now().UnixMilli())
	// Имя гость ввёл в форме входа — оно единственный источник подписи: аккаунта
	// у него нет (на то и пробный урок), профиль читать неоткуда.
	name := guestDisplayName(c.Query("name"))
	at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
	grant := &lkauth.VideoGrant{
		RoomJoin:     true,
		Room:         roomName,
		CanPublish:   &canPublish,
		CanSubscribe: &canSubscribe,
	}
	at.SetVideoGrant(grant).
		SetIdentity(identity).
		SetName(name).
		SetValidFor(3 * time.Hour)

	token, err := at.ToJWT()
	if err != nil {
		h.log.Error("Failed to generate quick guest token", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"room_name":  roomName,
		"server_url": h.livekitURL,
	})
}
