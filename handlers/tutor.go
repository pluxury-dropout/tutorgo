package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type TutorHandler struct {
	service service.TutorService
	log     *slog.Logger
}

func NewTutorHandler(svc service.TutorService, log *slog.Logger) *TutorHandler {
	return &TutorHandler{service: svc, log: log}
}

func (h *TutorHandler) GetByID(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	id := c.Param("id")
	if id != tutorID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}
	tutor, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		h.log.Error("Failed to get tutor", slog.String("id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusNotFound, gin.H{"error": "Tutor not found"})
		return
	}
	h.log.Info("Tutor retrieved", slog.String("id", id))
	c.JSON(http.StatusOK, tutor)
}

func (h *TutorHandler) Update(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	id := c.Param("id")
	if id != tutorID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}
	var req models.UpdateTutorRequest
	if !bindAndValidate(c, &req) {
		return
	}
	tutor, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		h.log.Error("Failed to update tutor", slog.String("id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update tutor"})
		return
	}
	h.log.Info("Tutor updated", slog.String("id", id))
	c.JSON(http.StatusOK, tutor)
}

func (h *TutorHandler) Delete(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	id := c.Param("id")
	if id != tutorID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		h.log.Error("Failed to delete tutor", slog.String("id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tutor"})
		return
	}
	h.log.Info("Tutor deleted", slog.String("id", id))
	c.Status(http.StatusNoContent)
}
