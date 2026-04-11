package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"
)

type PaymentHandler struct {
	service service.PaymentService
	log     *slog.Logger
}

func NewPaymentHandler(svc service.PaymentService, log *slog.Logger) *PaymentHandler {
	return &PaymentHandler{service: svc, log: log}
}

func (h *PaymentHandler) Handle(w http.ResponseWriter, r *http.Request) {
	tutorID := r.Context().Value("tutorID").(string)
	switch r.Method {
	case http.MethodGet:
		h.getPayments(w, r, tutorID)
	case http.MethodPost:
		h.createPayment(w, r, tutorID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PaymentHandler) getPayments(w http.ResponseWriter, r *http.Request, tutorID string) {
	courseID := r.URL.Query().Get("course_id")
	if courseID == "" {
		http.Error(w, "course_id is required", http.StatusBadRequest)
		return
	}
	payments, err := h.service.GetByCourse(courseID, tutorID)
	if err != nil {
		http.Error(w, "Failed to retrieve payments", http.StatusInternalServerError)
		h.log.Error("Failed to get payments", slog.String("error", err.Error()))
		return
	}
	h.log.Info("Payments retrieved", slog.Int("count", len(payments)))
	respondJSON(w, http.StatusOK, payments)
}

func (h *PaymentHandler) createPayment(w http.ResponseWriter, r *http.Request, tutorID string) {
	var req models.CreatePaymentRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}
	payment, err := h.service.Create(req, tutorID)
	if err != nil {
		http.Error(w, "Failed to create payment", http.StatusInternalServerError)
		h.log.Error("Failed to create payment", slog.String("error", err.Error()))
		return
	}
	h.log.Info("Payment created", slog.String("id", payment.ID), slog.Float64("amount", payment.Amount))
	respondJSON(w, http.StatusCreated, payment)
}

func (h *PaymentHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	tutorID := r.Context().Value("tutorID").(string)
	courseID := r.URL.Query().Get("course_id")
	if courseID == "" {
		http.Error(w, "course_id is required", http.StatusBadRequest)
		return
	}
	balance, err := h.service.GetBalance(courseID, tutorID)
	if err != nil {
		http.Error(w, "Failed to get balance", http.StatusInternalServerError)
		h.log.Error("Failed to get balance", slog.String("error", err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, map[string]int{"lessons_remaining": balance})
}
