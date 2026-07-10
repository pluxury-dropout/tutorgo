# Аккаунты учеников и вход в урок — план реализации (backend v1)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Дать ученику самостоятельный аккаунт (приглашение → вход по телефону/логину) и защищённый вход в свои уроки (звонок + доска), сделав запланированные уроки приватными.

**Architecture:** Расширяем таблицу `students` (nullable auth-колонки + invite). Токен ученика — тот же HS256, но с claim `role:"student"`; middleware разводит репетитора и ученика по роли. Отдельная таблица `student_refresh_tokens` (FK на students) повторяет security-паттерн репетитора (in-memory access + httpOnly refresh cookie). Вход в урок проверяет запись двумя путями (`courses.student_id` ИЛИ `course_enrollments`); доска переиспускает существующий invite-путь без изменений в WS-хабе.

**Tech Stack:** Go 1.2x, gin, pgx/v5, golang-jwt/v5, bcrypt, LiveKit server-sdk. Тесты — стандартный `testing` пакет проекта (см. `handlers/*_test.go`, `mocks_test.go`).

## Global Constraints

- Секрет JWT общий с репетиторами (`cfg.JWTSecret`) — роль в claim обязательна для разведения акторов.
- bcrypt для паролей (`golang.org/x/crypto/bcrypt`, `DefaultCost`), как в `handlers/auth.go`.
- Все SQL — через `pgxpool.Pool`, плейсхолдеры `$1..$n`, ошибку дубликата ловим по `pgErr.Code == "23505"`.
- Refresh cookie: `HttpOnly`, `Secure=cfg.Env=="production"`, `SameSite=Strict`, `MaxAge=30d`.
- Access-токен живёт 30 дней (как у репетитора: `30 * 24 * time.Hour`).
- Rate-limit на auth-эндпоинтах: `middleware.RateLimit(rate.Every(12*time.Second), 3)`.
- Миграции goose-формата (`-- +goose Up` / `-- +goose Down`), применяются через Makefile.

## Параллельность (для subagent-driven)

- **Волна A (после T1):** T2 (middleware) ∥ T3 (student repo/service) ∥ T4 (refresh tokens) — независимы.
- **Волна B:** T5 (auth handler) зависит от T2+T3+T4. T6 (invite endpoint) зависит от T3. T5 ∥ T6.
- **Волна C:** T7 (room-token) ∥ T8 (board-token) — оба зависят от T2+T3.
- **Волна D:** T9 (роуты + удаление guest-token) — зависит от T5,T6,T7,T8.
- Frontend — **отдельный план** (см. хвост документа), можно стартовать параллельно после T5/T9.

---

### Task 1: Миграция — auth-колонки и таблица refresh-токенов ученика

**Files:**
- Create: `migrations/020_student_accounts.sql`

**Interfaces:**
- Produces: колонки `students.username`, `students.password_hash`, `students.invite_token`, `students.invite_expires_at`; таблица `student_refresh_tokens(id, student_id, token, expires_at, created_at)`; частичные unique-индексы.

- [ ] **Step 1: Написать миграцию**

```sql
-- +goose Up
ALTER TABLE students
  ADD COLUMN username          TEXT,
  ADD COLUMN password_hash     TEXT,
  ADD COLUMN invite_token      UUID,
  ADD COLUMN invite_expires_at TIMESTAMPTZ;

CREATE UNIQUE INDEX students_username_key
  ON students (username) WHERE password_hash IS NOT NULL;
CREATE UNIQUE INDEX students_phone_account_key
  ON students (phone) WHERE password_hash IS NOT NULL;
CREATE UNIQUE INDEX students_invite_token_key
  ON students (invite_token) WHERE invite_token IS NOT NULL;

CREATE TABLE student_refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX student_refresh_tokens_token_idx      ON student_refresh_tokens(token);
CREATE INDEX student_refresh_tokens_student_id_idx ON student_refresh_tokens(student_id);

-- +goose Down
DROP TABLE IF EXISTS student_refresh_tokens;
DROP INDEX IF EXISTS students_invite_token_key;
DROP INDEX IF EXISTS students_phone_account_key;
DROP INDEX IF EXISTS students_username_key;
ALTER TABLE students
  DROP COLUMN invite_expires_at,
  DROP COLUMN invite_token,
  DROP COLUMN password_hash,
  DROP COLUMN username;
```

