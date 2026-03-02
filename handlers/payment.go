package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"tutorgo/db"
	"tutorgo/models"

	"github.com/jackc/pgx/v5"
)

type PaymentHandler struct {
	conn *pgx.Conn
	log  *slog.Logger
}

func NewPaymentHandler(conn *pgx.Conn, log *slog.Logger) *PaymentHandler {
	return &PaymentHandler{conn: conn, log: log}
}

func (h *PaymentHandler) Handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getPayments(w, r)
	case http.MethodPost:
		h.createPayment(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PaymentHandler) getPayments(w http.ResponseWriter, r *http.Request) {
	courseID := r.URL.Query().Get("course_id")
	if courseID == "" {
		http.Error(w, "course_id is required", http.StatusBadRequest)
		return
	}

	payments, err := db.GetPaymentsByCourse(h.conn, courseID)
	if err != nil {
		http.Error(w, "Failed to retrieve payments", http.StatusInternalServerError)
		h.log.Error("Failed to get payments", slog.String("error", err.Error()))
		return
	}

	h.log.Info("Payments retrieved", slog.Int("count", len(payments)))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payments)
}

func (h *PaymentHandler) createPayment(w http.ResponseWriter, r *http.Request) {
	var req models.CreatePaymentRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid data format", http.StatusBadRequest)
		return
	}

	payment, err := db.CreatePayment(h.conn, req)
	if err != nil {
		http.Error(w, "Failed to create payment", http.StatusInternalServerError)
		h.log.Error("Failed to create payment", slog.String("error", err.Error()))
		return
	}

	h.log.Info("Payment created", slog.String("id", payment.ID), slog.Float64("amount", payment.Amount))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(payment)
}

func (h *PaymentHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	courseID := r.URL.Query().Get("course_id")
	if courseID == "" {
		http.Error(w, "course_id is required", http.StatusBadRequest)
		return
	}

	balance, err := db.GetCourseBalance(h.conn, courseID)
	if err != nil {
		http.Error(w, "Failed to get balance", http.StatusInternalServerError)
		h.log.Error("Failed to get balance", slog.String("error", err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{"lessons_remaining": balance})
}
