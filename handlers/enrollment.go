package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type EnrollmentHandler struct {
	service service.EnrollmentService
	log     *slog.Logger
}

func NewEnrollmentHandler(svc service.EnrollmentService, log *slog.Logger) *EnrollmentHandler {
	return &EnrollmentHandler{service: svc, log: log}
}

func (h *EnrollmentHandler) Add(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	courseID := c.Param("id")
	var req models.EnrollStudentRequest
	if !bindAndValidate(c, &req) {
		return
	}
	enrollment, err := h.service.Add(c.Request.Context(), courseID, req, tutorID)
	if err != nil {
		h.log.Error("Failed to enroll student", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.log.Info("Student enrolled", slog.String("course_id", courseID), slog.String("student_id", req.StudentID))
	c.JSON(http.StatusCreated, enrollment)
}

func (h *EnrollmentHandler) Remove(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	courseID := c.Param("id")
	studentID := c.Param("studentId")
	if err := h.service.Remove(c.Request.Context(), courseID, studentID, tutorID); err != nil {
		h.log.Error("Failed to remove enrollment", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.log.Info("Student removed from course", slog.String("course_id", courseID), slog.String("student_id", studentID))
	c.Status(http.StatusNoContent)
}

func (h *EnrollmentHandler) GetByCourse(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	courseID := c.Param("id")
	enrollments, err := h.service.GetByCourse(c.Request.Context(), courseID, tutorID)
	if err != nil {
		h.log.Error("Failed to get enrollments", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, enrollments)
}
