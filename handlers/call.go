package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"tutorgo/service"

	"github.com/gin-gonic/gin"
	lkauth "github.com/livekit/protocol/auth"
)

type CallHandler struct {
	lessonService service.LessonService
	log           *slog.Logger
	livekitURL    string
	apiKey        string
	apiSecret     string
}

func NewCallHandler(svc service.LessonService, log *slog.Logger, url, key, secret string) *CallHandler {
	return &CallHandler{lessonService: svc, log: log, livekitURL: url, apiKey: key, apiSecret: secret}
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
