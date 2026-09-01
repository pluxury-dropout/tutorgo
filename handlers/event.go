package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type EventHandler struct {
	service service.EventService
	log     *slog.Logger
}

func NewEventHandler(svc service.EventService, log *slog.Logger) *EventHandler {
	return &EventHandler{service: svc, log: log}
}

func (h *EventHandler) GetByRange(c *gin.Context) {
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
	events, err := h.service.GetByRange(c.Request.Context(), tutorID, from, to)
	if err != nil {
		h.log.Error("Failed to get events", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	if events == nil {
		events = []models.Event{}
	}
	c.JSON(http.StatusOK, events)
}

func (h *EventHandler) GetByID(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	event, err := h.service.GetByID(c.Request.Context(), c.Param("id"), tutorID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, event)
}

func (h *EventHandler) Create(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreateEventRequest
	if !bindAndValidate(c, &req) {
		return
	}
	event, err := h.service.Create(c.Request.Context(), tutorID, req)
	if err != nil {
		h.log.Error("Failed to create event", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, event)
}

func (h *EventHandler) Update(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.UpdateEventRequest
	if !bindAndValidate(c, &req) {
		return
	}
	event, err := h.service.Update(c.Request.Context(), c.Param("id"), tutorID, req)
	if err != nil {
		h.log.Error("Failed to update event", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, event)
}

func (h *EventHandler) Delete(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.Param("id"), tutorID); err != nil {
		h.log.Error("Failed to delete event", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
