package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

func (h *StudentHandler) Invite(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	studentID := c.Param("id")
	// Ownership check: student must belong to this tutor.
	if _, err := h.service.GetByID(c.Request.Context(), studentID, tutorID); err != nil {
		handleServiceError(c, err)
		return
	}
	token := uuid.NewString()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if err := h.service.SetInvite(c.Request.Context(), studentID, token, expiresAt); err != nil {
		h.log.Error("set invite failed", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create invite"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invite_token": token, "expires_at": expiresAt})
}

// GET /student/me — профиль текущего ученика.
func (h *StudentHandler) Me(c *gin.Context) {
	studentID := c.GetString("studentID")
	if studentID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	profile, err := h.service.GetProfile(c.Request.Context(), studentID)
	if err != nil {
		h.log.Error("student profile failed", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load profile"})
		return
	}
	c.JSON(http.StatusOK, profile)
}

// GET /student/homework — ДЗ по курсам ученика (только с непустым текстом).
func (h *StudentHandler) Homework(c *gin.Context) {
	studentID := c.GetString("studentID")
	if studentID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	hw, err := h.service.ListHomework(c.Request.Context(), studentID)
	if err != nil {
		h.log.Error("student homework failed", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load homework"})
		return
	}
	if hw == nil {
		hw = []models.StudentHomework{}
	}
	c.JSON(http.StatusOK, hw)
}

// GET /student/lessons?filter=upcoming|past — уроки текущего ученика.
func (h *StudentHandler) ListLessons(c *gin.Context) {
	studentID := c.GetString("studentID")
	if studentID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	past := c.Query("filter") == "past" // всё, кроме "past", трактуем как upcoming
	lessons, err := h.service.ListLessons(c.Request.Context(), studentID, past)
	if err != nil {
		h.log.Error("student lessons failed", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load lessons"})
		return
	}
	c.JSON(http.StatusOK, lessons)
}
