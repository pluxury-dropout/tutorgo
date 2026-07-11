package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"tutorgo/handlers"
	"tutorgo/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
	"log/slog"
)

func newStudentAuthRouter(svc *mockStudentService, refreshSvc *mockStudentRefreshTokenService) *gin.Engine {
	r := gin.New()
	h := handlers.NewStudentAuthHandler(svc, refreshSvc, slog.Default(), "test-secret", false)
	r.POST("/student/auth/accept-invite", h.AcceptInvite)
	r.POST("/student/auth/login", h.Login)
	r.POST("/student/auth/refresh", h.Refresh)
	r.POST("/student/auth/logout", h.Logout)
	return r
}

// Login

func TestStudentLogin_Success(t *testing.T) {
	svc := new(mockStudentService)
	refreshSvc := new(mockStudentRefreshTokenService)
	r := newStudentAuthRouter(svc, refreshSvc)

	password := "password123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)

	svc.On("GetCredentialsByLogin", mock.Anything, "kamila").Return(testStudentID, string(hash), nil)
	refreshSvc.On("Create", mock.Anything, testStudentID).Return("refresh-token-value", nil)

	w := makeRequest(t, r, http.MethodPost, "/student/auth/login", models.StudentLoginRequest{
		Identifier: "kamila",
		Password:   password,
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.LoginResponse
	decodeJSON(t, w, &got)
	assert.NotEmpty(t, got.AccessToken)

	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(got.AccessToken, claims, func(*jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	})
	assert.NoError(t, err)
	assert.Equal(t, testStudentID, claims["id"])
	assert.Equal(t, "student", claims["role"])

	var cookieFound bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "student_refresh_token" {
			cookieFound = true
			assert.Equal(t, "refresh-token-value", c.Value)
		}
	}
	assert.True(t, cookieFound, "expected Set-Cookie for student_refresh_token")

	svc.AssertExpectations(t)
	refreshSvc.AssertExpectations(t)
}

func TestStudentLogin_WrongPassword(t *testing.T) {
	svc := new(mockStudentService)
	refreshSvc := new(mockStudentRefreshTokenService)
	r := newStudentAuthRouter(svc, refreshSvc)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.MinCost)
	svc.On("GetCredentialsByLogin", mock.Anything, "kamila").Return(testStudentID, string(hash), nil)

	w := makeRequest(t, r, http.MethodPost, "/student/auth/login", models.StudentLoginRequest{
		Identifier: "kamila",
		Password:   "wrongpw",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertExpectations(t)
	refreshSvc.AssertNotCalled(t, "Create")
}

func TestStudentLogin_NotFound(t *testing.T) {
	svc := new(mockStudentService)
	refreshSvc := new(mockStudentRefreshTokenService)
	r := newStudentAuthRouter(svc, refreshSvc)

	svc.On("GetCredentialsByLogin", mock.Anything, "unknown").Return("", "", assert.AnError)

	w := makeRequest(t, r, http.MethodPost, "/student/auth/login", models.StudentLoginRequest{
		Identifier: "unknown",
		Password:   "password123",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertExpectations(t)
}

// AcceptInvite

func TestStudentAcceptInvite_Expired(t *testing.T) {
	svc := new(mockStudentService)
	refreshSvc := new(mockStudentRefreshTokenService)
	r := newStudentAuthRouter(svc, refreshSvc)

	token := "11111111-1111-1111-1111-111111111111"
	expired := time.Now().Add(-1 * time.Hour)
	svc.On("GetByInviteToken", mock.Anything, token).Return(testStudentID, expired, nil)

	w := makeRequest(t, r, http.MethodPost, "/student/auth/accept-invite", models.AcceptInviteRequest{
		Token:    token,
		Username: "kamila123",
		Password: "password123",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
	svc.AssertNotCalled(t, "ActivateAccount")
}

func TestStudentAcceptInvite_InvalidToken(t *testing.T) {
	svc := new(mockStudentService)
	refreshSvc := new(mockStudentRefreshTokenService)
	r := newStudentAuthRouter(svc, refreshSvc)

	token := "22222222-2222-2222-2222-222222222222"
	svc.On("GetByInviteToken", mock.Anything, token).Return("", time.Time{}, assert.AnError)

	w := makeRequest(t, r, http.MethodPost, "/student/auth/accept-invite", models.AcceptInviteRequest{
		Token:    token,
		Username: "kamila123",
		Password: "password123",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestStudentAcceptInvite_Success(t *testing.T) {
	svc := new(mockStudentService)
	refreshSvc := new(mockStudentRefreshTokenService)
	r := newStudentAuthRouter(svc, refreshSvc)

	token := "33333333-3333-3333-3333-333333333333"
	valid := time.Now().Add(1 * time.Hour)
	svc.On("GetByInviteToken", mock.Anything, token).Return(testStudentID, valid, nil)
	svc.On("ActivateAccount", mock.Anything, testStudentID, "kamila123", mock.AnythingOfType("string")).Return(nil)
	refreshSvc.On("Create", mock.Anything, testStudentID).Return("refresh-token-value", nil)

	w := makeRequest(t, r, http.MethodPost, "/student/auth/accept-invite", models.AcceptInviteRequest{
		Token:    token,
		Username: "kamila123",
		Password: "password123",
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.LoginResponse
	decodeJSON(t, w, &got)
	assert.NotEmpty(t, got.AccessToken)
	svc.AssertExpectations(t)
	refreshSvc.AssertExpectations(t)
}

func TestStudentAcceptInvite_Duplicate(t *testing.T) {
	svc := new(mockStudentService)
	refreshSvc := new(mockStudentRefreshTokenService)
	r := newStudentAuthRouter(svc, refreshSvc)

	token := "44444444-4444-4444-4444-444444444444"
	valid := time.Now().Add(1 * time.Hour)
	svc.On("GetByInviteToken", mock.Anything, token).Return(testStudentID, valid, nil)
	svc.On("ActivateAccount", mock.Anything, testStudentID, "taken", mock.AnythingOfType("string")).
		Return(&pgconn.PgError{Code: "23505"})

	w := makeRequest(t, r, http.MethodPost, "/student/auth/accept-invite", models.AcceptInviteRequest{
		Token:    token,
		Username: "taken",
		Password: "password123",
	})

	assert.Equal(t, http.StatusConflict, w.Code)
	svc.AssertExpectations(t)
	refreshSvc.AssertNotCalled(t, "Create")
}

// Refresh / Logout

func TestStudentRefresh_Success(t *testing.T) {
	svc := new(mockStudentService)
	refreshSvc := new(mockStudentRefreshTokenService)
	r := newStudentAuthRouter(svc, refreshSvc)

	refreshSvc.On("Validate", mock.Anything, "valid-refresh").Return(testStudentID, nil)

	req := httptest.NewRequest(http.MethodPost, "/student/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "student_refresh_token", Value: "valid-refresh"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.LoginResponse
	decodeJSON(t, w, &got)
	assert.NotEmpty(t, got.AccessToken)
	refreshSvc.AssertExpectations(t)
}

func TestStudentLogout_Success(t *testing.T) {
	svc := new(mockStudentService)
	refreshSvc := new(mockStudentRefreshTokenService)
	r := newStudentAuthRouter(svc, refreshSvc)

	refreshSvc.On("Revoke", mock.Anything, "valid-refresh").Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/student/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "student_refresh_token", Value: "valid-refresh"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	refreshSvc.AssertExpectations(t)
}

func TestStudentRefresh_NoCookie(t *testing.T) {
	svc := new(mockStudentService)
	refreshSvc := new(mockStudentRefreshTokenService)
	r := newStudentAuthRouter(svc, refreshSvc)

	w := makeRequest(t, r, http.MethodPost, "/student/auth/refresh", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestStudentLogout_NoCookie(t *testing.T) {
	svc := new(mockStudentService)
	refreshSvc := new(mockStudentRefreshTokenService)
	r := newStudentAuthRouter(svc, refreshSvc)

	w := makeRequest(t, r, http.MethodPost, "/student/auth/logout", nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	refreshSvc.AssertNotCalled(t, "Revoke")
}

// ChangePassword

func newStudentAuthProtectedRouter(svc *mockStudentService, refreshSvc *mockStudentRefreshTokenService) *gin.Engine {
	r := gin.New()
	h := handlers.NewStudentAuthHandler(svc, refreshSvc, slog.Default(), "test-secret", false)
	r.POST("/student/password", func(c *gin.Context) { c.Set("studentID", testStudentID); c.Next() }, h.ChangePassword)
	return r
}

func TestStudentChangePassword_WrongOld(t *testing.T) {
	svc := new(mockStudentService)
	refreshSvc := new(mockStudentRefreshTokenService)
	r := newStudentAuthProtectedRouter(svc, refreshSvc)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.MinCost)
	svc.On("GetPasswordHash", mock.Anything, testStudentID).Return(string(hash), nil)

	w := makeRequest(t, r, http.MethodPost, "/student/password", models.StudentChangePasswordRequest{
		OldPassword: "wrongpw", NewPassword: "newpass123",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "UpdatePassword")
	refreshSvc.AssertNotCalled(t, "RevokeAll")
}

func TestStudentChangePassword_Success(t *testing.T) {
	svc := new(mockStudentService)
	refreshSvc := new(mockStudentRefreshTokenService)
	r := newStudentAuthProtectedRouter(svc, refreshSvc)

	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpass"), bcrypt.MinCost)
	svc.On("GetPasswordHash", mock.Anything, testStudentID).Return(string(hash), nil)
	svc.On("UpdatePassword", mock.Anything, testStudentID, mock.AnythingOfType("string")).Return(nil)
	refreshSvc.On("RevokeAll", mock.Anything, testStudentID).Return(nil)
	refreshSvc.On("Create", mock.Anything, testStudentID).Return("fresh-refresh", nil)

	w := makeRequest(t, r, http.MethodPost, "/student/password", models.StudentChangePasswordRequest{
		OldPassword: "oldpass", NewPassword: "newpass123",
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.LoginResponse
	decodeJSON(t, w, &got)
	assert.NotEmpty(t, got.AccessToken)
	svc.AssertExpectations(t)
	refreshSvc.AssertExpectations(t)
}
