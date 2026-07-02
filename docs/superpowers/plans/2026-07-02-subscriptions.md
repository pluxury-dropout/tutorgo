# Subscriptions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** SaaS-биллинг для репетиторов: бесплатный 30-дневный триал (без карты) → платный тариф (месяц/год-со-скидкой), грейс-период 7 дней → блокировка. Провайдер оплаты заглушён.

**Architecture:** Одна таблица `subscriptions` (1:1 с репетитором). Состояние доступа вычисляется на чтении из `period_end` (без крона, без хранимого `status`). Чистая функция `EffectiveState` — ядро, тестируется первой. Gin-middleware гейтит бизнес-ручки при `blocked`. Триал создаётся в одной транзакции с регистрацией. Старые репетиторы — grandfathered.

**Tech Stack:** Go, Gin, pgx/v5 + pgxpool, goose (миграции), testify/mock (тесты).

## Global Constraints

- Слои: `handlers → services → repositories → pgxpool`. Проводка только в `router/router.go`, без глобалов.
- Все данные scoped по `tutorID` из `c.GetString("tutorID")`.
- Repo принимает `*pgxpool.Pool` в конструкторе; методы под транзакцией принимают интерфейс `Querier` (его реализуют и `*pgxpool.Pool`, и `pgx.Tx`).
- Сервис-тесты: `package service_test`, mock-структуры реализуют интерфейс репозитория прямо в тест-файле.
- Grace = **7 дней**. Цены (плейсхолдеры, уточнить перед запуском): monthly = 5000 KZT, yearly = 48000 KZT.
- Инвариант: отсутствие строки подписки = `blocked`.
- Ошибки-ответы в стиле проекта: `c.JSON(status, gin.H{"error": "..."})`.

---

### Task 1: Чистая функция `EffectiveState` (ядро, TDD первой)

**Files:**
- Create: `service/subscription_state.go`
- Test: `service/subscription_state_test.go`
- Modify: `models/subscription.go` (создать — минимальная модель для функции)

**Interfaces:**
- Produces:
  - `models.Subscription{ TutorID string; Plan *string; PeriodEnd *time.Time; Grandfathered bool }`
  - `service.EffectiveState(sub *models.Subscription, now time.Time) string`
  - Константы `service.StateActive = "active"`, `service.StateGrace = "grace"`, `service.StateBlocked = "blocked"`
  - Константа `service.GraceDays = 7`

- [ ] **Step 1: Создать модель**

Create `models/subscription.go`:

```go
package models

import "time"

// Subscription — подписка репетитора на SaaS (1:1 с tutor).
type Subscription struct {
	TutorID       string     `json:"-"`
	Plan          *string    `json:"plan"`       // nil = пробный период
	PeriodEnd     *time.Time `json:"period_end"` // nil только для grandfathered
	Grandfathered bool       `json:"-"`
}
```

- [ ] **Step 2: Написать падающий тест**

Create `service/subscription_state_test.go`:

```go
package service_test

import (
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T { return &v }

func TestEffectiveState(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	end := now.Add(24 * time.Hour) // period_end через сутки
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name string
		sub  *models.Subscription
		want string
	}{
		{"nil row → blocked", nil, service.StateBlocked},
		{"grandfathered (period_end nil) → active",
			&models.Subscription{Grandfathered: true}, service.StateActive},
		{"before period_end → active",
			&models.Subscription{PeriodEnd: ptr(end)}, service.StateActive},
		{"exactly at period_end → grace",
			&models.Subscription{PeriodEnd: ptr(now)}, service.StateGrace},
		{"within grace → grace",
			&models.Subscription{PeriodEnd: ptr(past)}, service.StateGrace},
		{"past grace → blocked",
			&models.Subscription{PeriodEnd: ptr(now.Add(-8 * 24 * time.Hour))}, service.StateBlocked},
		{"non-grandfathered with nil period_end → blocked (safety)",
			&models.Subscription{PeriodEnd: nil}, service.StateBlocked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, service.EffectiveState(tt.sub, now))
		})
	}
}
```

- [ ] **Step 3: Запустить тест — убедиться, что падает**

Run: `go test ./service/ -run TestEffectiveState -v`
Expected: FAIL (undefined: service.EffectiveState / service.StateActive)

