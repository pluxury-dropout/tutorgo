package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type LessonHandler struct {
	service service.LessonService
	log     *slog.Logger
}

func NewLessonHandler(svc service.LessonService, log *slog.Logger) *LessonHandler {
	return &LessonHandler{service: svc, log: log}
}

func (h *LessonHandler) GetByCourse(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	courseID := c.Query("course_id")
	if courseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "course_id is required"})
		return
	}
	lessons, err := h.service.GetByCourse(courseID, tutorID)
	if err != nil {
		h.log.Error("Failed to get lessons", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve lessons"})
		return
	}
	h.log.Info("Lessons retrieved", slog.Int("count", len(lessons)))
	c.JSON(http.StatusOK, lessons)
}

func (h *LessonHandler) Create(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	var req models.CreateLessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	lesson, err := h.service.Create(req, tutorID)
	if err != nil {
		h.log.Error("Failed to create lesson", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create lesson"})
		return
	}
	h.log.Info("Lesson created", slog.String("id", lesson.ID))
	c.JSON(http.StatusCreated, lesson)
}

func (h *LessonHandler) GetByID(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	id := c.Param("id")
	lesson, err := h.service.GetByID(id, tutorID)
	if err != nil {
		h.log.Error("Failed to get lesson", slog.String("id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusNotFound, gin.H{"error": "Lesson not found"})
		return
	}
	c.JSON(http.StatusOK, lesson)
}

func (h *LessonHandler) Update(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	id := c.Param("id")
	var req models.UpdateLessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	lesson, err := h.service.Update(id, req, tutorID)
	if err != nil {
		h.log.Error("Failed to update lesson", slog.String("id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update lesson"})
		return
	}
	h.log.Info("Lesson updated", slog.String("id", id))
	c.JSON(http.StatusOK, lesson)
}

func (h *LessonHandler) Delete(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	id := c.Param("id")
	if err := h.service.Delete(id, tutorID); err != nil {
		h.log.Error("Failed to delete lesson", slog.String("id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete lesson"})
		return
	}
	h.log.Info("Lesson deleted", slog.String("id", id))
	c.Status(http.StatusNoContent)
}
