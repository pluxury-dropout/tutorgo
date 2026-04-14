package handlers_test

import (
	"errors"
	"net/http"
	"testing"
	"tutorgo/handlers"
	"tutorgo/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
	"log/slog"
)

func newAuthRouter(svc *mockTutorService) *gin.Engine {
	r := gin.New()
	h := handlers.NewAuthHandler(svc, slog.Default(), "test-secret")
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)
	return r
}

// Register

func TestAuthRegister_Success(t *testing.T) {
	svc := new(mockTutorService)
	r := newAuthRouter(svc)

	req := models.RegisterRequest{
		Email:     "tutor@example.com",
		Password:  "password123",
		FirstName: "Amir",
		LastName:  "Bekov",
	}

	// Password is hashed internally — we can't predict the exact hash, use mock.Anything
	svc.On("Create", mock.Anything, mock.MatchedBy(func(cr models.CreateTutorRequest) bool {
		return cr.Email == req.Email && cr.FirstName == req.FirstName
	}), mock.AnythingOfType("string")).Return(testTutor, nil)

	w := makeRequest(t, r, http.MethodPost, "/auth/register", req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestAuthRegister_ValidationError(t *testing.T) {
	svc := new(mockTutorService)
	r := newAuthRouter(svc)

	// password is too short (min=6)
	w := makeRequest(t, r, http.MethodPost, "/auth/register", map[string]string{
		"email":      "tutor@example.com",
		"password":   "123",
		"first_name": "Amir",
		"last_name":  "Bekov",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Create")
}

func TestAuthRegister_ServiceError(t *testing.T) {
	svc := new(mockTutorService)
	r := newAuthRouter(svc)

	req := models.RegisterRequest{
		Email:     "tutor@example.com",
		Password:  "password123",
		FirstName: "Amir",
		LastName:  "Bekov",
	}

	svc.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("string")).Return(models.Tutor{}, errors.New("email already exists"))

	w := makeRequest(t, r, http.MethodPost, "/auth/register", req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// Login

func TestAuthLogin_Success(t *testing.T) {
	svc := new(mockTutorService)
	r := newAuthRouter(svc)

	password := "password123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)

	svc.On("GetByEmail", mock.Anything, "tutor@example.com").Return(testTutorID, string(hash), nil)

	w := makeRequest(t, r, http.MethodPost, "/auth/login", models.LoginRequest{
		Email:    "tutor@example.com",
		Password: password,
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.LoginResponse
	decodeJSON(t, w, &got)
	assert.NotEmpty(t, got.Token)
	svc.AssertExpectations(t)
}

func TestAuthLogin_WrongPassword(t *testing.T) {
	svc := new(mockTutorService)
	r := newAuthRouter(svc)

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
	r := newAuthRouter(svc)

	svc.On("GetByEmail", mock.Anything, "unknown@example.com").Return("", "", errors.New("not found"))

	w := makeRequest(t, r, http.MethodPost, "/auth/login", models.LoginRequest{
		Email:    "unknown@example.com",
		Password: "password123",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertExpectations(t)
}

func TestAuthLogin_ValidationError(t *testing.T) {
	svc := new(mockTutorService)
	r := newAuthRouter(svc)

	// email is invalid format
	w := makeRequest(t, r, http.MethodPost, "/auth/login", map[string]string{
		"email":    "not-an-email",
		"password": "password123",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "GetByEmail")
}
