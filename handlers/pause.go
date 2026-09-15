package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

// PauseHandler — заморозка ученика с карточки ученика (спека, п. 6.9).
type PauseHandler struct {
	service service.PauseService
	log     *slog.Logger
}

func NewPauseHandler(svc service.PauseService, log *slog.Logger) *PauseHandler {
	return &PauseHandler{service: svc, log: log}
}

func (h *PauseHandler) List(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	pauses, err := h.service.List(c.Request.Context(), c.Param("id"), tutorID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, pauses)
}

func (h *PauseHandler) Create(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreatePauseRequest
	if !bindAndValidate(c, &req) {
		return
	}
	studentID := c.Param("id")
	pause, err := h.service.Create(c.Request.Context(), studentID, tutorID, req)
	if err != nil {
		h.log.Error("Failed to create pause", slog.String("student_id", studentID), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Student paused", slog.String("student_id", studentID), slog.String("id", pause.ID))
	c.JSON(http.StatusCreated, pause)
}

func (h *PauseHandler) Delete(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.Param("pauseId"), c.Param("id"), tutorID); err != nil {
		handleServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
