package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"tutorgo/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	lkauth "github.com/livekit/protocol/auth"
	livekit "github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

type quickRoom struct {
	tutorID string
}

type CallHandler struct {
	lessonService service.LessonService
	log           *slog.Logger
	livekitURL    string
	apiKey        string
	apiSecret     string
	roomClient    *lksdk.RoomServiceClient

	quickMu    sync.RWMutex
	quickRooms map[string]*quickRoom
}

func NewCallHandler(svc service.LessonService, log *slog.Logger, url, key, secret string) *CallHandler {
	var roomClient *lksdk.RoomServiceClient
	if key != "" {
		roomClient = lksdk.NewRoomServiceClient(url, key, secret)
	}
	return &CallHandler{
		lessonService: svc,
		log:           log,
		livekitURL:    url,
		apiKey:        key,
		apiSecret:     secret,
		roomClient:    roomClient,
		quickRooms:    make(map[string]*quickRoom),
	}
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
	at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
	grant := &lkauth.VideoGrant{
		RoomJoin:     true,
		Room:         roomName,
		CanPublish:   &canPublish,
		CanSubscribe: &canSubscribe,
	}
	at.SetVideoGrant(grant).
		SetIdentity("tutor-" + tutorID).
		SetName("Репетитор").
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

// GET /public/lessons/:id/guest-token — публичный, для учеников по ссылке
func (h *CallHandler) GetGuestToken(c *gin.Context) {
	if h.apiKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "video calls not configured"})
		return
	}

	lessonID := c.Param("id")
	if err := h.lessonService.ExistsPublic(c.Request.Context(), lessonID); err != nil {
		h.log.Error("lesson existence check failed", slog.String("lessonID", lessonID), slog.String("error", err.Error()))
		c.JSON(http.StatusNotFound, gin.H{"error": "lesson not found"})
		return
	}

	roomName := "lesson-" + lessonID

	canPublish := true
	canSubscribe := true
	identity := fmt.Sprintf("guest-%d", time.Now().UnixMilli())
	at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
	grant := &lkauth.VideoGrant{
		RoomJoin:     true,
		Room:         roomName,
		CanPublish:   &canPublish,
		CanSubscribe: &canSubscribe,
	}
	at.SetVideoGrant(grant).
		SetIdentity(identity).
		SetName("Ученик").
		SetValidFor(3 * time.Hour)

	token, err := at.ToJWT()
	if err != nil {
		h.log.Error("Failed to generate guest token", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"room_name":  roomName,
		"server_url": h.livekitURL,
	})
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
	at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
	grant := &lkauth.VideoGrant{
		RoomJoin:     true,
		Room:         roomName,
		CanPublish:   &canPublish,
		CanSubscribe: &canSubscribe,
	}
	at.SetVideoGrant(grant).
		SetIdentity("tutor-" + tutorID).
		SetName("Репетитор").
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
	at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
	grant := &lkauth.VideoGrant{
		RoomJoin:     true,
		Room:         roomName,
		CanPublish:   &canPublish,
		CanSubscribe: &canSubscribe,
	}
	at.SetVideoGrant(grant).
		SetIdentity(identity).
		SetName("Ученик").
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
