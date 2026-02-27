package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
	"tutorgo/db"
	"tutorgo/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	conn      *pgx.Conn
	log       *slog.Logger
	jwtSecret string
}

func NewAuthHandler(conn *pgx.Conn, log *slog.Logger, jwtSecret string) *AuthHandler {
	return &AuthHandler{conn: conn, log: log, jwtSecret: jwtSecret}
}
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.RegisterRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid data format", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		http.Error(w, "Email, password, first_name and last_name are required", http.StatusBadRequest)
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to process password", http.StatusInternalServerError)
		h.log.Error("Failed to hash password", slog.String("error", err.Error()))
		return
	}

	tutor, err := db.RegisterTutor(h.conn, req, string(passwordHash))
	if err != nil {
		http.Error(w, "Failed to register tutor", http.StatusInternalServerError)
		h.log.Error("Failed to register tutor", slog.String("error", err.Error()))
		return
	}

	h.log.Info("Tutor registered", slog.String("id", tutor.ID), slog.String("email", tutor.Email))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tutor)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid data format", http.StatusBadRequest)
		return
	}

	id, passwordHash, err := db.GetTutorByEmail(h.conn, req.Email)
	if err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password))
	if err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  id,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		h.log.Error("Failed to sign token", slog.String("error", err.Error()))
		return
	}

	h.log.Info("Tutor logged in", slog.String("id", id))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.LoginResponse{Token: tokenString})
}