- [ ] **Step 4: Реализовать функцию**

Create `service/subscription_state.go`:

```go
package service

import (
	"time"

	"tutorgo/models"
)

const (
	StateActive  = "active"
	StateGrace   = "grace"
	StateBlocked = "blocked"

	GraceDays = 7
)

// EffectiveState вычисляет состояние доступа из period_end на момент now.
// Хранимого status нет — состояние всегда производное. См. spec.
func EffectiveState(sub *models.Subscription, now time.Time) string {
	if sub == nil {
		return StateBlocked // нет строки = блок
	}
	if sub.Grandfathered {
		return StateActive // short-circuit ДО сравнения дат (period_end == nil)
	}
	if sub.PeriodEnd == nil {
		return StateBlocked // не-grandfathered без даты = аномалия, безопасно блокируем
	}
	if now.Before(*sub.PeriodEnd) {
		return StateActive
	}
	if now.Before(sub.PeriodEnd.Add(GraceDays * 24 * time.Hour)) {
		return StateGrace
	}
	return StateBlocked
}
```

- [ ] **Step 5: Запустить тест — убедиться, что проходит**

Run: `go test ./service/ -run TestEffectiveState -v`
Expected: PASS (все 7 подтестов)

- [ ] **Step 6: Commit**

```bash
git add models/subscription.go service/subscription_state.go service/subscription_state_test.go
git commit -m "feat(subscriptions): EffectiveState pure function + tests"
```

---

### Task 2: Миграция 018 (таблица + grandfather-бэкфилл)

**Files:**
- Create: `migrations/018_subscriptions.sql`

**Interfaces:**
- Produces: таблица `subscriptions` со схемой из спека; строка `grandfathered=true` для каждого существующего репетитора.

- [ ] **Step 1: Написать миграцию**

Create `migrations/018_subscriptions.sql`:

```sql
-- +goose Up
CREATE TABLE subscriptions (
    tutor_id      UUID PRIMARY KEY REFERENCES tutors(id) ON DELETE CASCADE,
    plan          TEXT,
    period_end    TIMESTAMPTZ,
    grandfathered BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Бэкфилл: все существующие репетиторы — бесплатно навсегда.
INSERT INTO subscriptions (tutor_id, grandfathered)
SELECT id, TRUE FROM tutors
ON CONFLICT (tutor_id) DO NOTHING;

-- +goose Down
DROP TABLE subscriptions;
```

- [ ] **Step 2: Применить миграцию**

Run: `goose -dir migrations postgres "$DB_URL" up`
Expected: `OK   018_subscriptions.sql`

- [ ] **Step 3: Проверить бэкфилл (100% покрытие)**

Run:
```bash
psql "$DB_URL" -c "SELECT (SELECT count(*) FROM tutors) AS tutors, (SELECT count(*) FROM subscriptions) AS subs;"
```
Expected: `tutors` == `subs` (иначе непокрытый старый юзер будет залочен).

- [ ] **Step 4: Commit**

```bash
git add migrations/018_subscriptions.sql
git commit -m "feat(subscriptions): migration 018 (table + grandfather backfill)"
```

---

### Task 3: Модель-запросы + repository

**Files:**
- Create: `repository/subscription.go`
- Modify: `models/subscription.go` (добавить DTO статуса и request-структуры)

**Interfaces:**
- Consumes: `models.Subscription` (Task 1).
- Produces:
  - `repository.Querier` interface (Exec/Query/QueryRow) — реализуют `*pgxpool.Pool` и `pgx.Tx`.
  - `repository.SubscriptionRepository` interface:
    - `GetByTutor(ctx, tutorID string) (*models.Subscription, error)` — `nil, nil` если строки нет
    - `Activate(ctx, tutorID, plan string, periodEnd time.Time) error`
    - `CreateTrialTx(ctx, q Querier, tutorID string) error`
  - `repository.NewSubscriptionRepository(conn *pgxpool.Pool) SubscriptionRepository`
  - `models.SubscriptionStatus{ State string; Plan *string; PeriodEnd *time.Time; Prices Prices }`
  - `models.Prices{ Monthly int; Yearly int; Currency string }`
  - `models.SubscriptionPlanRequest{ Plan string }` (validate `oneof=monthly yearly`)

