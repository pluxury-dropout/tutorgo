# Student Backend Endpoints Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Дать залогиненному ученику API для кабинета: список уроков, профиль, смена пароля.

**Architecture:** Три вертикальных среза по паттерну фичи (repo → service → handler → router + тесты), все под `middleware.AuthStudent` в группе `/student`, без subscription-гейта. Спека: `docs/superpowers/specs/2026-07-12-student-backend-endpoints-design.md`.

**Tech Stack:** Go, Gin, pgxpool, testify/mock, bcrypt, golang-jwt.

## Global Constraints

- Все новые роуты — внутри `stu := r.Group("/student"); stu.Use(middleware.AuthStudent(...))`.
- `studentID` читать из контекста с nil-guard: `if c.GetString("studentID") == "" { 401 }`.
- Пустой список → `[]`, не `null` (инициализировать слайс).
- Тесты: service-слой `package service_test`, handler-слой `package handlers_test`, моки реализуют интерфейсы прямо в тест-файлах.
- **Параллелизм:** задачи 1–3 делят файлы `repository/student.go`, `service/student.go`, `handlers/mocks_test.go` — выполнять **последовательно** (fresh subagent на задачу), не параллельно. Безопасного параллелизма в этом бэкенде нет.

---

### Task 1: `GET /student/me` — профиль ученика

**Files:**
- Modify: `models/student.go` (добавить `StudentProfile`)
- Modify: `repository/student.go` (интерфейс + метод `GetProfile`)
- Modify: `service/student.go` (интерфейс + метод `GetProfile`)
- Modify: `handlers/student.go` (метод `Me`)
- Modify: `router/router.go` (роут)
- Test: `handlers/student_test.go`, `handlers/mocks_test.go`

**Interfaces:**
- Produces: `models.StudentProfile{FirstName, LastName, Phone, Username string}`; `StudentRepository.GetProfile(ctx, studentID string) (models.StudentProfile, error)`; `StudentService.GetProfile(...)` (та же сигнатура); `(*StudentHandler).Me(c *gin.Context)`.

- [ ] **Step 1: Добавить модель профиля** в `models/student.go` (в конец файла)

```go
type StudentProfile struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Username  string `json:"username"`
}
```

- [ ] **Step 2: Repo — добавить в интерфейс `StudentRepository`** (после `CourseAndTutorForLesson`)

```go
	GetProfile(ctx context.Context, studentID string) (models.StudentProfile, error)
```

- [ ] **Step 3: Repo — реализация** в `repository/student.go` (в конец файла)

```go
func (r *studentRepository) GetProfile(ctx context.Context, studentID string) (models.StudentProfile, error) {
	var p models.StudentProfile
	err := r.conn.QueryRow(ctx,
		`SELECT first_name, last_name, phone, COALESCE(username, '')
		 FROM students WHERE id = $1`, studentID,
	).Scan(&p.FirstName, &p.LastName, &p.Phone, &p.Username)
	return p, err
}
```

- [ ] **Step 4: Service — добавить в интерфейс `StudentService`** (после `CourseAndTutorForLesson`) и реализацию (в конец `service/student.go`)

```go
	GetProfile(ctx context.Context, studentID string) (models.StudentProfile, error)
```

```go
func (s *studentService) GetProfile(ctx context.Context, studentID string) (models.StudentProfile, error) {
	return s.repo.GetProfile(ctx, studentID)
}
```

- [ ] **Step 5: Handler — метод `Me`** в `handlers/student.go` (в конец файла)

```go
// GET /student/me — профиль текущего ученика.
func (h *StudentHandler) Me(c *gin.Context) {
	studentID := c.GetString("studentID")
	if studentID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	profile, err := h.service.GetProfile(c.Request.Context(), studentID)
	if err != nil {
		h.log.Error("student profile failed", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load profile"})
		return
	}
	c.JSON(http.StatusOK, profile)
}
```

- [ ] **Step 6: Роут** в `router/router.go` — внутри блока `stu` (после `stu.GET("/lessons/:id/board-token", ...)`)

```go
		stu.GET("/me", studentHandler.Me)
```

- [ ] **Step 7: Обновить мок** `mockStudentService` в `handlers/mocks_test.go` — добавить метод (рядом с прочими `mockStudentService`)

```go
func (m *mockStudentService) GetProfile(ctx context.Context, studentID string) (models.StudentProfile, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).(models.StudentProfile), args.Error(1)
}
```

- [ ] **Step 8: Написать падающий тест** в `handlers/student_test.go` (добавить). `makeRequest` не ставит studentID в контекст, а `StudentHandler` имеет неэкспортируемые поля (нельзя собрать литералом из тест-пакета) — поэтому строим хендлер через конструктор `handlers.NewStudentHandler(svc, slog.Default())` и инжектируем `studentID` middleware-функцией перед хендлером.

