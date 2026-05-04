package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type CourseHandler struct {
	service service.CourseService
	log     *slog.Logger
}

func NewCourseHandler(svc service.CourseService, log *slog.Logger) *CourseHandler {
	return &CourseHandler{service: svc, log: log}
}

func (h *CourseHandler) GetAll(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var p models.Pagination
	_ = c.ShouldBindQuery(&p)
	p.Normalize()

	courses, total, err := h.service.GetAll(c.Request.Context(), tutorID, p)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.PagedResponse[models.Course]{
		Data: courses, Total: total, Page: p.Page, Limit: p.Limit,
	})
}

func (h *CourseHandler) Create(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreateCourseRequest
	if !bindAndValidate(c, &req) {
		return
	}
	course, err := h.service.Create(c.Request.Context(), req, tutorID)
	if err != nil {
		h.log.Error("Failed to create course", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Course created", slog.String("id", course.ID))
	c.JSON(http.StatusCreated, course)
}

func (h *CourseHandler) GetByID(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	course, err := h.service.GetByID(c.Request.Context(), id, tutorID)
	if err != nil {
		h.log.Error("Failed to get course", slog.String("id", id), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, course)
}

func (h *CourseHandler) GetByStudent(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	studentID := c.Param("id")
	courses, err := h.service.GetByStudent(c.Request.Context(), studentID, tutorID)
	if err != nil {
		h.log.Error("Failed to get courses by student", slog.String("studentID", studentID), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, courses)
}

func (h *CourseHandler) Update(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	var req models.UpdateCourseRequest
	if !bindAndValidate(c, &req) {
		return
	}
	course, err := h.service.Update(c.Request.Context(), id, tutorID, req)
	if err != nil {
		h.log.Error("Failed to update course", slog.String("id", id), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Course updated", slog.String("id", id))
	c.JSON(http.StatusOK, course)
}

func (h *CourseHandler) Delete(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), id, tutorID); err != nil {
		h.log.Error("Failed to delete course", slog.String("id", id), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Course deleted", slog.String("id", id))
	c.Status(http.StatusNoContent)
}
