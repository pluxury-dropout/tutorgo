# Refresh Token Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add refresh tokens so sessions last 30 days, password change logs out all devices, and logout revokes only the current session.

**Architecture:** Access token (JWT, 15 min) in localStorage; refresh token (random 32-byte string, 30 days) in httpOnly cookie. On 401, frontend auto-calls `/auth/refresh`, saves new access token, retries original request. Password change calls `DeleteAllByTutorID` to invalidate all sessions. Logout deletes only the current device's token.

**Tech Stack:** Go (pgx, golang-jwt, crypto/rand), Next.js (axios interceptors, Zustand)

---

## File Map

**New files:**
- `migrations/010_refresh_tokens.sql` — DB table
- `repository/refresh_token.go` — DB interface + pgx implementation
- `service/refresh_token.go` — business logic + token generation
- `service/refresh_token_test.go` — service unit tests

**Modified files:**
- `config/config.go` — add `Env string` field (controls cookie `Secure` flag)
- `models/auth.go` — rename `LoginResponse.Token` → `AccessToken`, add json tag `access_token`
- `handlers/auth.go` — update `Login` (15 min TTL + cookie), add `Refresh` + `Logout`, update constructor
- `handlers/tutor.go` — `ChangePassword` calls `DeleteAllByTutorID`, update constructor
- `router/router.go` — wire new repo/service, pass to handlers, add 2 routes
- `frontend/src/lib/api/auth.ts` — rename token field, add `logout`
- `frontend/src/lib/api/client.ts` — 401 interceptor: try refresh → retry, else clear + redirect
- `frontend/src/app/(auth)/login/page.tsx` — use `access_token` field
- `frontend/src/components/layout/Sidebar.tsx` — async logout handler

---

### Task 1: Config — add Env field

**Files:**
- Modify: `config/config.go`

- [ ] **Step 1: Add `Env` field to Config struct and load it**

```go
// config/config.go
type Config struct {
	DBUrl            string
	ServerPort       string
	JWTSecret        string
	AllowedOrigin    string
	LiveKitURL       string
	LiveKitAPIKey    string
	LiveKitAPISecret string
	Env              string
}

// inside Load(), in the cfg := Config{...} block, add:
Env: os.Getenv("APP_ENV"), // "production" in prod, empty/other in dev
```

Full updated `Load()` function — replace the `cfg := Config{...}` block:
```go
cfg := Config{
    DBUrl:            os.Getenv("DB_URL"),
    ServerPort:       port,
    JWTSecret:        os.Getenv("JWT_SECRET"),
    AllowedOrigin:    os.Getenv("ALLOWED_ORIGIN"),
    LiveKitURL:       os.Getenv("LIVEKIT_URL"),
    LiveKitAPIKey:    os.Getenv("LIVEKIT_API_KEY"),
    LiveKitAPISecret: os.Getenv("LIVEKIT_API_SECRET"),
    Env:              os.Getenv("APP_ENV"),
}
```

- [ ] **Step 2: Build to verify no errors**

```bash
go build ./...
```
Expected: no output (clean build)

- [ ] **Step 3: Commit**

```bash
git add config/config.go
git commit -m "feat: add Env field to config"
```

---

### Task 2: Migration — refresh_tokens table

**Files:**
- Create: `migrations/010_refresh_tokens.sql`

- [ ] **Step 1: Write the migration**

```sql
-- migrations/010_refresh_tokens.sql
-- +goose Up
CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tutor_id   UUID NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX refresh_tokens_token_idx    ON refresh_tokens(token);
CREATE INDEX refresh_tokens_tutor_id_idx ON refresh_tokens(tutor_id);

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
```

- [ ] **Step 2: Apply migration**

```bash
goose -dir migrations postgres "$DB_URL" up
```
Expected: `OK   010_refresh_tokens.sql`

- [ ] **Step 3: Commit**

```bash
git add migrations/010_refresh_tokens.sql
git commit -m "feat: add refresh_tokens migration"
```

---

### Task 3: Repository — refresh_token.go

**Files:**
- Create: `repository/refresh_token.go`

