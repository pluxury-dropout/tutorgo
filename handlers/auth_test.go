package handlers_test

import (
	"errors"
	"net/http"
	"testing"
	"tutorgo/handlers"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
	"log/slog"
)

func newAuthRouter(svc *mockTutorService, regSvc *mockRegistrationService, refreshSvc *mockRefreshTokenService) *gin.Engine {
	r := gin.New()
	h := handlers.NewAuthHandler(svc, regSvc, refreshSvc, slog.Default(), "test-secret", false)
	r.POST("/auth/register", h.Register)
	r.POST("/auth/register/verify", h.RegisterVerify)
	r.POST("/auth/register/resend", h.RegisterResend)
	r.POST("/auth/login", h.Login)
	return r
}

// Register — шаг 1 (Start): шлём код, аккаунт ещё не создан

func TestAuthRegister_Accepted(t *testing.T) {
	svc := new(mockTutorService)
	regSvc := new(mockRegistrationService)
	refreshSvc := new(mockRefreshTokenService)
	r := newAuthRouter(svc, regSvc, refreshSvc)

	req := models.RegisterRequest{
		Email:     "tutor@example.com",
		Password:  "password123",
		FirstName: "Amir",
		LastName:  "Bekov",
	}
	regSvc.On("Start", mock.Anything, mock.MatchedBy(func(rr models.RegisterRequest) bool {
		return rr.Email == req.Email && rr.FirstName == req.FirstName
	})).Return(nil)

	w := makeRequest(t, r, http.MethodPost, "/auth/register", req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	regSvc.AssertExpectations(t)
}

func TestAuthRegister_ValidationError(t *testing.T) {
	svc := new(mockTutorService)
	regSvc := new(mockRegistrationService)
	refreshSvc := new(mockRefreshTokenService)
	r := newAuthRouter(svc, regSvc, refreshSvc)

	// password is too short (min=6)
	w := makeRequest(t, r, http.MethodPost, "/auth/register", map[string]string{
		"email":      "tutor@example.com",
		"password":   "123",
		"first_name": "Amir",
		"last_name":  "Bekov",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	regSvc.AssertNotCalled(t, "Start")
}

func TestAuthRegister_EmailTaken(t *testing.T) {
	svc := new(mockTutorService)
	regSvc := new(mockRegistrationService)
	refreshSvc := new(mockRefreshTokenService)
	r := newAuthRouter(svc, regSvc, refreshSvc)

	req := models.RegisterRequest{
		Email:     "tutor@example.com",
		Password:  "password123",
		FirstName: "Amir",
		LastName:  "Bekov",
	}
	regSvc.On("Start", mock.Anything, mock.Anything).Return(service.ErrEmailTaken)

	w := makeRequest(t, r, http.MethodPost, "/auth/register", req)

	assert.Equal(t, http.StatusConflict, w.Code)
	regSvc.AssertExpectations(t)
}

// Register — шаг 2 (Verify)

func TestAuthRegisterVerify_Success(t *testing.T) {
	svc := new(mockTutorService)
	regSvc := new(mockRegistrationService)
	refreshSvc := new(mockRefreshTokenService)
	r := newAuthRouter(svc, regSvc, refreshSvc)

	regSvc.On("Verify", mock.Anything, "tutor@example.com", "123456").Return(testTutor, nil)
	refreshSvc.On("Create", mock.Anything, testTutorID).Return("refresh-token-value", nil)

	w := makeRequest(t, r, http.MethodPost, "/auth/register/verify", models.VerifyRegistrationRequest{
		Email: "tutor@example.com",
		Code:  "123456",
	})

	assert.Equal(t, http.StatusCreated, w.Code)
	var got models.LoginResponse
	decodeJSON(t, w, &got)
	assert.NotEmpty(t, got.AccessToken)
	regSvc.AssertExpectations(t)
	refreshSvc.AssertExpectations(t)
}

func TestAuthRegisterVerify_InvalidCode(t *testing.T) {
	svc := new(mockTutorService)
	regSvc := new(mockRegistrationService)
	refreshSvc := new(mockRefreshTokenService)
	r := newAuthRouter(svc, regSvc, refreshSvc)

	regSvc.On("Verify", mock.Anything, "tutor@example.com", "000000").Return(models.Tutor{}, service.ErrInvalidCode)

	w := makeRequest(t, r, http.MethodPost, "/auth/register/verify", models.VerifyRegistrationRequest{
		Email: "tutor@example.com",
		Code:  "000000",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	regSvc.AssertExpectations(t)
}

// Register — resend

func TestAuthRegisterResend_Cooldown(t *testing.T) {
	svc := new(mockTutorService)
	regSvc := new(mockRegistrationService)
	refreshSvc := new(mockRefreshTokenService)
	r := newAuthRouter(svc, regSvc, refreshSvc)

	regSvc.On("Resend", mock.Anything, "tutor@example.com").Return(service.ErrResendCooldown)

	w := makeRequest(t, r, http.MethodPost, "/auth/register/resend", models.ResendRegistrationRequest{
		Email: "tutor@example.com",
	})

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	regSvc.AssertExpectations(t)
}

// Login

func TestAuthLogin_Success(t *testing.T) {
	svc := new(mockTutorService)
	refreshSvc := new(mockRefreshTokenService)
	r := newAuthRouter(svc, new(mockRegistrationService), refreshSvc)

	password := "password123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)

	svc.On("GetByEmail", mock.Anything, "tutor@example.com").Return(testTutorID, string(hash), nil)
	refreshSvc.On("Create", mock.Anything, testTutorID).Return("refresh-token-value", nil)

	w := makeRequest(t, r, http.MethodPost, "/auth/login", models.LoginRequest{
		Email:    "tutor@example.com",
		Password: password,
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.LoginResponse
	decodeJSON(t, w, &got)
	assert.NotEmpty(t, got.AccessToken)
	svc.AssertExpectations(t)
	refreshSvc.AssertExpectations(t)
}

func TestAuthLogin_WrongPassword(t *testing.T) {
	svc := new(mockTutorService)
	refreshSvc := new(mockRefreshTokenService)
	r := newAuthRouter(svc, new(mockRegistrationService), refreshSvc)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	svc.On("GetByEmail", mock.Anything, "tutor@example.com").Return(testTutorID, string(hash), nil)

	w := makeRequest(t, r, http.MethodPost, "/auth/login", models.LoginRequest{
		Email:    "tutor@example.com",
		Password: "wrong-password",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertExpectations(t)
}

func TestAuthLogin_EmailNotFound(t *testing.T) {
	svc := new(mockTutorService)
	refreshSvc := new(mockRefreshTokenService)
	r := newAuthRouter(svc, new(mockRegistrationService), refreshSvc)

	svc.On("GetByEmail", mock.Anything, "unknown@example.com").Return("", "", errors.New("not found"))

	w := makeRequest(t, r, http.MethodPost, "/auth/login", models.LoginRequest{
		Email:    "unknown@example.com",
		Password: "password123",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertExpectations(t)
}

func TestAuthLogin_ByPhone_Success(t *testing.T) {
	svc := new(mockTutorService)
	refreshSvc := new(mockRefreshTokenService)
	r := newAuthRouter(svc, new(mockRegistrationService), refreshSvc)

	password := "password123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)

	svc.On("GetByPhone", mock.Anything, "+77001234567").Return(testTutorID, string(hash), nil)
	refreshSvc.On("Create", mock.Anything, testTutorID).Return("refresh-token-value", nil)

	w := makeRequest(t, r, http.MethodPost, "/auth/login", models.LoginRequest{
		Phone:    "+77001234567",
		Password: password,
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.LoginResponse
	decodeJSON(t, w, &got)
	assert.NotEmpty(t, got.AccessToken)
	svc.AssertExpectations(t)
	refreshSvc.AssertExpectations(t)
}

func TestAuthLogin_PhoneNotFound(t *testing.T) {
	svc := new(mockTutorService)
	refreshSvc := new(mockRefreshTokenService)
	r := newAuthRouter(svc, new(mockRegistrationService), refreshSvc)

	svc.On("GetByPhone", mock.Anything, "+70000000000").Return("", "", errors.New("not found"))

	w := makeRequest(t, r, http.MethodPost, "/auth/login", models.LoginRequest{
		Phone:    "+70000000000",
		Password: "password123",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertExpectations(t)
}

func TestAuthLogin_ValidationError(t *testing.T) {
	svc := new(mockTutorService)
	refreshSvc := new(mockRefreshTokenService)
	r := newAuthRouter(svc, new(mockRegistrationService), refreshSvc)

	// email is invalid format
	w := makeRequest(t, r, http.MethodPost, "/auth/login", map[string]string{
		"email":    "not-an-email",
		"password": "password123",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "GetByEmail")
}