- [ ] **Step 1: Добавить DTO в модель**

Modify `models/subscription.go` — дописать в конец файла:

```go
type Prices struct {
	Monthly  int    `json:"monthly"`
	Yearly   int    `json:"yearly"`
	Currency string `json:"currency"`
}

type SubscriptionStatus struct {
	State     string     `json:"state"`
	Plan      *string    `json:"plan"`
	PeriodEnd *time.Time `json:"period_end"`
	Prices    Prices     `json:"prices"`
}

type SubscriptionPlanRequest struct {
	Plan string `json:"plan" validate:"required,oneof=monthly yearly"`
}
```

- [ ] **Step 2: Написать repository**

Create `repository/subscription.go`:

```go
package repository

import (
	"context"
	"errors"
	"time"

	"tutorgo/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier реализуют и *pgxpool.Pool, и pgx.Tx — позволяет одному методу
// работать как вне, так и внутри транзакции.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type SubscriptionRepository interface {
	GetByTutor(ctx context.Context, tutorID string) (*models.Subscription, error)
	Activate(ctx context.Context, tutorID, plan string, periodEnd time.Time) error
	CreateTrialTx(ctx context.Context, q Querier, tutorID string) error
}

type subscriptionRepository struct {
	conn *pgxpool.Pool
}

func NewSubscriptionRepository(conn *pgxpool.Pool) SubscriptionRepository {
	return &subscriptionRepository{conn: conn}
}

func (r *subscriptionRepository) GetByTutor(ctx context.Context, tutorID string) (*models.Subscription, error) {
	var s models.Subscription
	err := r.conn.QueryRow(ctx,
		`SELECT tutor_id, plan, period_end, grandfathered
		 FROM subscriptions WHERE tutor_id = $1`,
		tutorID,
	).Scan(&s.TutorID, &s.Plan, &s.PeriodEnd, &s.Grandfathered)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // нет строки — вызывающий трактует как blocked
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *subscriptionRepository) Activate(ctx context.Context, tutorID, plan string, periodEnd time.Time) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE subscriptions
		 SET plan = $2, period_end = $3, updated_at = now()
		 WHERE tutor_id = $1`,
		tutorID, plan, periodEnd,
	)
	return err
}

func (r *subscriptionRepository) CreateTrialTx(ctx context.Context, q Querier, tutorID string) error {
	_, err := q.Exec(ctx,
		`INSERT INTO subscriptions (tutor_id, plan, period_end, grandfathered)
		 VALUES ($1, NULL, now() + interval '30 days', FALSE)`,
		tutorID,
	)
	return err
}
```

- [ ] **Step 3: Проверить компиляцию**

Run: `go build ./...`
Expected: без ошибок.

- [ ] **Step 4: Commit**

```bash
git add repository/subscription.go models/subscription.go
git commit -m "feat(subscriptions): model DTOs + repository (Querier, GetByTutor, Activate, CreateTrialTx)"
```

---

### Task 4: Subscription service (TDD)

**Files:**
- Create: `service/subscription.go`
- Test: `service/subscription_test.go`

**Interfaces:**
- Consumes: `repository.SubscriptionRepository`, `service.EffectiveState`, `models.*`.
- Produces:
  - `service.SubscriptionService` interface:
    - `GetStatus(ctx, tutorID string) (models.SubscriptionStatus, error)`
    - `State(ctx, tutorID string) (string, error)`
    - `Checkout(ctx, tutorID, plan string) (string, error)` — возвращает checkout_url (заглушка)
    - `Confirm(ctx, tutorID, plan string) error`
  - `service.NewSubscriptionService(repo repository.SubscriptionRepository) SubscriptionService`
  - Константы цен: `PriceMonthly = 5000`, `PriceYearly = 48000`, `Currency = "KZT"`

- [ ] **Step 1: Написать падающий тест**

Create `service/subscription_test.go`:

```go
package service_test