```go
func TestStudentMe_Success(t *testing.T) {
	svc := new(mockStudentService)
	h := handlers.NewStudentHandler(svc, slog.Default())
	r := gin.New()
	r.GET("/student/me", func(c *gin.Context) { c.Set("studentID", testStudentID); c.Next() }, h.Me)

	profile := models.StudentProfile{FirstName: "Kamila", LastName: "N", Phone: "+7700", Username: "kamila123"}
	svc.On("GetProfile", mock.Anything, testStudentID).Return(profile, nil)

	w := makeRequest(t, r, http.MethodGet, "/student/me", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.StudentProfile
	decodeJSON(t, w, &got)
	assert.Equal(t, "kamila123", got.Username)
	svc.AssertExpectations(t)
}
```

- [ ] **Step 9: Запустить тест — убедиться, что падает (нет метода/роута)**

Run: `go test ./handlers/ -run TestStudentMe_Success -v`
Expected: FAIL (компиляция или assert), пока хендлер/мок не готовы.

- [ ] **Step 10: Прогнать всё, убедиться, что зелёно**

Run: `go build ./... && go test ./handlers/ -run TestStudentMe_Success -v`
Expected: PASS

- [ ] **Step 11: Commit**

```bash
git add models/student.go repository/student.go service/student.go handlers/student.go handlers/student_test.go handlers/mocks_test.go router/router.go
git commit -m "feat(student): GET /student/me profile endpoint"
```

---

### Task 2: `GET /student/lessons?filter=upcoming|past` — список уроков

**Files:**
- Modify: `repository/student.go` (интерфейс + `ListLessons`)
- Modify: `service/student.go` (интерфейс + `ListLessons`)
- Modify: `handlers/student.go` (метод `ListLessons`)
- Modify: `router/router.go` (роут)
- Test: `handlers/student_test.go`, `handlers/mocks_test.go`

**Interfaces:**
- Consumes: существующая `models.CalendarLesson`.
- Produces: `StudentRepository.ListLessons(ctx, studentID string, past bool) ([]models.CalendarLesson, error)`; `StudentService.ListLessons(...)` (та же сигнатура); `(*StudentHandler).ListLessons(c *gin.Context)`.

- [ ] **Step 1: Repo — добавить в интерфейс `StudentRepository`**

```go
	ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error)
```

- [ ] **Step 2: Repo — реализация** в `repository/student.go` (в конец файла). Два query-строки по флагу `past` (boring over clever, без CASE в SQL).

```go
func (r *studentRepository) ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error) {
	// enrollment-джойн идентичен EnrolledInLesson: индивидуальный курс (c.student_id)
	// ИЛИ групповой через course_enrollments. Фильтр активности курса намеренно
	// опущен — ученик видит все свои уроки, включая архивные (история).
	base := `SELECT l.id, l.course_id, l.scheduled_at, l.duration_minutes, l.status,
	                l.notes, c.subject, s.first_name, (c.student_id IS NULL) AS is_group
	         FROM lessons l
	         JOIN courses c ON c.id = l.course_id
	         LEFT JOIN students s ON s.id = c.student_id
	         WHERE (c.student_id = $1
	                OR EXISTS (SELECT 1 FROM course_enrollments ce
	                           WHERE ce.course_id = c.id AND ce.student_id = $1))`
	var q string
	if past {
		q = base + ` AND l.scheduled_at < now() ORDER BY l.scheduled_at DESC`
	} else {
		q = base + ` AND l.scheduled_at >= now() ORDER BY l.scheduled_at ASC`
	}
	rows, err := r.conn.Query(ctx, q, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lessons := []models.CalendarLesson{}
	for rows.Next() {
		var l models.CalendarLesson
		if err := rows.Scan(&l.ID, &l.CourseID, &l.ScheduledAt, &l.DurationMinutes,
			&l.Status, &l.Notes, &l.Subject, &l.StudentName, &l.IsGroup); err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}
	return lessons, rows.Err()
}
```

- [ ] **Step 3: Service — интерфейс + реализация** в `service/student.go`

```go
	ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error)
```

```go
func (s *studentService) ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error) {
	return s.repo.ListLessons(ctx, studentID, past)
}
```

- [ ] **Step 4: Handler — метод `ListLessons`** в `handlers/student.go`

