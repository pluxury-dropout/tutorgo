package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	service         service.TutorService
	refreshTokenSvc service.RefreshTokenService
	log             *slog.Logger
	jwtSecret       string
	secureCookie    bool
}

func NewAuthHandler(
	svc service.TutorService,
	refreshTokenSvc service.RefreshTokenService,
	log *slog.Logger,
	jwtSecret string,
	secureCookie bool,
) *AuthHandler {
	return &AuthHandler{
		service:         svc,
		refreshTokenSvc: refreshTokenSvc,
		log:             log,
		jwtSecret:       jwtSecret,
		secureCookie:    secureCookie,
	}
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "Email or phone is already taken"})
			return
		}
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

	var id, passwordHash string
	var err error
	if req.Phone != "" {
		id, passwordHash, err = h.service.GetByPhone(c.Request.Context(), req.Phone)
	} else {
		id, passwordHash, err = h.service.GetByEmail(c.Request.Context(), req.Email)
	}
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	accessToken, err := h.newAccessToken(id)
	if err != nil {
		h.log.Error("Failed to sign token", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	refreshToken, err := h.refreshTokenSvc.Create(c.Request.Context(), id)
	if err != nil {
		h.log.Error("Failed to create refresh token", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	h.setRefreshCookie(c, refreshToken)
	h.log.Info("Tutor logged in", slog.String("id", id))
	c.JSON(http.StatusOK, models.LoginResponse{AccessToken: accessToken})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	cookieToken, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No refresh token"})
		return
	}

	tutorID, err := h.refreshTokenSvc.Validate(c.Request.Context(), cookieToken)
	if err != nil {
		h.clearRefreshCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	accessToken, err := h.newAccessToken(tutorID)
	if err != nil {
		h.log.Error("Failed to sign token", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.LoginResponse{AccessToken: accessToken})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	cookieToken, err := c.Cookie("refresh_token")
	if err == nil {
		_ = h.refreshTokenSvc.Revoke(c.Request.Context(), cookieToken)
	}
	h.clearRefreshCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) newAccessToken(tutorID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  tutorID,
		"exp": time.Now().Add(15 * time.Minute).Unix(),
	})
	return token.SignedString([]byte(h.jwtSecret))
}

func (h *AuthHandler) setRefreshCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   30 * 24 * 60 * 60,
		Path:     "/auth",
	})
}

func (h *AuthHandler) clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Path:     "/auth",
	})
}
