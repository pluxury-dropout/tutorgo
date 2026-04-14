package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
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

func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if !bindAndValidate(c, &req) {
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.log.Error("Failed to hash password", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	createReq := models.CreateTutorRequest{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
	}

	tutor, err := h.service.Create(c.Request.Context(), createReq, string(passwordHash))
	if err != nil {
		h.log.Error("Failed to register tutor", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register tutor"})
		return
	}

	h.log.Info("Tutor registered", slog.String("id", tutor.ID), slog.String("email", tutor.Email))
	c.JSON(http.StatusCreated, tutor)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if !bindAndValidate(c, &req) {
		return
	}

	id, passwordHash, err := h.service.GetByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	if err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  id,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		h.log.Error("Failed to sign token", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	h.log.Info("Tutor logged in", slog.String("id", id))
	c.JSON(http.StatusOK, models.LoginResponse{Token: tokenString})
}