```go
// GET /student/lessons?filter=upcoming|past — уроки текущего ученика.
func (h *StudentHandler) ListLessons(c *gin.Context) {
	studentID := c.GetString("studentID")
	if studentID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	past := c.Query("filter") == "past" // всё, кроме "past", трактуем как upcoming
	lessons, err := h.service.ListLessons(c.Request.Context(), studentID, past)
	if err != nil {
		h.log.Error("student lessons failed", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load lessons"})
		return
	}
	c.JSON(http.StatusOK, lessons)
}
```

- [ ] **Step 5: Роут** в `router/router.go` — в блоке `stu`

```go
		stu.GET("/lessons", studentHandler.ListLessons)
```

- [ ] **Step 6: Мок** `mockStudentService` в `handlers/mocks_test.go`

```go
func (m *mockStudentService) ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error) {
	args := m.Called(ctx, studentID, past)
	return args.Get(0).([]models.CalendarLesson), args.Error(1)
}
```

- [ ] **Step 7: Падающие тесты** в `handlers/student_test.go`

```go
func TestStudentListLessons_Upcoming(t *testing.T) {
	svc := new(mockStudentService)
	h := handlers.NewStudentHandler(svc, slog.Default())
	r := gin.New()
	r.GET("/student/lessons", func(c *gin.Context) { c.Set("studentID", testStudentID); c.Next() }, h.ListLessons)

	svc.On("ListLessons", mock.Anything, testStudentID, false).
		Return([]models.CalendarLesson{{ID: "l1", Subject: "Math"}}, nil)

	w := makeRequest(t, r, http.MethodGet, "/student/lessons", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []models.CalendarLesson
	decodeJSON(t, w, &got)
	assert.Len(t, got, 1)
	assert.Equal(t, "Math", got[0].Subject)
	svc.AssertExpectations(t)
}

func TestStudentListLessons_PastFilter(t *testing.T) {
	svc := new(mockStudentService)
	h := handlers.NewStudentHandler(svc, slog.Default())
	r := gin.New()
	r.GET("/student/lessons", func(c *gin.Context) { c.Set("studentID", testStudentID); c.Next() }, h.ListLessons)

	// ?filter=past → past=true в сервис
	svc.On("ListLessons", mock.Anything, testStudentID, true).
		Return([]models.CalendarLesson{}, nil)

	w := makeRequest(t, r, http.MethodGet, "/student/lessons?filter=past", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t) // проверяет, что вызван именно с true
}
```

- [ ] **Step 8: Запустить — падает**

Run: `go test ./handlers/ -run TestStudentListLessons -v`
Expected: FAIL (нет метода/мока).

- [ ] **Step 9: Прогнать — зелено**

Run: `go build ./... && go test ./handlers/ -run TestStudentListLessons -v`
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add repository/student.go service/student.go handlers/student.go handlers/student_test.go handlers/mocks_test.go router/router.go
git commit -m "feat(student): GET /student/lessons with upcoming/past filter"
```

---

### Task 3: `POST /student/password` — смена пароля

**Files:**
- Modify: `models/student.go` (`ChangePasswordRequest`)
- Modify: `repository/student.go` (`GetPasswordHash`, `UpdatePassword`)
- Modify: `service/student.go` (те же в интерфейс + проброс)
- Modify: `repository/student_refresh_token.go` (`DeleteByStudentID`)
- Modify: `service/student_refresh_token.go` (`RevokeAll`)
- Modify: `handlers/student_auth.go` (`ChangePassword`)
- Modify: `router/router.go` (роут)
- Test: `handlers/student_auth_test.go`, `handlers/mocks_test.go`

**Interfaces:**
- Consumes: `(*StudentAuthHandler).issueSession(c, studentID)`, поля `h.service` (StudentService), `h.refreshSvc` (StudentRefreshTokenService).
- Produces: `models.ChangePasswordRequest{OldPassword, NewPassword string}`; `StudentRepository.GetPasswordHash(ctx, studentID) (string, error)`, `.UpdatePassword(ctx, studentID, hash string) error`; `StudentService` те же; `StudentRefreshTokenService.RevokeAll(ctx, studentID) error` (repo `DeleteByStudentID`); `(*StudentAuthHandler).ChangePassword(c)`.

- [ ] **Step 1: Модель запроса** в `models/student.go`

```go
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required,min=6"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}
```

- [ ] **Step 2: Repo — student: интерфейс + реализация**

```go
	GetPasswordHash(ctx context.Context, studentID string) (string, error)
	UpdatePassword(ctx context.Context, studentID, hash string) error
```

```go
func (r *studentRepository) GetPasswordHash(ctx context.Context, studentID string) (string, error) {
	var hash string
	err := r.conn.QueryRow(ctx,
		`SELECT COALESCE(password_hash, '') FROM students WHERE id = $1`, studentID,
	).Scan(&hash)
	return hash, err
}

