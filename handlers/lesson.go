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
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	courseID := c.Query("course_id")
	if courseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "course_id is required"})
		return
	}
	lessons, err := h.service.GetByCourse(c.Request.Context(), courseID, tutorID)
	if err != nil {
		h.log.Error("Failed to get lessons", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve lessons"})
		return
	}
	h.log.Info("Lessons retrieved", slog.Int("count", len(lessons)))
	c.JSON(http.StatusOK, lessons)
}

func (h *LessonHandler) CreateBulk(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreateBulkLessonRequest
	if !bindAndValidate(c, &req) {
		return
	}
	lessons, err := h.service.CreateBulk(c.Request.Context(), req, tutorID)
	if err != nil {
		h.log.Error("Failed to create lessons", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create lessons"})
		return
	}
	h.log.Info("Lessons created", slog.Int("count", len(lessons)))
	c.JSON(http.StatusCreated, lessons)
}

func (h *LessonHandler) Create(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreateLessonRequest
	if !bindAndValidate(c, &req) {
		return
	}
	lesson, err := h.service.Create(c.Request.Context(), req, tutorID)
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
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	lesson, err := h.service.GetByID(c.Request.Context(), id, tutorID)
	if err != nil {
		h.log.Error("Failed to get lesson", slog.String("id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusNotFound, gin.H{"error": "Lesson not found"})
		return
	}
	c.JSON(http.StatusOK, lesson)
}

func (h *LessonHandler) Update(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	var req models.UpdateLessonRequest
	if !bindAndValidate(c, &req) {
		return
	}
	lesson, err := h.service.Update(c.Request.Context(), id, req, tutorID)
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
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), id, tutorID); err != nil {
		h.log.Error("Failed to delete lesson", slog.String("id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete lesson"})
		return
	}
	h.log.Info("Lesson deleted", slog.String("id", id))
	c.Status(http.StatusNoContent)
}

func (h *LessonHandler) DeleteByCourse(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	courseID := c.Query("course_id")
	if courseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "course_id is required"})
		return
	}
	if err := h.service.DeleteByCourse(c.Request.Context(), courseID, tutorID); err != nil {
		h.log.Error("Failed to delete lessons by course", slog.String("courseId", courseID), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete lessons"})
		return
	}
	h.log.Info("Lessons deleted by course", slog.String("courseId", courseID))
	c.Status(http.StatusNoContent)
}

func (h *LessonHandler) DeleteSeries(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	seriesID := c.Param("seriesId")
	fromDate := c.Query("from")
	var fromDatePtr *string
	if fromDate != "" {
		fromDatePtr = &fromDate
	}
	if err := h.service.DeleteSeries(c.Request.Context(), seriesID, tutorID, fromDatePtr); err != nil {
		h.log.Error("Failed to delete series", slog.String("seriesId", seriesID), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete series"})
		return
	}
	h.log.Info("Series deleted", slog.String("seriesId", seriesID))
	c.Status(http.StatusNoContent)
}

func (h *LessonHandler) UpdateSeries(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	seriesID := c.Param("seriesId")
	var req models.UpdateSeriesRequest
	if !bindAndValidate(c, &req) {
		return
	}
	if err := h.service.UpdateSeries(c.Request.Context(), seriesID, tutorID, req); err != nil {
		h.log.Error("Failed to update series", slog.String("seriesId", seriesID), slog.String("error", err.Error()))
		if err.Error() == "at least one field must be provided" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update series"})
		return
	}
	h.log.Info("Series updated", slog.String("seriesId", seriesID))
	c.Status(http.StatusNoContent)
}

func (h *LessonHandler) GetCalendar(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	from := c.Query("from")
	to := c.Query("to")
	if from == "" || to == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to query parameters are required"})
		return
	}
	lessons, err := h.service.GetCalendar(c.Request.Context(), tutorID, from, to)
	if err != nil {
		h.log.Error("Failed to get calendar", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve calendar"})
		return
	}
	c.JSON(http.StatusOK, lessons)
}
