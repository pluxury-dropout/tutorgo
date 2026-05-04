package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type StudentHandler struct {
	service service.StudentService
	log     *slog.Logger
}

func NewStudentHandler(svc service.StudentService, log *slog.Logger) *StudentHandler {
	return &StudentHandler{service: svc, log: log}
}

func (h *StudentHandler) GetAll(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var p models.Pagination
	_ = c.ShouldBindQuery(&p)
	p.Normalize()

	students, total, err := h.service.GetAll(c.Request.Context(), tutorID, p)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.PagedResponse[models.Student]{
		Data: students, Total: total, Page: p.Page, Limit: p.Limit,
	})
}

func (h *StudentHandler) Create(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreateStudentRequest
	if !bindAndValidate(c, &req) {
		return
	}
	student, err := h.service.Create(c.Request.Context(), req, tutorID)
	if err != nil {
		h.log.Error("Failed to create student", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Student created", slog.String("id", student.ID))
	c.JSON(http.StatusCreated, student)
}

func (h *StudentHandler) GetByID(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	student, err := h.service.GetByID(c.Request.Context(), id, tutorID)
	if err != nil {
		h.log.Error("Failed to get student", slog.String("id", id), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, student)
}

func (h *StudentHandler) Update(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	var req models.UpdateStudentRequest
	if !bindAndValidate(c, &req) {
		return
	}
	student, err := h.service.Update(c.Request.Context(), id, tutorID, req)
	if err != nil {
		h.log.Error("Failed to update student", slog.String("id", id), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Student updated", slog.String("id", id))
	c.JSON(http.StatusOK, student)
}

func (h *StudentHandler) Delete(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), id, tutorID); err != nil {
		h.log.Error("Failed to delete student", slog.String("id", id), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Student deleted", slog.String("id", id))
	c.Status(http.StatusNoContent)
}