- [ ] **Step 1: Write the repository**

```go
// repository/refresh_token.go
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshTokenRepository interface {
	Create(ctx context.Context, tutorID, token string, expiresAt time.Time) error
	GetByToken(ctx context.Context, token string) (tutorID string, expiresAt time.Time, err error)
	DeleteByToken(ctx context.Context, token string) error
	DeleteAllByTutorID(ctx context.Context, tutorID string) error
}

type refreshTokenRepository struct {
	conn *pgxpool.Pool
}

func NewRefreshTokenRepository(conn *pgxpool.Pool) RefreshTokenRepository {
	return &refreshTokenRepository{conn: conn}
}

func (r *refreshTokenRepository) Create(ctx context.Context, tutorID, token string, expiresAt time.Time) error {
	_, err := r.conn.Exec(ctx,
		`INSERT INTO refresh_tokens (tutor_id, token, expires_at) VALUES ($1, $2, $3)`,
		tutorID, token, expiresAt,
	)
	return err
}

func (r *refreshTokenRepository) GetByToken(ctx context.Context, token string) (string, time.Time, error) {
	var tutorID string
	var expiresAt time.Time
	err := r.conn.QueryRow(ctx,
		`SELECT tutor_id, expires_at FROM refresh_tokens WHERE token = $1`,
		token,
	).Scan(&tutorID, &expiresAt)
	return tutorID, expiresAt, err
}

func (r *refreshTokenRepository) DeleteByToken(ctx context.Context, token string) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE token = $1`, token)
	return err
}