import (
	"context"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockSubRepo struct{ mock.Mock }

func (m *mockSubRepo) GetByTutor(ctx context.Context, tutorID string) (*models.Subscription, error) {
	args := m.Called(ctx, tutorID)
	sub, _ := args.Get(0).(*models.Subscription)
	return sub, args.Error(1)
}
func (m *mockSubRepo) Activate(ctx context.Context, tutorID, plan string, periodEnd time.Time) error {
	args := m.Called(ctx, tutorID, plan, periodEnd)
	return args.Error(0)
}
func (m *mockSubRepo) CreateTrialTx(ctx context.Context, q repository.Querier, tutorID string) error {
	args := m.Called(ctx, q, tutorID)
	return args.Error(0)
}

func TestSubscriptionService_State_NoRowBlocked(t *testing.T) {
	repo := new(mockSubRepo)
	repo.On("GetByTutor", mock.Anything, "t1").Return((*models.Subscription)(nil), nil)
	svc := service.NewSubscriptionService(repo)

	state, err := svc.State(context.Background(), "t1")
	assert.NoError(t, err)
	assert.Equal(t, service.StateBlocked, state)
}

func TestSubscriptionService_GetStatus_IncludesPrices(t *testing.T) {
	repo := new(mockSubRepo)
	repo.On("GetByTutor", mock.Anything, "t1").Return(&models.Subscription{Grandfathered: true}, nil)
	svc := service.NewSubscriptionService(repo)

	st, err := svc.GetStatus(context.Background(), "t1")
	assert.NoError(t, err)
	assert.Equal(t, service.StateActive, st.State)
	assert.Equal(t, service.PriceMonthly, st.Prices.Monthly)
	assert.Equal(t, service.Currency, st.Prices.Currency)
}

func TestSubscriptionService_Confirm_ActivatesMonthly(t *testing.T) {
	repo := new(mockSubRepo)
	repo.On("Activate", mock.Anything, "t1", "monthly", mock.MatchedBy(func(pe time.Time) bool {
		// период ~30 дней вперёд
		return pe.After(time.Now().Add(29*24*time.Hour)) && pe.Before(time.Now().Add(31*24*time.Hour))
	})).Return(nil)
	svc := service.NewSubscriptionService(repo)

	err := svc.Confirm(context.Background(), "t1", "monthly")
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
```

Добавить импорт `"tutorgo/repository"` в блок импортов этого теста (нужен для `repository.Querier` в mock).

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `go test ./service/ -run TestSubscriptionService -v`
Expected: FAIL (undefined: service.NewSubscriptionService)

- [ ] **Step 3: Реализовать сервис**

Create `service/subscription.go`:

```go
package service

import (
	"context"
	"fmt"
	"time"

	"tutorgo/models"
	"tutorgo/repository"
)

const (
	PriceMonthly = 5000
	PriceYearly  = 48000
	Currency     = "KZT"
)

type SubscriptionService interface {
	GetStatus(ctx context.Context, tutorID string) (models.SubscriptionStatus, error)
	State(ctx context.Context, tutorID string) (string, error)
	Checkout(ctx context.Context, tutorID, plan string) (string, error)
	Confirm(ctx context.Context, tutorID, plan string) error
}

type subscriptionService struct {
	repo repository.SubscriptionRepository
}

func NewSubscriptionService(repo repository.SubscriptionRepository) SubscriptionService {
	return &subscriptionService{repo: repo}
}

func (s *subscriptionService) State(ctx context.Context, tutorID string) (string, error) {
	sub, err := s.repo.GetByTutor(ctx, tutorID)
	if err != nil {
		return "", err
	}
	return EffectiveState(sub, time.Now()), nil
}

func (s *subscriptionService) GetStatus(ctx context.Context, tutorID string) (models.SubscriptionStatus, error) {
	sub, err := s.repo.GetByTutor(ctx, tutorID)
	if err != nil {
		return models.SubscriptionStatus{}, err
	}
	st := models.SubscriptionStatus{
		State:  EffectiveState(sub, time.Now()),
		Prices: models.Prices{Monthly: PriceMonthly, Yearly: PriceYearly, Currency: Currency},
	}
	if sub != nil {
		st.Plan = sub.Plan
		st.PeriodEnd = sub.PeriodEnd
	}
	return st, nil
}

// Checkout — заглушка. Позже: создание платёжной сессии у провайдера.
func (s *subscriptionService) Checkout(ctx context.Context, tutorID, plan string) (string, error) {
	return fmt.Sprintf("https://pay.example.invalid/checkout?tutor=%s&plan=%s", tutorID, plan), nil
}

// Confirm — заглушка вместо вебхука провайдера: активирует подписку.
func (s *subscriptionService) Confirm(ctx context.Context, tutorID, plan string) error {
	days := 30
	if plan == "yearly" {
		days = 365
	}
	periodEnd := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	return s.repo.Activate(ctx, tutorID, plan, periodEnd)
}
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `go test ./service/ -run TestSubscriptionService -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add service/subscription.go service/subscription_test.go
git commit -m "feat(subscriptions): service (State/GetStatus/Checkout/Confirm) + tests"
```

---

### Task 5: Middleware `RequireActiveSubscription`

**Files:**
- Create: `middleware/subscription.go`

**Interfaces:**
- Consumes: `service.SubscriptionService` (использует `State`), `service.StateBlocked`.
- Produces: `middleware.RequireActiveSubscription(svc SubscriptionState) gin.HandlerFunc`, где
  `SubscriptionState interface { State(ctx, tutorID string) (string, error) }`.

- [ ] **Step 1: Реализовать middleware**

Create `middleware/subscription.go`:

```go
package middleware

import (
	"context"
	"net/http"

	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

// SubscriptionState — узкий интерфейс, чтобы middleware не зависел от всего сервиса.
type SubscriptionState interface {
	State(ctx context.Context, tutorID string) (string, error)
}

// RequireActiveSubscription блокирует бизнес-ручки при истёкшей подписке (402).
// Ставится ПОСЛЕ middleware.Auth (нужен tutorID в контексте).
func RequireActiveSubscription(svc SubscriptionState) gin.HandlerFunc {
	return func(c *gin.Context) {
		tutorID := c.GetString("tutorID")
		if tutorID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		state, err := svc.State(c.Request.Context(), tutorID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		if state == service.StateBlocked {
			c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{"error": "subscription_required"})
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 2: Проверить компиляцию**

Run: `go build ./...`
Expected: без ошибок.

- [ ] **Step 3: Commit**

```bash
git add middleware/subscription.go
git commit -m "feat(subscriptions): RequireActiveSubscription middleware (402 when blocked)"
```

---

### Task 6: Handlers + разбиение групп в router

**Files:**
- Create: `handlers/subscription.go`
- Modify: `router/router.go`

**Interfaces:**
- Consumes: `service.SubscriptionService`, `middleware.RequireActiveSubscription`.
- Produces: `handlers.NewSubscriptionHandler(svc service.SubscriptionService, log *slog.Logger) *SubscriptionHandler` с методами `GetStatus`, `Checkout`, `Confirm`.

- [ ] **Step 1: Реализовать handler**

Create `handlers/subscription.go`:

```go
package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type SubscriptionHandler struct {
	svc service.SubscriptionService
	log *slog.Logger
}

func NewSubscriptionHandler(svc service.SubscriptionService, log *slog.Logger) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc, log: log}
}

