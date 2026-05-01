package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	service service.TaskService
	log     *slog.Logger
}

func NewTaskHandler(svc service.TaskService, log *slog.Logger) *TaskHandler {
	return &TaskHandler{service: svc, log: log}
}

func (h *TaskHandler) GetByRange(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	from := c.Query("from")
	to := c.Query("to")
	if from == "" || to == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to are required"})
		return
	}
	tasks, err := h.service.GetByRange(c.Request.Context(), tutorID, from, to)
	if err != nil {
		h.log.Error("Failed to get tasks", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve tasks"})
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	c.JSON(http.StatusOK, tasks)
}

func (h *TaskHandler) Create(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreateTaskRequest
	if !bindAndValidate(c, &req) {
		return
	}
	task, err := h.service.Create(c.Request.Context(), tutorID, req)
	if err != nil {
		h.log.Error("Failed to create task", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task"})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *TaskHandler) Update(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	var req models.UpdateTaskRequest
	if !bindAndValidate(c, &req) {
		return
	}
	task, err := h.service.Update(c.Request.Context(), id, tutorID, req)
	if err != nil {
		h.log.Error("Failed to update task", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task"})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), id, tutorID); err != nil {
		h.log.Error("Failed to delete task", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete task"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TaskHandler) ToggleDone(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	task, err := h.service.ToggleDone(c.Request.Context(), id, tutorID)
	if err != nil {
		h.log.Error("Failed to toggle task", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle task"})
		return
	}
	c.JSON(http.StatusOK, task)
}