func (r *refreshTokenRepository) DeleteAllByTutorID(ctx context.Context, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE tutor_id = $1`, tutorID)
	return err
}
```

- [ ] **Step 2: Build to verify**

```bash
go build ./...
```
Expected: no output

- [ ] **Step 3: Commit**

```bash
git add repository/refresh_token.go
git commit -m "feat: add refresh token repository"
```

---

### Task 4: Service — refresh_token.go + tests

**Files:**
- Create: `service/refresh_token.go`
- Create: `service/refresh_token_test.go`

- [ ] **Step 1: Write the failing tests first**

```go
// service/refresh_token_test.go
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockRefreshTokenRepo struct {
	mock.Mock
}

func (m *mockRefreshTokenRepo) Create(ctx context.Context, tutorID, token string, expiresAt time.Time) error {
	args := m.Called(ctx, tutorID, token, expiresAt)
	return args.Error(0)
}

func (m *mockRefreshTokenRepo) GetByToken(ctx context.Context, token string) (string, time.Time, error) {
	args := m.Called(ctx, token)
	return args.String(0), args.Get(1).(time.Time), args.Error(2)
}

func (m *mockRefreshTokenRepo) DeleteByToken(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *mockRefreshTokenRepo) DeleteAllByTutorID(ctx context.Context, tutorID string) error {
	args := m.Called(ctx, tutorID)
	return args.Error(0)
}

func TestRefreshToken_Create_ReturnsNonEmptyToken(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	repo.On("Create", mock.Anything, "tutor-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
		Return(nil)

	token, err := svc.Create(context.Background(), "tutor-1")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	repo.AssertExpectations(t)
}

func TestRefreshToken_Create_RepoError(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	repo.On("Create", mock.Anything, "tutor-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
		Return(errors.New("db error"))

	token, err := svc.Create(context.Background(), "tutor-1")

	assert.Error(t, err)
	assert.Empty(t, token)
	repo.AssertExpectations(t)
}

func TestRefreshToken_Validate_ValidToken(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	future := time.Now().Add(24 * time.Hour)
	repo.On("GetByToken", mock.Anything, "tok123").
		Return("tutor-1", future, nil)

	tutorID, err := svc.Validate(context.Background(), "tok123")

	assert.NoError(t, err)
	assert.Equal(t, "tutor-1", tutorID)
	repo.AssertExpectations(t)
}

func TestRefreshToken_Validate_Expired(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	past := time.Now().Add(-1 * time.Hour)
	repo.On("GetByToken", mock.Anything, "tok123").
		Return("tutor-1", past, nil)

	tutorID, err := svc.Validate(context.Background(), "tok123")

	assert.Error(t, err)
	assert.Empty(t, tutorID)
	repo.AssertExpectations(t)
}

func TestRefreshToken_Validate_NotFound(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	repo.On("GetByToken", mock.Anything, "tok123").
		Return("", time.Time{}, errors.New("not found"))

	tutorID, err := svc.Validate(context.Background(), "tok123")

	assert.Error(t, err)
	assert.Empty(t, tutorID)
	repo.AssertExpectations(t)
}

func TestRefreshToken_Revoke(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	repo.On("DeleteByToken", mock.Anything, "tok123").Return(nil)

	err := svc.Revoke(context.Background(), "tok123")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestRefreshToken_RevokeAll(t *testing.T) {
	repo := new(mockRefreshTokenRepo)
	svc := service.NewRefreshTokenService(repo)

	repo.On("DeleteAllByTutorID", mock.Anything, "tutor-1").Return(nil)

	err := svc.RevokeAll(context.Background(), "tutor-1")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./service/ -run TestRefreshToken -v
```
Expected: compile error `service.NewRefreshTokenService undefined`

- [ ] **Step 3: Write the service implementation**

```go
// service/refresh_token.go
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"tutorgo/repository"
)

var ErrTokenExpired = errors.New("token expired")

type RefreshTokenService interface {
	Create(ctx context.Context, tutorID string) (token string, err error)
	Validate(ctx context.Context, token string) (tutorID string, err error)
	Revoke(ctx context.Context, token string) error
	RevokeAll(ctx context.Context, tutorID string) error
}

type refreshTokenService struct {
	repo repository.RefreshTokenRepository
}

func NewRefreshTokenService(repo repository.RefreshTokenRepository) RefreshTokenService {
	return &refreshTokenService{repo: repo}
}

func (s *refreshTokenService) Create(ctx context.Context, tutorID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.URLEncoding.EncodeToString(b)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	if err := s.repo.Create(ctx, tutorID, token, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

func (s *refreshTokenService) Validate(ctx context.Context, token string) (string, error) {
	tutorID, expiresAt, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return "", err
	}
	if time.Now().After(expiresAt) {
		return "", ErrTokenExpired
	}
	return tutorID, nil
}

func (s *refreshTokenService) Revoke(ctx context.Context, token string) error {
	return s.repo.DeleteByToken(ctx, token)
}

func (s *refreshTokenService) RevokeAll(ctx context.Context, tutorID string) error {
	return s.repo.DeleteAllByTutorID(ctx, tutorID)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./service/ -run TestRefreshToken -v
```
Expected: all 6 tests PASS

- [ ] **Step 5: Commit**

```bash
git add service/refresh_token.go service/refresh_token_test.go
git commit -m "feat: add refresh token service with tests"
```

---

### Task 5: Update models/auth.go

**Files:**
- Modify: `models/auth.go`

- [ ] **Step 1: Rename `LoginResponse.Token` to `AccessToken`**

Replace the entire `LoginResponse` struct:
```go
// models/auth.go — replace LoginResponse
type LoginResponse struct {
	AccessToken string `json:"access_token"`
}
```

- [ ] **Step 2: Build to verify**

```bash
go build ./...
```
Expected: compile error in `handlers/auth.go` — `models.LoginResponse{Token: ...}` no longer valid. Will fix in Task 6.

---

### Task 6: Update handlers/auth.go

**Files:**
- Modify: `handlers/auth.go`

- [ ] **Step 1: Rewrite the file**

Replace the entire `handlers/auth.go` with:

```go
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
```

- [ ] **Step 2: Build to verify**

```bash
go build ./...
```
Expected: compile error in `router/router.go` — `NewAuthHandler` signature changed. Will fix in Task 8.

---

### Task 7: Update handlers/tutor.go — ChangePassword

**Files:**
- Modify: `handlers/tutor.go`

- [ ] **Step 1: Add `refreshTokenSvc` field and update `ChangePassword`**

Replace the `TutorHandler` struct and `NewTutorHandler`:
```go
type TutorHandler struct {
	service         service.TutorService
	refreshTokenSvc service.RefreshTokenService
	log             *slog.Logger
}

func NewTutorHandler(svc service.TutorService, refreshTokenSvc service.RefreshTokenService, log *slog.Logger) *TutorHandler {
	return &TutorHandler{service: svc, refreshTokenSvc: refreshTokenSvc, log: log}
}
```

Replace the `ChangePassword` method body — after `h.service.UpdatePassword` succeeds, add the revoke call:
```go
func (h *TutorHandler) ChangePassword(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	id := c.Param("id")
	if id != tutorID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}
	var req models.ChangePasswordRequest
	if !bindAndValidate(c, &req) {
		return
	}
	hash, err := h.service.GetPasswordHash(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tutor not found"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный текущий пароль"})
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}
	if err := h.service.UpdatePassword(c.Request.Context(), id, string(newHash)); err != nil {
		h.log.Error("Failed to update password", slog.String("id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}
	if err := h.refreshTokenSvc.RevokeAll(c.Request.Context(), id); err != nil {
		h.log.Error("Failed to revoke sessions", slog.String("id", id), slog.String("error", err.Error()))
	}
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 2: Build to verify**

```bash
go build ./...
```
Expected: compile error in `router/router.go` — `NewTutorHandler` signature changed. Will fix in next task.

---

### Task 8: Update router/router.go — wire everything

**Files:**
- Modify: `router/router.go`

- [ ] **Step 1: Add refreshTokenRepo and refreshTokenService, update handler constructors, add new routes**

After `tutorRepo := repository.NewTutorRepository(pool)` line, add:
```go
refreshTokenRepo := repository.NewRefreshTokenRepository(pool)
```

After `tutorService := service.NewTutorService(tutorRepo)` line, add:
```go
refreshTokenService := service.NewRefreshTokenService(refreshTokenRepo)
```

Replace `authHandler` and `tutorHandler` constructor calls:
```go
tutorHandler := handlers.NewTutorHandler(tutorService, refreshTokenService, log)
authHandler := handlers.NewAuthHandler(tutorService, refreshTokenService, log, cfg.JWTSecret, cfg.Env == "production")
```

After the existing public routes block, add two new routes:
```go
r.POST("/auth/refresh", authLimiter, authHandler.Refresh)
r.POST("/auth/logout", authLimiter, authHandler.Logout)
```

- [ ] **Step 2: Build to verify clean compile**

```bash
go build ./...
```
Expected: no output

- [ ] **Step 3: Run all tests**

```bash
go test ./...
```
Expected: all tests PASS

- [ ] **Step 4: Commit**

```bash
git add models/auth.go handlers/auth.go handlers/tutor.go router/router.go
git commit -m "feat: implement refresh token backend (handlers, routes)"
```

---

### Task 9: Frontend — update auth.ts API client

**Files:**
- Modify: `frontend/src/lib/api/auth.ts`

- [ ] **Step 1: Rename token field, add logout function**

Replace the entire file:
```typescript
import { api } from './client'
import { Tutor } from '@/types/api'

export interface LoginInput {
  email?: string
  phone?: string
  password: string
}

export interface RegisterInput {
  email: string
  password: string
  first_name: string
  last_name: string
  phone?: string
}

export const authApi = {
  login: (data: LoginInput) =>
    api.post<{ access_token: string }>('/auth/login', data).then((r) => r.data),

  register: (data: RegisterInput) =>
    api.post<Tutor>('/auth/register', data).then((r) => r.data),

  logout: () =>
    api.post('/auth/logout', {}, { withCredentials: true }).catch(() => {}),
}
```

---

### Task 10: Frontend — update login/page.tsx

**Files:**
- Modify: `frontend/src/app/(auth)/login/page.tsx`

- [ ] **Step 1: Use `access_token` instead of `token`**

Find and replace in `onSubmit`:
```typescript
// Before:
const { token } = await authApi.login(payload)
localStorage.setItem('tg_token', token)
const payloadPart = token.split('.')[1]

// After:
const { access_token } = await authApi.login(payload)
localStorage.setItem('tg_token', access_token)
const payloadPart = access_token.split('.')[1]
```

Also update `setAuth` call:
```typescript
// Before:
setAuth(token, user)

// After:
setAuth(access_token, user)
```

---

### Task 11: Frontend — update Sidebar.tsx logout handler

**Files:**
- Modify: `frontend/src/components/layout/Sidebar.tsx`

- [ ] **Step 1: Add authApi import and async logout handler**

Add import at the top of the file (with other imports):
```typescript
import { authApi } from '@/lib/api/auth'
```

Find the logout button area and replace:
```typescript
// Before (around line 301):
const { user, clearAuth } = useAuthStore()
// ...
onClick={clearAuth}

// After:
const { user, clearAuth } = useAuthStore()

async function handleLogout() {
  await authApi.logout()
  clearAuth()
}
// ...
onClick={handleLogout}
```

---

### Task 12: Frontend — update client.ts — 401 refresh retry

**Files:**
- Modify: `frontend/src/lib/api/client.ts`

- [ ] **Step 1: Replace the response interceptor with refresh logic**

Replace the entire file:
```typescript
import axios, { AxiosError } from 'axios'
import { ApiError } from '@/types/api'

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'

export const api = axios.create({
  baseURL: BASE_URL,
  headers: { 'Content-Type': 'application/json' },
})

api.interceptors.request.use((config) => {
  const token =
    typeof window !== 'undefined' ? localStorage.getItem('tg_token') : null
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

let isRefreshing = false

api.interceptors.response.use(
  (r) => r,
  async (error: AxiosError<{ error: string } | Record<string, string>>) => {
    const isAuthRoute = error.config?.url?.startsWith('/auth/')

    if (error.response?.status === 401 && !isAuthRoute && !isRefreshing) {
      isRefreshing = true
      try {
        // Use raw axios (not `api`) to avoid going through this interceptor again
        const { data } = await axios.post<{ access_token: string }>(
          `${BASE_URL}/auth/refresh`,
          {},
          { withCredentials: true },
        )
        localStorage.setItem('tg_token', data.access_token)
        isRefreshing = false

        if (error.config) {
          error.config.headers = error.config.headers ?? {}
          error.config.headers['Authorization'] = `Bearer ${data.access_token}`
          return api.request(error.config)
        }
      } catch {
        isRefreshing = false
        localStorage.removeItem('tg_token')
        localStorage.removeItem('tg_user')
        if (typeof window !== 'undefined') window.location.href = '/login'
      }
    }

    const status = error.response?.status ?? 0
    const data = error.response?.data

    let normalized: ApiError
    if (data && typeof data === 'object' && 'error' in data) {
      normalized = { message: data.error as string, status }
    } else if (data && typeof data === 'object') {
      normalized = {
        message: 'Validation error',
        fieldErrors: data as Record<string, string>,
        status,
      }
    } else {
      normalized = { message: 'Unknown error', status }
    }

    return Promise.reject(normalized)
  },
)
```

- [ ] **Step 2: Build frontend to verify TypeScript is clean**

```bash
cd frontend && npm run build
```
Expected: successful build, no TypeScript errors

- [ ] **Step 3: Commit all frontend changes**

```bash
git add frontend/src/lib/api/auth.ts frontend/src/lib/api/client.ts frontend/src/app/(auth)/login/page.tsx frontend/src/components/layout/Sidebar.tsx
git commit -m "feat: implement refresh token frontend (interceptor, logout, login)"
```

---

## Manual Verification Checklist

After completing all tasks, verify these flows manually:

- [ ] Login → check Network tab: response has `access_token`, cookie `refresh_token` is set (httpOnly)
- [ ] Wait 15 min (or manually set JWT exp to past) → make any API call → should auto-refresh and succeed
- [ ] Logout → cookie cleared, redirected to `/login`
- [ ] Login on two tabs → change password → both tabs should be logged out within 15 min (access token expiry)
- [ ] Call `POST /auth/refresh` without cookie → 401
