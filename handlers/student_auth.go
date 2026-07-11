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

type StudentAuthHandler struct {
	service      service.StudentService
	refreshSvc   service.StudentRefreshTokenService
	log          *slog.Logger
	jwtSecret    string
	secureCookie bool
}

func NewStudentAuthHandler(svc service.StudentService, refreshSvc service.StudentRefreshTokenService, log *slog.Logger, jwtSecret string, secureCookie bool) *StudentAuthHandler {
	return &StudentAuthHandler{service: svc, refreshSvc: refreshSvc, log: log, jwtSecret: jwtSecret, secureCookie: secureCookie}
}

func (h *StudentAuthHandler) AcceptInvite(c *gin.Context) {
	var req models.AcceptInviteRequest
	if !bindAndValidate(c, &req) {
		return
	}
	studentID, exp, err := h.service.GetByInviteToken(c.Request.Context(), req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invite"})
		return
	}
	if time.Now().After(exp) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invite expired"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}
	if err := h.service.ActivateAccount(c.Request.Context(), studentID, req.Username, string(hash)); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "Username or phone already taken"})
			return
		}
		h.log.Error("activate account failed", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
		return
	}
	h.issueSession(c, studentID)
}

func (h *StudentAuthHandler) Login(c *gin.Context) {
	var req models.StudentLoginRequest
	if !bindAndValidate(c, &req) {
		return
	}
	id, hash, err := h.service.GetCredentialsByLogin(c.Request.Context(), req.Identifier)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}
	h.issueSession(c, id)
}

func (h *StudentAuthHandler) Refresh(c *gin.Context) {
	cookieToken, err := c.Cookie("student_refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No refresh token"})
		return
	}
	studentID, err := h.refreshSvc.Validate(c.Request.Context(), cookieToken)
	if err != nil {
		h.clearCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}
	access, err := h.newAccessToken(studentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	c.JSON(http.StatusOK, models.LoginResponse{AccessToken: access})
}

func (h *StudentAuthHandler) Logout(c *gin.Context) {
	if cookieToken, err := c.Cookie("student_refresh_token"); err == nil {
		_ = h.refreshSvc.Revoke(c.Request.Context(), cookieToken)
	}
	h.clearCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *StudentAuthHandler) ChangePassword(c *gin.Context) {
	studentID := c.GetString("studentID")
	if studentID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.StudentChangePasswordRequest
	if !bindAndValidate(c, &req) {
		return
	}
	hash, err := h.service.GetPasswordHash(c.Request.Context(), studentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load account"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.OldPassword)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process password"})
		return
	}
	if err := h.service.UpdatePassword(c.Request.Context(), studentID, string(newHash)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}
	// Смена пароля выкидывает все сессии; текущему устройству тут же выдаём свежую.
	if err := h.refreshSvc.RevokeAll(c.Request.Context(), studentID); err != nil {
		h.log.Error("revoke all sessions failed", slog.String("error", err.Error()))
	}
	h.issueSession(c, studentID)
}

func (h *StudentAuthHandler) issueSession(c *gin.Context, studentID string) {
	access, err := h.newAccessToken(studentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	refresh, err := h.refreshSvc.Create(c.Request.Context(), studentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}
	h.setCookie(c, refresh)
	c.JSON(http.StatusOK, models.LoginResponse{AccessToken: access})
}

func (h *StudentAuthHandler) newAccessToken(studentID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":   studentID,
		"role": "student",
		"exp":  time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	return token.SignedString([]byte(h.jwtSecret))
}

func (h *StudentAuthHandler) setCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: "student_refresh_token", Value: token, HttpOnly: true,
		Secure: h.secureCookie, SameSite: http.SameSiteStrictMode,
		MaxAge: 30 * 24 * 60 * 60, Path: "/student/auth",
	})
}

func (h *StudentAuthHandler) clearCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: "student_refresh_token", Value: "", HttpOnly: true,
		Secure: h.secureCookie, SameSite: http.SameSiteStrictMode,
		MaxAge: -1, Path: "/student/auth",
	})
}
