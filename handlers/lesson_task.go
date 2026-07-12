package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

// LessonTaskHandler — HTTP-слой задач урока (lesson_tasks). Отдельный от
// TaskHandler (tutor-канбан). Маппинг ошибок — через общий handleServiceError.
type LessonTaskHandler struct {
	service service.LessonTaskService
	log     *slog.Logger
}

func NewLessonTaskHandler(svc service.LessonTaskService, log *slog.Logger) *LessonTaskHandler {
	return &LessonTaskHandler{service: svc, log: log}
}

// GET /lessons/:id/tasks
func (h *LessonTaskHandler) ListForTutor(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	tasks, err := h.service.ListForTutor(c.Request.Context(), c.Param("id"), tutorID)
	if err != nil {
		h.log.Error("Failed to list lesson tasks", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// POST /lessons/:id/tasks
func (h *LessonTaskHandler) Create(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreateLessonTaskRequest
	if !bindAndValidate(c, &req) {
		return
	}
	task, err := h.service.Create(c.Request.Context(), c.Param("id"), tutorID, req)
	if err != nil {
		h.log.Error("Failed to create lesson task", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, task)
}

// PUT /lesson-tasks/:id
func (h *LessonTaskHandler) Update(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.UpdateLessonTaskRequest
	if !bindAndValidate(c, &req) {
		return
	}
	task, err := h.service.Update(c.Request.Context(), c.Param("id"), tutorID, req)
	if err != nil {
		h.log.Error("Failed to update lesson task", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, task)
}

// DELETE /lesson-tasks/:id
func (h *LessonTaskHandler) Delete(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.Param("id"), tutorID); err != nil {
		h.log.Error("Failed to delete lesson task", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GET /student/lessons/:id/tasks
func (h *LessonTaskHandler) ListForStudent(c *gin.Context) {
	studentID := c.GetString("studentID")
	if studentID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	tasks, err := h.service.ListForStudent(c.Request.Context(), c.Param("id"), studentID)
	if err != nil {
		h.log.Error("Failed to list student lesson tasks", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// PATCH /student/lesson-tasks/:id
func (h *LessonTaskHandler) StudentSetDone(c *gin.Context) {
	studentID := c.GetString("studentID")
	if studentID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.SetLessonTaskDoneRequest
	if !bindAndValidate(c, &req) {
		return
	}
	err := h.service.SetDone(c.Request.Context(), c.Param("id"), studentID, req.Done)
	if err != nil {
		h.log.Error("Failed to set lesson task done", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
