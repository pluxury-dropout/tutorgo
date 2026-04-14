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
	courseID := c.Query("course_id")
	if courseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "course_id is required"})
		return
	}
	payments, err := h.service.GetByCourse(c.Request.Context(), courseID, tutorID)
	if err != nil {
		h.log.Error("Failed to get payments", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve payments"})
		return
	}
	h.log.Info("Payments retrieved", slog.Int("count", len(payments)))
	c.JSON(http.StatusOK, payments)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment"})
		return
	}
	h.log.Info("Payment created", slog.String("id", payment.ID), slog.Float64("amount", payment.Amount))
	c.JSON(http.StatusCreated, payment)
}

func (h *PaymentHandler) GetBalance(c *gin.Context) {
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
	balance, err := h.service.GetBalance(c.Request.Context(), courseID, tutorID)
	if err != nil {
		h.log.Error("Failed to get balance", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get balance"})
		return
	}
	c.JSON(http.StatusOK, balance)
}
