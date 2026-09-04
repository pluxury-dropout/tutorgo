package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type OnboardingHandler struct {
	service service.OnboardingService
	log     *slog.Logger
}

func NewOnboardingHandler(svc service.OnboardingService, log *slog.Logger) *OnboardingHandler {
	return &OnboardingHandler{service: svc, log: log}
}

// CreateStudent — POST /onboarding/student: ученик, курс и серия одним сабмитом.
func (h *OnboardingHandler) CreateStudent(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.OnboardingStudentRequest
	if !bindAndValidate(c, &req) {
		return
	}
	res, err := h.service.CreateStudent(c.Request.Context(), req, tutorID)
	if err != nil {
		h.log.Error("Failed to onboard student", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Student onboarded",
		slog.String("student_id", res.Student.ID),
		slog.Int("lessons", res.LessonsCreated))
	c.JSON(http.StatusCreated, res)
}
