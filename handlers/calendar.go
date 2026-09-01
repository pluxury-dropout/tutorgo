package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type CalendarHandler struct {
	service service.CalendarService
	log     *slog.Logger
}

func NewCalendarHandler(svc service.CalendarService, log *slog.Logger) *CalendarHandler {
	return &CalendarHandler{service: svc, log: log}
}

// GetFeed — GET /calendar/feed?from=&to=&kinds=lesson,event,task
func (h *CalendarHandler) GetFeed(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	from, to := c.Query("from"), c.Query("to")
	if from == "" || to == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to are required"})
		return
	}
	items, err := h.service.GetFeed(c.Request.Context(), tutorID, from, to, c.Query("kinds"))
	if err != nil {
		h.log.Error("Failed to get calendar feed", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetConflicts — GET /calendar/conflicts?starts_at=&duration_minutes=&exclude_type=&exclude_id=
func (h *CalendarHandler) GetConflicts(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	startsAt, err := time.Parse(time.RFC3339, c.Query("starts_at"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "starts_at must be an RFC3339 timestamp"})
		return
	}
	duration, err := strconv.Atoi(c.Query("duration_minutes"))
	if err != nil || duration <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "duration_minutes must be a positive number"})
		return
	}

	var excludeID *string
	if id := c.Query("exclude_id"); id != "" {
		excludeID = &id
	}
	conflicts, err := h.service.GetConflicts(c.Request.Context(), tutorID, startsAt, duration, c.Query("exclude_type"), excludeID)
	if err != nil {
		h.log.Error("Failed to get calendar conflicts", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	if conflicts == nil {
		conflicts = []models.CalendarItem{}
	}
	c.JSON(http.StatusOK, gin.H{"conflicts": conflicts})
}