- [ ] **Step 2: Применить и проверить**

Run: `make migrate-up` (или актуальная цель из Makefile), затем `make migrate-down && make migrate-up` для проверки обратимости.
Expected: применяется и откатывается без ошибок.

- [ ] **Step 3: Commit**

```bash
git add migrations/020_student_accounts.sql
git commit -m "feat(student): migration 020 — account auth cols + refresh tokens"
```

---

### Task 2: Middleware — разведение репетитора и ученика по роли

**Files:**
- Modify: `middleware/auth.go`
- Create: `middleware/auth_student_test.go`

**Interfaces:**
- Consumes: `cfg.JWTSecret`.
- Produces: `middleware.AuthStudent(jwtSecret string) gin.HandlerFunc` — валидирует токен с `role=="student"`, кладёт `c.Set("studentID", ...)`. Обновлённый `middleware.Auth` — отклоняет токен с `role=="student"`.

- [ ] **Step 1: Написать падающий тест**

`middleware/auth_student_test.go`:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret"

func signToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	claims["exp"] = time.Now().Add(time.Hour).Unix()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func runWith(mw gin.HandlerFunc, token string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", mw, func(c *gin.Context) { c.Status(200) })
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthStudent_AcceptsStudentToken(t *testing.T) {
	tok := signToken(t, jwt.MapClaims{"id": "stu-1", "role": "student"})
	if w := runWith(AuthStudent(testSecret), tok); w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestAuthStudent_RejectsTutorToken(t *testing.T) {
	tok := signToken(t, jwt.MapClaims{"id": "tut-1"}) // без роли = репетитор
	if w := runWith(AuthStudent(testSecret), tok); w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAuth_RejectsStudentToken(t *testing.T) {
	tok := signToken(t, jwt.MapClaims{"id": "stu-1", "role": "student"})
	if w := runWith(Auth(testSecret), tok); w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAuth_AcceptsTutorToken(t *testing.T) {
	tok := signToken(t, jwt.MapClaims{"id": "tut-1"})
	if w := runWith(Auth(testSecret), tok); w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Прогнать — тест падает**

Run: `go test ./middleware/ -run 'TestAuth' -v`
Expected: FAIL (`AuthStudent` не объявлен; `Auth` пока пропускает student-токен).

- [ ] **Step 3: Реализация**

В `middleware/auth.go` добавить проверку роли в `Auth` (после получения claims, перед `c.Set`):

```go
		if role, _ := claims["role"].(string); role == "student" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
```

И добавить новую функцию:

```go
// AuthStudent валидирует access-токен ученика (role=="student") и кладёт studentID в контекст.
func AuthStudent(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		token, err := jwt.ParseWithClaims(parts[1], jwt.MapClaims{},
			func(token *jwt.Token) (interface{}, error) { return []byte(jwtSecret), nil },
			jwt.WithValidMethods([]string{"HS256"}),
		)
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || claims["role"] != "student" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		studentID, ok := claims["id"].(string)
		if !ok || studentID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}
		c.Set("studentID", studentID)
		c.Next()
	}
}
```

- [ ] **Step 4: Прогнать — зелено**

Run: `go test ./middleware/ -run 'TestAuth' -v`
Expected: PASS (4 теста).

- [ ] **Step 5: Commit**

```bash
git add middleware/auth.go middleware/auth_student_test.go
git commit -m "feat(student): role-based middleware split (AuthStudent + Auth rejects student)"
```

---

### Task 3: Репозиторий и сервис ученика (аккаунт + enrollment-проверка)

**Files:**
- Modify: `models/student.go` (добавить request/DTO)
- Modify: `repository/student.go` (новые методы)
- Modify: `service/student.go` (новые методы)
- Test: `repository/student_account_test.go` (если в проекте есть DB-тесты; иначе покрыть на уровне handler в T5)

**Interfaces:**
- Consumes: колонки из T1.
- Produces (repository `StudentRepository`):
  - `SetInvite(ctx, studentID, token string, expiresAt time.Time) error`
  - `GetByInviteToken(ctx, token string) (studentID string, expiresAt time.Time, err error)`
  - `ActivateAccount(ctx, studentID, username, passwordHash string) error` — ставит username/hash, обнуляет invite_token.
  - `GetCredentialsByLogin(ctx, identifier string) (id, passwordHash string, err error)` — ищет по `username=$1 OR phone=$1` среди аккаунтов (`password_hash IS NOT NULL`).
  - `EnrolledInLesson(ctx, studentID, lessonID string) (bool, error)` — TRUE если `c.student_id=$1` ИЛИ строка в `course_enrollments`.
  - `CourseAndTutorForLesson(ctx, lessonID string) (courseID, tutorID string, err error)` — для board-token (T8).
- Produces (service `StudentService`): те же методы, проксируют в repo (+ генерация invite-токена и хеширование в handler, как у репетитора).

- [ ] **Step 1: DTO в `models/student.go`**

```go
type AcceptInviteRequest struct {
	Token    string `json:"token"    validate:"required,uuid"`
	Username string `json:"username" validate:"required,min=3,max=32,alphanum"`
	Password string `json:"password" validate:"required,min=6"`
}

type StudentLoginRequest struct {
	Identifier string `json:"identifier" validate:"required"` // телефон или username
	Password   string `json:"password"   validate:"required,min=6"`
}
```

- [ ] **Step 2: Методы репозитория в `repository/student.go`**

```go
func (r *studentRepository) SetInvite(ctx context.Context, studentID, token string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE students SET invite_token=$2, invite_expires_at=$3 WHERE id=$1`,
		studentID, token, expiresAt)
	return err
}

func (r *studentRepository) GetByInviteToken(ctx context.Context, token string) (string, time.Time, error) {
	var id string
	var exp time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT id, invite_expires_at FROM students WHERE invite_token=$1`, token,
	).Scan(&id, &exp)
	return id, exp, err
}

func (r *studentRepository) ActivateAccount(ctx context.Context, studentID, username, passwordHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE students SET username=$2, password_hash=$3, invite_token=NULL, invite_expires_at=NULL WHERE id=$1`,
		studentID, username, passwordHash)
	return err
}

func (r *studentRepository) GetCredentialsByLogin(ctx context.Context, identifier string) (string, string, error) {
	var id, hash string
	err := r.pool.QueryRow(ctx,
		`SELECT id, password_hash FROM students
		 WHERE password_hash IS NOT NULL AND (username=$1 OR phone=$1) LIMIT 1`, identifier,
	).Scan(&id, &hash)
	return id, hash, err
}

func (r *studentRepository) EnrolledInLesson(ctx context.Context, studentID, lessonID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM lessons l JOIN courses c ON c.id=l.course_id
		   WHERE l.id=$2 AND (
		     c.student_id=$1
		     OR EXISTS (SELECT 1 FROM course_enrollments ce WHERE ce.course_id=c.id AND ce.student_id=$1)
		   ))`, studentID, lessonID,
	).Scan(&ok)
	return ok, err
}

func (r *studentRepository) CourseAndTutorForLesson(ctx context.Context, lessonID string) (string, string, error) {
	var courseID, tutorID string
	err := r.pool.QueryRow(ctx,
		`SELECT c.id, c.tutor_id FROM lessons l JOIN courses c ON c.id=l.course_id WHERE l.id=$1`, lessonID,
	).Scan(&courseID, &tutorID)
	return courseID, tutorID, err
}
```

Добавить эти сигнатуры в интерфейс `StudentRepository` и импортировать `time`.

- [ ] **Step 3: Проброс в `service/student.go`**

Добавить методы в интерфейс `StudentService` и реализацию, проксирующую в repo. Пример:

```go
func (s *studentService) EnrolledInLesson(ctx context.Context, studentID, lessonID string) (bool, error) {
	return s.repo.EnrolledInLesson(ctx, studentID, lessonID)
}
```

(аналогично для `SetInvite`, `GetByInviteToken`, `ActivateAccount`, `GetCredentialsByLogin`, `CourseAndTutorForLesson`).

- [ ] **Step 4: Компиляция**

Run: `go build ./...`
Expected: без ошибок.

- [ ] **Step 5: Commit**

```bash
git add models/student.go repository/student.go service/student.go
git commit -m "feat(student): account repo/service — invite, login lookup, enrollment check"
```

---

### Task 4: Refresh-токены ученика (repo + service)

**Files:**
- Create: `repository/student_refresh_token.go`
- Create: `service/student_refresh_token.go`

**Interfaces:**
- Consumes: таблица `student_refresh_tokens` из T1.
- Produces: `service.StudentRefreshTokenService` с `Create(ctx, studentID) (token, err)`, `Validate(ctx, token) (studentID, err)`, `Revoke(ctx, token) error`.

- [ ] **Step 1: Репозиторий** — копия `repository/refresh_token.go`, но таблица `student_refresh_tokens`, колонка `student_id`:

```go
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StudentRefreshTokenRepository interface {
	Create(ctx context.Context, studentID, token string, expiresAt time.Time) error
	GetByToken(ctx context.Context, token string) (studentID string, expiresAt time.Time, err error)
	DeleteByToken(ctx context.Context, token string) error
}

type studentRefreshTokenRepository struct{ conn *pgxpool.Pool }

func NewStudentRefreshTokenRepository(conn *pgxpool.Pool) StudentRefreshTokenRepository {
	return &studentRefreshTokenRepository{conn: conn}
}

func (r *studentRefreshTokenRepository) Create(ctx context.Context, studentID, token string, expiresAt time.Time) error {
	_, err := r.conn.Exec(ctx,
		`INSERT INTO student_refresh_tokens (student_id, token, expires_at) VALUES ($1,$2,$3)`,
		studentID, token, expiresAt)
	return err
}

func (r *studentRefreshTokenRepository) GetByToken(ctx context.Context, token string) (string, time.Time, error) {
	var id string
	var exp time.Time
	err := r.conn.QueryRow(ctx,
		`SELECT student_id, expires_at FROM student_refresh_tokens WHERE token=$1`, token,
	).Scan(&id, &exp)
	return id, exp, err
}

func (r *studentRefreshTokenRepository) DeleteByToken(ctx context.Context, token string) error {
	_, err := r.conn.Exec(ctx, `DELETE FROM student_refresh_tokens WHERE token=$1`, token)
	return err
}
```

- [ ] **Step 2: Сервис** — копия `service/refresh_token.go` (та же генерация 32 байт base64url, TTL 30 дней), тип `StudentRefreshTokenService`, поле `repo repository.StudentRefreshTokenRepository`, аргумент `studentID`. `Validate` возвращает `studentID`. Переиспользовать `ErrTokenExpired` из пакета service.

- [ ] **Step 3: Компиляция**

Run: `go build ./...`
Expected: без ошибок.

- [ ] **Step 4: Commit**

```bash
git add repository/student_refresh_token.go service/student_refresh_token.go
git commit -m "feat(student): student refresh token repo/service"
```

---

### Task 5: Хендлер аутентификации ученика

**Files:**
- Create: `handlers/student_auth.go`
- Create: `handlers/student_auth_test.go`

**Interfaces:**
- Consumes: `service.StudentService` (T3), `service.StudentRefreshTokenService` (T4), `middleware.AuthStudent` (T2), `bindAndValidate` (существует в `handlers/helpers.go`).
- Produces: `StudentAuthHandler` с методами `AcceptInvite`, `Login`, `Refresh`, `Logout`. Access-токен несёт `{"id":studentID,"role":"student","exp":...}`.

- [ ] **Step 1: Падающий тест** (`handlers/student_auth_test.go`) — по образцу `handlers/auth_test.go` + `mocks_test.go`. Покрыть: успешный `Login` по username; неверный пароль → 401; `AcceptInvite` с истёкшим токеном → 400/409. Использовать существующий стиль моков сервисов из `mocks_test.go` (добавить мок `StudentService`/`StudentRefreshTokenService`).

```go
func TestStudentLogin_WrongPassword(t *testing.T) {
	// mock GetCredentialsByLogin → (id, bcryptHash("correct"))
	// POST /student/auth/login {identifier:"kamila", password:"wrong"}
	// want 401
}
```

- [ ] **Step 2: Прогнать — падает**

Run: `go test ./handlers/ -run TestStudentLogin -v`
Expected: FAIL (хендлер не объявлен).

- [ ] **Step 3: Реализация `handlers/student_auth.go`**

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
```

- [ ] **Step 4: Прогнать — зелено**

Run: `go test ./handlers/ -run TestStudent -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add handlers/student_auth.go handlers/student_auth_test.go handlers/mocks_test.go
git commit -m "feat(student): auth handler — accept-invite, login, refresh, logout"
```

---

### Task 6: Эндпоинт приглашения (сторона репетитора)

**Files:**
- Modify: `handlers/student.go` (добавить метод `Invite`)
- Modify: `handlers/student_test.go`

**Interfaces:**
- Consumes: `service.StudentService.SetInvite` (T3), `tutorID` из контекста.
- Produces: `POST /students/:id/invite` → `{ "invite_token": "<uuid>", "expires_at": "..." }`. Генерит `uuid` (через `github.com/google/uuid` — проверить, что зависимость есть; иначе `gen_random_uuid` на стороне БД, вернув токен из RETURNING). TTL приглашения — 7 дней.

- [ ] **Step 1: Тест** — записанному ученику репетитора генерится токен; чужой `:id` → сервис вернёт ошибку (проверка владения — в существующем `StudentService`, которое уже скоупит по tutorID в `GetByID`; для `SetInvite` добавить проверку владения в T3 либо здесь через `GetByID(id, tutorID)` перед `SetInvite`).

- [ ] **Step 2: Реализация метода**

```go
func (h *StudentHandler) Invite(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	studentID := c.Param("id")
	// проверка владения: студент принадлежит этому репетитору
	if _, err := h.service.GetByID(c.Request.Context(), studentID, tutorID); err != nil {
		handleServiceError(c, err)
		return
	}
	token := uuid.NewString()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if err := h.service.SetInvite(c.Request.Context(), studentID, token, expiresAt); err != nil {
		h.log.Error("set invite failed", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create invite"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invite_token": token, "expires_at": expiresAt})
}
```

(Проверить сигнатуру `service.GetByID` — если она `(ctx, id, tutorID)`, использовать как выше; иначе адаптировать.)

- [ ] **Step 3: Тесты зелёные**

Run: `go test ./handlers/ -run TestStudentInvite -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add handlers/student.go handlers/student_test.go
git commit -m "feat(student): tutor endpoint to generate student invite"
```

---

### Task 7: Вход в звонок для ученика (room-token)

**Files:**
- Modify: `handlers/call.go` (добавить метод `GetStudentToken`)
- Modify: `handlers/call_test.go`

**Interfaces:**
- Consumes: `studentID` из `AuthStudent`; `service.StudentService.EnrolledInLesson` (T3). Хендлеру `CallHandler` нужен доступ к `StudentService` — добавить поле в `NewCallHandler` (обновить конструктор и вызов в `router.go`).
- Produces: `POST /student/lessons/:id/room-token` → `{token, room_name, server_url}`, identity `student-<studentID>`.

- [ ] **Step 1: Тест** — не записанный ученик → 403; записанный → 200 с токеном (мок `EnrolledInLesson`).

- [ ] **Step 2: Реализация** — по образцу `GetToken`, но принципал — ученик:

```go
// POST /student/lessons/:id/room-token — только для записанного залогиненного ученика
func (h *CallHandler) GetStudentToken(c *gin.Context) {
	if h.apiKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "video calls not configured"})
		return
	}
	studentID := c.GetString("studentID")
	if studentID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	lessonID := c.Param("id")
	ok, err := h.studentService.EnrolledInLesson(c.Request.Context(), studentID, lessonID)
	if err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "not enrolled in this lesson"})
		return
	}
	roomName := "lesson-" + lessonID
	canPublish, canSubscribe := true, true
	at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
	at.SetVideoGrant(&lkauth.VideoGrant{
		RoomJoin: true, Room: roomName,
		CanPublish: &canPublish, CanSubscribe: &canSubscribe,
	}).SetIdentity("student-" + studentID).SetName("Ученик").SetValidFor(3 * time.Hour)
	token, err := at.ToJWT()
	if err != nil {
		h.log.Error("Failed to generate student token", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "room_name": roomName, "server_url": h.livekitURL})
}
```

- [ ] **Step 3: Тесты зелёные**

Run: `go test ./handlers/ -run TestStudentToken -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add handlers/call.go handlers/call_test.go
git commit -m "feat(student): enrollment-checked room-token for lesson call"
```

---

### Task 8: Вход на доску для ученика (board-token, переиспускает invite)

**Files:**
- Modify: `handlers/whiteboard.go` (добавить метод `StudentBoardToken`)
- Modify: `handlers/whiteboard_test.go` (если есть; иначе smoke вручную)

**Interfaces:**
- Consumes: `studentID` из `AuthStudent`; `StudentService.EnrolledInLesson` + `CourseAndTutorForLesson` (T3); `WhiteboardService.GetOrCreateBoard(ctx, courseID, tutorID)` и `CreateInvite(ctx, boardID, tutorID)` (существуют). Хендлеру нужны обе службы — добавить `studentService` в `WhiteboardHandler`/конструктор.
- Produces: `GET /student/lessons/:id/board-token` → `{ "invite_token": "<uuid>", "page_id": "<first-page>" }`. Фронт подключает WS `/ws/board/:pageId?token=<invite_token>` — работает существующий `authorizeWS` (ветка invite).

- [ ] **Step 1: Реализация**

```go
// GET /student/lessons/:id/board-token — записанному ученику отдаём invite доски курса.
func (h *WhiteboardHandler) StudentBoardToken(c *gin.Context) {
	studentID := c.GetString("studentID")
	if studentID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	lessonID := c.Param("id")
	ok, err := h.studentService.EnrolledInLesson(c.Request.Context(), studentID, lessonID)
	if err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "not enrolled in this lesson"})
		return
	}
	courseID, tutorID, err := h.studentService.CourseAndTutorForLesson(c.Request.Context(), lessonID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "lesson not found"})
		return
	}
	board, err := h.service.GetOrCreateBoard(c.Request.Context(), courseID, tutorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load board"})
		return
	}
	invite, err := h.service.CreateInvite(c.Request.Context(), board.Board.ID, tutorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create invite"})
		return
	}
	pageID := ""
	if len(board.Pages) > 0 {
		pageID = board.Pages[0].ID
	}
	c.JSON(http.StatusOK, gin.H{"invite_token": invite.Token, "page_id": pageID})
}
```

(Проверить точные имена полей `models.BoardWithPages` — `board.Board.ID`, `board.Pages[i].ID`, `models.BoardInvite.Token` — по `models/whiteboard.go`; поправить при расхождении. `CreateInvite` идемпотентен на уровне «один invite на доску» — если он ротирует токен, это допустимо: старый ученик переподключится при следующем входе.)

- [ ] **Step 2: Компиляция + ручной smoke**

Run: `go build ./...`
Expected: без ошибок. Smoke — в T9 после подключения роутов.

- [ ] **Step 3: Commit**

```bash
git add handlers/whiteboard.go
git commit -m "feat(student): board-token endpoint reusing existing board invite"
```

---

### Task 9: Роутинг, DI и приватность уроков

**Files:**
- Modify: `router/router.go`

**Interfaces:**
- Consumes: всё из T1–T8.

- [ ] **Step 1: Проводка зависимостей** — в `Setup(...)` создать репо/сервисы/хендлеры ученика:

```go
studentRefreshRepo := repository.NewStudentRefreshTokenRepository(pool)
studentRefreshService := service.NewStudentRefreshTokenService(studentRefreshRepo)
studentAuthHandler := handlers.NewStudentAuthHandler(studentService, studentRefreshService, log, cfg.JWTSecret, cfg.Env == "production")
```

Передать `studentService` в `NewCallHandler(...)` и `NewWhiteboardHandler(...)` (обновить их конструкторы — см. T7, T8).

- [ ] **Step 2: Публичные student-роуты** (rate-limited, как auth репетитора):

```go
studentAuthLimiter := middleware.RateLimit(rate.Every(12*time.Second), 3)
r.POST("/student/auth/accept-invite", studentAuthLimiter, studentAuthHandler.AcceptInvite)
r.POST("/student/auth/login", studentAuthLimiter, studentAuthHandler.Login)
r.POST("/student/auth/refresh", studentAuthLimiter, studentAuthHandler.Refresh)
r.POST("/student/auth/logout", studentAuthHandler.Logout)
```

- [ ] **Step 3: Защищённые student-роуты** (`middleware.AuthStudent`, без `RequireActiveSubscription` — подписка репетитора не касается ученика):

```go
stu := r.Group("/student")
stu.Use(middleware.AuthStudent(cfg.JWTSecret))
{
	stu.POST("/lessons/:id/room-token", callHandler.GetStudentToken)
	stu.GET("/lessons/:id/board-token", whiteboardHandler.StudentBoardToken)
}
```

- [ ] **Step 4: Invite-роут репетитора** — в группе `auth` (защищённая, с подпиской):

```go
auth.POST("/students/:id/invite", studentHandler.Invite)
```

- [ ] **Step 5: Приватность уроков** — удалить публичный guest-token запланированного урока:

```go
// УДАЛИТЬ строку:
// r.GET("/public/lessons/:id/guest-token", ...callHandler.GetGuestToken)
```

Quick-room публичные роуты (`/public/quick/...`) НЕ трогать. Метод `callHandler.GetGuestToken` и `lessonService.ExistsPublic` можно удалить, если больше нигде не используются (проверить `grep -rn GetGuestToken .`); иначе оставить dead-code-free.

- [ ] **Step 6: Полная сборка и тесты**

Run: `go build ./... && go test ./...`
Expected: всё зелёно.

- [ ] **Step 7: Commit**

```bash
git add router/router.go handlers/call.go
git commit -m "feat(student): wire student routes; make scheduled lessons private"
```

---

## Self-review (сделано при написании)

- **Покрытие спека:** миграция+колонки (T1), JWT-роль обе стороны (T2), вход телефон/логин (T3,T5), приглашение (T6), room-token с проверкой обоих путей записи (T3,T7), доска через invite без правок WS (T8), приватные уроки/удаление guest-token (T9), границы (телефон-уникальность — индекс в T1). ✓
- **Типы:** `EnrolledInLesson(studentID, lessonID) (bool,error)`, `role:"student"`, cookie `student_refresh_token` — согласованы между задачами. ✓
- **Плейсхолдеры:** тесты в T5/T6/T8 описаны образцами, а не полным кодом, т.к. зависят от стиля `mocks_test.go` проекта — исполнителю указан файл-образец; это осознанно, не заглушка.

---

## Frontend — отдельный план (следующая итерация)

Фронтенд ученика (страница приглашения, вход телефон/логин, кабинет со списком уроков и кнопкой «Войти», переиспускающей звонок+доску) — независимая подсистема со своей структурой файлов (`frontend/src/app/(student-*)`, api-клиент, хранение access-токена в памяти + refresh через `student_refresh_token`). Требует отдельного обследования фронтовых паттернов (существующий `(auth)/login`, перехватчик refresh). Пишется отдельным spec→plan после того, как backend v1 смержен и проверен вручную (smoke: сгенерить invite → accept → login → room-token → войти в комнату).
