package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	service   service.TutorService
	log       *slog.Logger
	jwtSecret string
}

func NewAuthHandler(svc service.TutorService, log *slog.Logger, jwtSecret string) *AuthHandler {
	return &AuthHandler{service: svc, log: log, jwtSecret: jwtSecret}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.RegisterRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to process password", http.StatusInternalServerError)
		h.log.Error("Failed to hash password", slog.String("error", err.Error()))
		return
	}

	createReq := models.CreateTutorRequest{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
	}

	tutor, err := h.service.Create(createReq, string(passwordHash))
	if err != nil {
		http.Error(w, "Failed to register tutor", http.StatusInternalServerError)
		h.log.Error("Failed to register tutor", slog.String("error", err.Error()))
		return
	}

	h.log.Info("Tutor registered", slog.String("id", tutor.ID), slog.String("email", tutor.Email))
	respondJSON(w, http.StatusCreated, tutor)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.LoginRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	id, passwordHash, err := h.service.GetByEmail(req.Email)
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
	respondJSON(w, http.StatusOK, models.LoginResponse{Token: tokenString})
}