func (r *studentRepository) UpdatePassword(ctx context.Context, studentID, hash string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE students SET password_hash = $2 WHERE id = $1`, studentID, hash)
	return err
}
```

- [ ] **Step 3: Repo — refresh: интерфейс + `DeleteByStudentID`** в `repository/student_refresh_token.go` (таблица DELETE-based, revoked_at нет)

```go
	DeleteByStudentID(ctx context.Context, studentID string) error
```

```go
func (r *studentRefreshTokenRepository) DeleteByStudentID(ctx context.Context, studentID string) error {
	_, err := r.conn.Exec(ctx, `DELETE FROM student_refresh_tokens WHERE student_id=$1`, studentID)
	return err
}
```

- [ ] **Step 4: Service — student: интерфейс + проброс**

```go
	GetPasswordHash(ctx context.Context, studentID string) (string, error)
	UpdatePassword(ctx context.Context, studentID, hash string) error
```

```go
func (s *studentService) GetPasswordHash(ctx context.Context, studentID string) (string, error) {
	return s.repo.GetPasswordHash(ctx, studentID)
}

func (s *studentService) UpdatePassword(ctx context.Context, studentID, hash string) error {
	return s.repo.UpdatePassword(ctx, studentID, hash)
}
```

- [ ] **Step 5: Service — refresh: интерфейс + `RevokeAll`** в `service/student_refresh_token.go`

```go
	RevokeAll(ctx context.Context, studentID string) error
```

```go
func (s *studentRefreshTokenService) RevokeAll(ctx context.Context, studentID string) error {
	return s.repo.DeleteByStudentID(ctx, studentID)
}
```

- [ ] **Step 6: Handler — `ChangePassword`** в `handlers/student_auth.go`. Порядок: bind → fetch hash → сверка старого → хеш нового → update → revoke all → issueSession (выдаёт свежий токен инициатору).

```go
func (h *StudentAuthHandler) ChangePassword(c *gin.Context) {
	studentID := c.GetString("studentID")
	if studentID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.ChangePasswordRequest
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
```

- [ ] **Step 7: Роут** в `router/router.go` — в блоке `stu`

```go
		stu.POST("/password", studentAuthHandler.ChangePassword)
```

- [ ] **Step 8: Моки** в `handlers/mocks_test.go` — расширить `mockStudentService` и `mockStudentRefreshTokenService`

```go
func (m *mockStudentService) GetPasswordHash(ctx context.Context, studentID string) (string, error) {
	args := m.Called(ctx, studentID)
	return args.String(0), args.Error(1)
}
func (m *mockStudentService) UpdatePassword(ctx context.Context, studentID, hash string) error {
	return m.Called(ctx, studentID, hash).Error(0)
}
func (m *mockStudentRefreshTokenService) RevokeAll(ctx context.Context, studentID string) error {
	return m.Called(ctx, studentID).Error(0)
}
```

- [ ] **Step 9: Падающие тесты** в `handlers/student_auth_test.go`. Роутер собираем как в `newStudentAuthRouter`, но с инъекцией `studentID` (роут защищён AuthStudent). Добавь локальный хелпер:

```go
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

	w := makeRequest(t, r, http.MethodPost, "/student/password", models.ChangePasswordRequest{
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

	w := makeRequest(t, r, http.MethodPost, "/student/password", models.ChangePasswordRequest{
		OldPassword: "oldpass", NewPassword: "newpass123",
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.LoginResponse
	decodeJSON(t, w, &got)
	assert.NotEmpty(t, got.AccessToken)
	svc.AssertExpectations(t)
	refreshSvc.AssertExpectations(t)
}
```

- [ ] **Step 10: Запустить — падает**

Run: `go test ./handlers/ -run TestStudentChangePassword -v`
Expected: FAIL.

- [ ] **Step 11: Прогнать всё — зелено**

Run: `go build ./... && go test ./service/ ./handlers/ -v 2>&1 | tail -20`
Expected: PASS (все пакеты).

- [ ] **Step 12: Commit**

```bash
git add models/student.go repository/student.go repository/student_refresh_token.go service/student.go service/student_refresh_token.go handlers/student_auth.go handlers/student_auth_test.go handlers/mocks_test.go router/router.go
git commit -m "feat(student): POST /student/password with session revocation"
```

---

## Итоговая проверка (после Task 3)

- [ ] `go build ./... && go vet ./... && go test ./...` — всё зелёное.
- [ ] Роуты `stu.GET("/me")`, `stu.GET("/lessons")`, `stu.POST("/password")` зарегистрированы под `AuthStudent`.
- [ ] Обновить память `project_student_accounts.md`: backend-эндпоинты кабинета закрыты, осталась frontend-спека (+ board WS live-auth внутри неё).