func (h *SubscriptionHandler) GetStatus(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	status, err := h.svc.GetStatus(c.Request.Context(), tutorID)
	if err != nil {
		h.log.Error("get subscription status", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *SubscriptionHandler) Checkout(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.SubscriptionPlanRequest
	if !bindAndValidate(c, &req) {
		return
	}
	url, err := h.svc.Checkout(c.Request.Context(), tutorID, req.Plan)
	if err != nil {
		h.log.Error("checkout", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"checkout_url": url})
}

func (h *SubscriptionHandler) Confirm(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.SubscriptionPlanRequest
	if !bindAndValidate(c, &req) {
		return
	}
	if err := h.svc.Confirm(c.Request.Context(), tutorID, req.Plan); err != nil {
		h.log.Error("confirm", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 2: Проводка в router — репо/сервис/хендлер**

Modify `router/router.go` — в блоке Repositories добавить:

```go
	subscriptionRepo := repository.NewSubscriptionRepository(pool)
```
в блоке Services добавить:
```go
	subscriptionService := service.NewSubscriptionService(subscriptionRepo)
```
в блоке Handlers добавить:
```go
	subscriptionHandler := handlers.NewSubscriptionHandler(subscriptionService, log)
```

- [ ] **Step 3: Разбить группу `auth` на открытую и гейтящую**

Modify `router/router.go` — внутри `auth := r.Group("/")` / `auth.Use(middleware.Auth(...))`:

Открытые даже при `blocked` (добавить ПЕРЕД созданием gated-группы):
```go
		// Subscription — доступно даже при истёкшей подписке (чтобы заплатить)
		auth.GET("/subscription", subscriptionHandler.GetStatus)
		auth.POST("/subscription/checkout", subscriptionHandler.Checkout)
		auth.POST("/subscription/confirm", subscriptionHandler.Confirm)
		// Профиль тоже открыт
		auth.GET("/tutors/:id", tutorHandler.GetByID)
		auth.PUT("/tutors/:id", tutorHandler.Update)
```

Затем создать gated-подгруппу и **перенести туда все остальные** существующие маршруты (students, courses, payments, lessons, enrollments, attendance, tasks, calls, whiteboard, а также `PUT /tutors/:id/password` и `DELETE /tutors/:id`):
```go
		gated := auth.Group("/")
		gated.Use(middleware.RequireActiveSubscription(subscriptionService))
		{
			gated.PUT("/tutors/:id/password", tutorHandler.ChangePassword)
			gated.DELETE("/tutors/:id", tutorHandler.Delete)
			gated.GET("/students", studentHandler.GetAll)
			// ...перенести сюда ВСЕ остальные ранее объявленные auth.* маршруты...
		}
```

Важно: удалить старые дублирующие объявления `auth.GET("/tutors/:id", ...)` и `auth.PUT("/tutors/:id", ...)` из gated-части (они теперь в открытой). Остальные `auth.X(...)` заменить на `gated.X(...)`.

- [ ] **Step 4: Проверить компиляцию и сборку**

Run: `go build ./... && go vet ./router/`
Expected: без ошибок.

- [ ] **Step 5: Ручная проверка гейта (blocked → 402)**

Run (после `./tmp/main.exe`, с токеном grandfathered-юзера — должно быть 200; для проверки 402 временно вставить в БД истёкшую подписку тестовому юзеру):
```bash
# grandfathered/активный — 200
curl -s -o /dev/null -w "%{http_code}\n" -H "Authorization: Bearer $TOKEN" localhost:8080/students
# subscription всегда доступен
curl -s -H "Authorization: Bearer $TOKEN" localhost:8080/subscription
```
Expected: `/students` → 200 для активного; `/subscription` → JSON со `state`, `prices`.

- [ ] **Step 6: Commit**

```bash
git add handlers/subscription.go router/router.go
git commit -m "feat(subscriptions): handlers + split auth into open/gated route groups"
```

---

### Task 7: Trial при регистрации (в одной транзакции)

**Files:**
- Modify: `repository/tutor.go` (добавить `CreateTx`, интерфейс `TutorRepository`)
- Modify: `service/tutor.go` (добавить `Register`, инжект pool + subRepo)
- Modify: `handlers/auth.go` (Register → `service.Register`)
- Modify: `router/router.go` (обновить `NewTutorService`)

**Interfaces:**
- Consumes: `repository.Querier`, `repository.SubscriptionRepository.CreateTrialTx`, `*pgxpool.Pool`.
- Produces:
  - `tutorRepository.CreateTx(ctx, q Querier, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error)`
  - `TutorService.Register(ctx, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error)`

- [ ] **Step 1: Добавить `CreateTx` в tutor repository**

Modify `repository/tutor.go` — добавить в интерфейс `TutorRepository`:
```go
	CreateTx(ctx context.Context, q Querier, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error)
```
Заменить тело существующего `Create` на делегирование + добавить `CreateTx`:
```go
func (r *tutorRepository) Create(ctx context.Context, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error) {
	return r.CreateTx(ctx, r.conn, req, passwordHash)
}

func (r *tutorRepository) CreateTx(ctx context.Context, q Querier, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error) {
	var tutor models.Tutor
	err := q.QueryRow(ctx,
		`INSERT INTO tutors (email, password_hash, first_name, last_name, phone)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, email, first_name, last_name, phone`,
		req.Email, passwordHash, req.FirstName, req.LastName, req.Phone,
	).Scan(&tutor.ID, &tutor.Email, &tutor.FirstName, &tutor.LastName, &tutor.Phone)
	return tutor, err
}
```

- [ ] **Step 2: Добавить `Register` в tutor service (транзакция)**

Modify `service/tutor.go`:

Обновить структуру и конструктор:
```go
type tutorService struct {
	repo    repository.TutorRepository
	subRepo repository.SubscriptionRepository
	pool    *pgxpool.Pool
}

func NewTutorService(repo repository.TutorRepository, subRepo repository.SubscriptionRepository, pool *pgxpool.Pool) TutorService {
	return &tutorService{repo: repo, subRepo: subRepo, pool: pool}
}
```
Добавить в интерфейс `TutorService`:
```go
	Register(ctx context.Context, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error)
```
Добавить метод:
```go
func (s *tutorService) Register(ctx context.Context, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Tutor{}, err
	}
	defer tx.Rollback(ctx) // no-op после Commit

	tutor, err := s.repo.CreateTx(ctx, tx, req, passwordHash)
	if err != nil {
		return models.Tutor{}, err
	}
	if err := s.subRepo.CreateTrialTx(ctx, tx, tutor.ID); err != nil {
		return models.Tutor{}, err
	}
	return tutor, tx.Commit(ctx)
}
```
Добавить импорт `"github.com/jackc/pgx/v5/pgxpool"`.

- [ ] **Step 3: Handler зовёт `Register`**

Modify `handlers/auth.go` — в методе `Register` заменить:
```go
	tutor, err := h.service.Create(c.Request.Context(), createReq, string(passwordHash))
```
на:
```go
	tutor, err := h.service.Register(c.Request.Context(), createReq, string(passwordHash))
```
(Обработка ошибки `23505` остаётся без изменений — unique-конфликт email/phone внутри транзакции откатит всё.)

- [ ] **Step 4: Обновить проводку**

Modify `router/router.go` — заменить:
```go
	tutorService := service.NewTutorService(tutorRepo)
```
на (после объявления `subscriptionRepo`):
```go
	tutorService := service.NewTutorService(tutorRepo, subscriptionRepo, pool)
```

- [ ] **Step 5: Проверить сборку и существующие тесты**

Run: `go build ./... && go test ./...`
Expected: сборка ок, все тесты PASS.

- [ ] **Step 6: Ручная проверка — регистрация создаёт trial**

Run (поднять сервер, зарегистрировать нового юзера):
```bash
curl -s -X POST localhost:8080/auth/register -H "Content-Type: application/json" \
  -d '{"email":"trial@test.kz","password":"secret1","first_name":"Aa","last_name":"Bb"}'
psql "$DB_URL" -c "SELECT plan, period_end, grandfathered FROM subscriptions WHERE tutor_id=(SELECT id FROM tutors WHERE email='trial@test.kz');"
```
Expected: строка с `plan=NULL`, `period_end` ≈ now+30d, `grandfathered=false`.

- [ ] **Step 7: Commit**

```bash
git add repository/tutor.go service/tutor.go handlers/auth.go router/router.go
git commit -m "feat(subscriptions): create trial subscription in same tx as registration"
```

---

## Self-Review

**Spec coverage:**
- Таблица `subscriptions` → Task 2 ✓
- Функция доступа (compute-on-read, grace 7д, grandfather short-circuit, nil=blocked) → Task 1 ✓
- Middleware + список открытых роутов (subscription, GET/PUT tutors) → Task 5, Task 6 ✓
- `GET /subscription` (state+prices), `checkout`, `confirm` (заглушки) → Task 4, Task 6 ✓
- Цены-константы KZT → Task 4 ✓
- Trial при регистрации в одной транзакции → Task 7 ✓
- Миграция 018 + grandfather-бэкфилл + проверка 100% → Task 2 ✓
- Не делаем: крон, PaymentProvider-интерфейс, cancel, plans-таблица, email — отражено (ничего для них не создаётся) ✓

**Type consistency:** `EffectiveState(*models.Subscription, time.Time) string` — един везде; `State`/`GetStatus`/`Checkout`/`Confirm` совпадают между сервисом (Task 4), middleware (Task 5), handler (Task 6); `Querier` определён в Task 3 и используется в Task 3/7; `CreateTrialTx(ctx, Querier, tutorID)` совпадает в repo (Task 3), mock (Task 4), service.Register (Task 7).

**Placeholder scan:** цены помечены как плейсхолдеры осознанно (Global Constraints); код в шагах полный, TODO нет.
