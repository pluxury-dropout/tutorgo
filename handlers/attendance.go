package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type AttendanceHandler struct {
	service service.AttendanceService
	log     *slog.Logger
}

func NewAttendanceHandler(svc service.AttendanceService, log *slog.Logger) *AttendanceHandler {
	return &AttendanceHandler{service: svc, log: log}
}

func (h *AttendanceHandler) Update(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	lessonID := c.Param("id")
	var req models.UpdateAttendanceRequest
	if !bindAndValidate(c, &req) {
		return
	}
	if err := h.service.Update(c.Request.Context(), lessonID, req, tutorID); err != nil {
		h.log.Error("Failed to update attendance", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Attendance updated", slog.String("lesson_id", lessonID))
	c.Status(http.StatusNoContent)
}

func (h *AttendanceHandler) Get(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	lessonID := c.Param("id")
	attendances, err := h.service.GetByLesson(c.Request.Context(), lessonID, tutorID)
	if err != nil {
		h.log.Error("Failed to get attendance", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, attendances)
}
