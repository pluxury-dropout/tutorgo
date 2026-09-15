package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	service service.PaymentService
	log     *slog.Logger
}

func NewPaymentHandler(svc service.PaymentService, log *slog.Logger) *PaymentHandler {
	return &PaymentHandler{service: svc, log: log}
}

func (h *PaymentHandler) GetAll(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var p models.Pagination
	_ = c.ShouldBindQuery(&p)
	p.Normalize()

	courseID := c.Query("course_id")
	if courseID != "" {
		payments, total, err := h.service.GetByCourse(c.Request.Context(), courseID, tutorID, p)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, models.PagedResponse[models.Payment]{
			Data: payments, Total: total, Page: p.Page, Limit: p.Limit,
		})
		return
	}

	payments, total, err := h.service.GetAllByTutorPaged(c.Request.Context(), tutorID, p)
	if err != nil {
		h.log.Error("Failed to get all payments", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.PagedResponse[models.Payment]{
		Data: payments, Total: total, Page: p.Page, Limit: p.Limit,
	})
}

func (h *PaymentHandler) Create(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreatePaymentRequest
	if !bindAndValidate(c, &req) {
		return
	}
	payment, err := h.service.Create(c.Request.Context(), req, tutorID)
	if err != nil {
		h.log.Error("Failed to create payment", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Payment created", slog.String("id", payment.ID), slog.Float64("amount", payment.Amount))
	c.JSON(http.StatusCreated, payment)
}

func (h *PaymentHandler) GetRecent(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	payments, err := h.service.GetAllByTutor(c.Request.Context(), tutorID, 5)
	if err != nil {
		h.log.Error("Failed to get recent payments", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, payments)
}

func (h *PaymentHandler) GetMonthlyIncome(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	total, err := h.service.GetMonthlyIncome(c.Request.Context(), tutorID)
	if err != nil {
		h.log.Error("Failed to get monthly income", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total})
}

func (h *PaymentHandler) GetMonthlyExpected(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	total, err := h.service.GetMonthlyExpected(c.Request.Context(), tutorID)
	if err != nil {
		h.log.Error("Failed to get monthly expected", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total})
}

func (h *PaymentHandler) Delete(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), id, tutorID); err != nil {
		handleServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PaymentHandler) Update(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	var req models.UpdatePaymentRequest
	if !bindAndValidate(c, &req) {
		return
	}
	payment, err := h.service.Update(c.Request.Context(), id, tutorID, req)
	if err != nil {
		h.log.Error("Failed to update payment", slog.String("id", id), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Payment updated", slog.String("id", id))
	c.JSON(http.StatusOK, payment)
}

func (h *PaymentHandler) GetBalance(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	courseID := c.Query("course_id")
	studentID := c.Query("student_id")
	// Баланс — свойство пары «курс + ученик»: у группы без ученика он ничего не
	// значит (спека, п. 6.4).
	if courseID == "" || studentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "course_id and student_id are required"})
		return
	}
	balance, err := h.service.GetBalance(c.Request.Context(), courseID, studentID, tutorID)
	if err != nil {
		h.log.Error("Failed to get balance", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, balance)
}

// GetDebts — «кто мне должен» (спека, п. 6.5).
func (h *PaymentHandler) GetDebts(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	debts, err := h.service.GetDebts(c.Request.Context(), tutorID)
	if err != nil {
		h.log.Error("Failed to get debts", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, debts)
}
