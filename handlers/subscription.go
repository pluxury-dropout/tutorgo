package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type SubscriptionHandler struct {
	svc service.SubscriptionService
	log *slog.Logger
}

func NewSubscriptionHandler(svc service.SubscriptionService, log *slog.Logger) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc, log: log}
}

func (h *SubscriptionHandler) GetStatus(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	status, err := h.svc.GetStatus(c.Request.Context(), tutorID)
	if err != nil {
		h.log.Error("get subscription status", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *SubscriptionHandler) Checkout(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.SubscriptionPlanRequest
	if !bindAndValidate(c, &req) {
		return
	}
	url, err := h.svc.Checkout(c.Request.Context(), tutorID, req.Plan)
	if err != nil {
		h.log.Error("checkout", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"checkout_url": url})
}

func (h *SubscriptionHandler) Confirm(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.SubscriptionPlanRequest
	if !bindAndValidate(c, &req) {
		return
	}
	if err := h.svc.Confirm(c.Request.Context(), tutorID, req.Plan); err != nil {
		h.log.Error("confirm", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Status(http.StatusNoContent)
}

// POST /subscription/webhook — публичный, без JWT (шлёт провайдер).
func (h *SubscriptionHandler) Webhook(c *gin.Context) {
	err := h.svc.HandleWebhook(c.Request.Context(), c.Request)
	switch {
	case err == nil:
		c.Status(http.StatusOK)
	case errors.Is(err, service.ErrBadSignature):
		h.log.Warn("rejected subscription webhook: bad signature")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid webhook"})
	default:
		h.log.Error("subscription webhook", slog.String("error", err.Error()))
		c.Status(http.StatusInternalServerError) // провайдер передоставит, dedup спасёт
	}
}

func (h *SubscriptionHandler) Cancel(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.svc.Cancel(c.Request.Context(), tutorID); err != nil {
		h.log.Error("cancel subscription", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SubscriptionHandler) ChangePlan(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.SubscriptionPlanRequest
	if !bindAndValidate(c, &req) {
		return
	}
	if err := h.svc.ChangePlan(c.Request.Context(), tutorID, req.Plan); err != nil {
		h.log.Error("change plan", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Status(http.StatusNoContent)
}
