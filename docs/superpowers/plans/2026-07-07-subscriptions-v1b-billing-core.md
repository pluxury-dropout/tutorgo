# Subscriptions v1b — Billing State Machine (Implementation Plan)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Заменить заглушку оплаты подписки (`Confirm` выдаёт платный период без проверки) биллинг-стейт-машиной: токенизация (CIT) → webhook-активация → автопродление по токену (MIT) → самообслуживание (cancel, change-plan) — против **фейкового** `PaymentProvider`.

**Architecture:** Провайдер спрятан за одним интерфейсом `service.PaymentProvider` (единственный внешний I/O-seam, мокается как репозитории). Идемпотентность денег живёт в БД: `subscription_payments` с `UNIQUE(order_id)`; активация — условный `UPDATE ... WHERE status='pending'`, повтор → `rows=0` → no-op. MIT-конкурентность решается по-дневным `INSERT ON CONFLICT DO NOTHING` (claim-as-mutex), без `FOR UPDATE`.

**Tech Stack:** Go, Gin, pgx/v5, goose (migrations), testify/mock (тесты), stdlib.

**Scope (важно):** Этот план строит **только backend state machine против фейкового провайдера**. Шипнуть его НЕ значит включить приём денег. Отдельными планами (гейтятся аппрувом Freedom Pay + доками):
- реальный `freedompay`-адаптер (`InitPayment`/`Charge`/`ParseCallback` + проверка `pg_sig`);
- frontend (убрать клиентский `confirm`-POST, success/fail-страницы с поллингом, кнопки cancel/change-plan);
- `change-card` (tokenize-only, зависит от способа токенизации без списания у Freedom Pay).

**Cross-plan dependency:** Task 7 удаляет роут `POST /subscription/confirm`, который сейчас дёргает frontend. НЕ деплоить этот backend в прод раньше frontend-плана — иначе клиент 404-ит на confirm. Порядок выката: frontend → backend.

**Спека:** `docs/superpowers/specs/2026-07-07-subscriptions-v1b-design.md`

## Global Constraints

- `PriceMonthly = 10000`, `PriceYearly = 90000`, `Currency = "KZT"` — уже в `service/subscription.go:12-16`, не менять.
- Интервалы периодов: monthly = 30 дней, yearly = 365 дней (как в текущем `Confirm`).
- Grace = 7 дней (`service.GraceDays`), логику `EffectiveState` НЕ трогать.
- Все SQL через `Querier`-совместимые вызовы на `*pgxpool.Pool` (см. `repository/subscription.go`).
- `tutorID` в хендлерах — читать из Gin-контекста с nil-guard (паттерн `handlers/subscription.go`).
- Роуты подписки (кроме webhook) — в группе `open` (авторизованы, но ВНЕ `RequireActiveSubscription`). Webhook — на top-level `r`, ВНЕ JWT (как `/webhooks/livekit`).
- `amount int` — в тенге (по докам FreedomPay `pg_amount` десятичное в валютных единицах, KZT без субъединиц). `×100` НЕ нужен. Фейк-провайдер к единицам безразличен.

---

### Task 1: Migration 019 — токен-колонки + таблица платежей

**Files:**
- Create: `migrations/019_subscription_payments.sql`

**Interfaces:**
- Produces: колонки `subscriptions.card_token TEXT`, `subscriptions.autopay BOOLEAN`, `subscriptions.pending_plan TEXT`; таблица `subscription_payments(id, tutor_id, provider_payment_id UNIQUE, order_id UNIQUE, plan, amount, status, created_at)`.

- [ ] **Step 1: Написать миграцию**

```sql
-- +goose Up
ALTER TABLE subscriptions
  ADD COLUMN card_token   TEXT,
  ADD COLUMN autopay      BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN pending_plan TEXT;

CREATE TABLE subscription_payments (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tutor_id            UUID NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
  provider_payment_id TEXT UNIQUE,          -- NULL до success; второй рубеж дедупа
  order_id            TEXT NOT NULL UNIQUE, -- ключ корреляции CIT/MIT + claim-мьютекс
  plan                TEXT NOT NULL,
  amount              INTEGER NOT NULL,
  status              TEXT NOT NULL,        -- pending | success | failed
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE subscription_payments;
ALTER TABLE subscriptions
  DROP COLUMN card_token,
  DROP COLUMN autopay,
  DROP COLUMN pending_plan;
```

- [ ] **Step 2: Проверить синтаксис goose (если доступна локальная БД)**

Run: `goose -dir migrations postgres "$DB_URL" up` затем `goose -dir migrations postgres "$DB_URL" down` затем `up`.
Expected: `OK 019_subscription_payments.sql` в обе стороны без ошибок.
Если БД недоступна (sandbox) — визуально сверить `-- +goose Up/Down` парность и что `Down` откатывает всё из `Up`. Применение к прод-Supabase — вручную пользователем, как migration 018.

- [ ] **Step 3: Commit**

```bash
git add migrations/019_subscription_payments.sql
git commit -m "feat(subscriptions): migration 019 — card token + subscription_payments"
```

---

### Task 2: PaymentProvider interface + Callback + StubProvider

**Files:**
- Create: `service/payment_provider.go`
- Test: `service/payment_provider_test.go`

**Interfaces:**
- Produces:
  - `type service.Callback struct { OrderID, ProviderPaymentID, Status, CardToken string }` (`Status` ∈ `"success"|"failed"`)
  - `type service.PaymentProvider interface { InitPayment(ctx, orderID, tutorID, plan string, amount int) (string, error); Charge(ctx, orderID, token string, amount int) (string, error); ParseCallback(r *http.Request) (Callback, error) }`
  - `type service.StubProvider struct{}` — реализует интерфейс, для router-проводки до появления freedompay.

- [ ] **Step 1: Написать интерфейс, Callback и StubProvider**

```go
package service

import (
	"context"
	"errors"
	"net/http"
)

// Callback — провайдер-нейтральный разбор webhook/ответа платёжного провайдера.
type Callback struct {
	OrderID           string
	ProviderPaymentID string
	Status            string // "success" | "failed"
	CardToken         string // непусто на success CIT (галка "сохранить карту")
}

// PaymentProvider — единственный I/O-seam к платёжному провайдеру.
// Реализации: StubProvider (пока нет freedompay), freedompay (отдельный план),
// fakeProvider (тесты).
type PaymentProvider interface {
	// CIT: hosted-страница первой оплаты, возвращает redirect URL.
	InitPayment(ctx context.Context, orderID, tutorID, plan string, amount int) (redirectURL string, err error)
	// MIT: списание по сохранённому токену.
	Charge(ctx context.Context, orderID, token string, amount int) (providerPaymentID string, err error)
	// Reconciliation при ошибке Charge (провайдерский status_v2).
	// Возвращает "success" | "failed" | "unknown".
	CheckStatus(ctx context.Context, orderID string) (status string, err error)
	// Разбор + проверка подписи входящего webhook.
	ParseCallback(r *http.Request) (Callback, error)
}

// StubProvider — временная заглушка для проводки в router, пока не готов
// freedompay-адаптер (отдельный план).
// ponytail: stub до freedompay-адаптера — НЕ деплоить в прод с ним.
type StubProvider struct{}

var errStubProvider = errors.New("payment provider not configured")

func (StubProvider) InitPayment(context.Context, string, string, string, int) (string, error) {
	return "", errStubProvider
}
func (StubProvider) Charge(context.Context, string, string, int) (string, error) {
	return "", errStubProvider
}
func (StubProvider) CheckStatus(context.Context, string) (string, error) {
	return "unknown", errStubProvider
}
func (StubProvider) ParseCallback(*http.Request) (Callback, error) {
	return Callback{}, errStubProvider
}
```

- [ ] **Step 2: Написать тест — StubProvider удовлетворяет интерфейсу и возвращает ошибку**

```go
package service_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"tutorgo/service"

	"github.com/stretchr/testify/assert"
)

func TestStubProvider_SatisfiesInterfaceAndErrors(t *testing.T) {
	var p service.PaymentProvider = service.StubProvider{}

	_, err := p.InitPayment(context.Background(), "o1", "t1", "monthly", 10000)
	assert.Error(t, err)
	_, err = p.Charge(context.Background(), "o1", "tok", 10000)
	assert.Error(t, err)
	_, err = p.CheckStatus(context.Background(), "o1")
	assert.Error(t, err)
	_, err = p.ParseCallback(httptest.NewRequest("POST", "/subscription/webhook", nil))
	assert.Error(t, err)
}
```

- [ ] **Step 3: Запустить тест**

Run: `go test ./service/ -run TestStubProvider -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add service/payment_provider.go service/payment_provider_test.go
git commit -m "feat(subscriptions): PaymentProvider interface + Callback + StubProvider"
```

---

### Task 3: Repository — платёжные методы + модели + integration-тест атомарности

**Files:**
- Modify: `models/subscription.go` (add structs)
- Modify: `repository/subscription.go` (extend interface + SQL impl)
- Test: `repository/subscription_integration_test.go` (build tag `integration`)
- Modify: `service/subscription_test.go` (add stub methods to `mockSubRepo` so it still satisfies the extended interface — otherwise `go test ./service/` won't compile until T4)

**Interfaces:**
- Consumes: `models.Subscription` (task-независимо), `repository.Querier`.
- Produces (на `SubscriptionRepository`):
  - `InsertPendingPayment(ctx, tutorID, orderID, plan string, amount int) (claimed bool, err error)` — `claimed` = вставилось (`RowsAffected()==1`).
  - `MarkPaymentSuccess(ctx, orderID, providerPaymentID string) (activated bool, err error)` — `activated` = `RowsAffected()==1` (первый success).
  - `MarkPaymentFailed(ctx, orderID string) error`
  - `GetPaymentByOrderID(ctx, orderID string) (*models.SubscriptionPayment, error)` — `nil, nil` если нет строки.
  - `StartPaidPeriod(ctx, tutorID, plan, cardToken string, periodEnd time.Time) error` — CIT: токен + autopay=true + период.
  - `RenewPeriod(ctx, tutorID, plan string, periodEnd time.Time) error` — MIT: plan + period_end + `pending_plan=NULL`.
  - `ListDueAutopay(ctx, now time.Time) ([]models.DueSubscription, error)`
  - `Cancel(ctx, tutorID string) error` — `autopay=false, card_token=NULL`.
  - `SetPendingPlan(ctx, tutorID, plan string) error`
  - `type models.SubscriptionPayment struct { TutorID, OrderID, Plan string; Amount int; Status string }`
  - `type models.DueSubscription struct { TutorID, Plan string; PendingPlan *string; CardToken string; PeriodEnd time.Time }`

- [ ] **Step 1: Добавить модели**

В `models/subscription.go` дописать:

```go
// SubscriptionPayment — строка лога платежей (аудит + идемпотентность).
type SubscriptionPayment struct {
	TutorID string
	OrderID string
	Plan    string
	Amount  int
	Status  string
}

// DueSubscription — подписка, у которой пора списывать автопродление.
type DueSubscription struct {
	TutorID     string
	Plan        string
	PendingPlan *string
	CardToken   string
	PeriodEnd   time.Time
}
```

- [ ] **Step 2: Расширить интерфейс + реализацию репозитория**

В `repository/subscription.go` в `SubscriptionRepository` добавить сигнатуры из блока Produces, затем методы:

```go
func (r *subscriptionRepository) InsertPendingPayment(ctx context.Context, tutorID, orderID, plan string, amount int) (bool, error) {
	tag, err := r.conn.Exec(ctx,
		`INSERT INTO subscription_payments (tutor_id, order_id, plan, amount, status)
		 VALUES ($1, $2, $3, $4, 'pending')
		 ON CONFLICT (order_id) DO NOTHING`,
		tutorID, orderID, plan, amount,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *subscriptionRepository) MarkPaymentSuccess(ctx context.Context, orderID, providerPaymentID string) (bool, error) {
	tag, err := r.conn.Exec(ctx,
		`UPDATE subscription_payments
		 SET status = 'success', provider_payment_id = $2
		 WHERE order_id = $1 AND status = 'pending'`,
		orderID, providerPaymentID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *subscriptionRepository) MarkPaymentFailed(ctx context.Context, orderID string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE subscription_payments SET status = 'failed'
		 WHERE order_id = $1 AND status = 'pending'`,
		orderID,
	)
	return err
}

func (r *subscriptionRepository) GetPaymentByOrderID(ctx context.Context, orderID string) (*models.SubscriptionPayment, error) {
	var p models.SubscriptionPayment
	err := r.conn.QueryRow(ctx,
		`SELECT tutor_id, order_id, plan, amount, status
		 FROM subscription_payments WHERE order_id = $1`,
		orderID,
	).Scan(&p.TutorID, &p.OrderID, &p.Plan, &p.Amount, &p.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *subscriptionRepository) StartPaidPeriod(ctx context.Context, tutorID, plan, cardToken string, periodEnd time.Time) error {
	tag, err := r.conn.Exec(ctx,
		`UPDATE subscriptions
		 SET plan = $2, period_end = $3, card_token = $4, autopay = TRUE, updated_at = now()
		 WHERE tutor_id = $1`,
		tutorID, plan, periodEnd, cardToken,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("subscription not found")
	}
	return nil
}

func (r *subscriptionRepository) RenewPeriod(ctx context.Context, tutorID, plan string, periodEnd time.Time) error {
	tag, err := r.conn.Exec(ctx,
		`UPDATE subscriptions
		 SET plan = $2, period_end = $3, pending_plan = NULL, updated_at = now()
		 WHERE tutor_id = $1`,
		tutorID, plan, periodEnd,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("subscription not found")
	}
	return nil
}

func (r *subscriptionRepository) ListDueAutopay(ctx context.Context, now time.Time) ([]models.DueSubscription, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT tutor_id, plan, pending_plan, card_token, period_end
		 FROM subscriptions
		 WHERE autopay = TRUE AND card_token IS NOT NULL AND period_end <= $1`,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.DueSubscription
	for rows.Next() {
		var d models.DueSubscription
		var plan *string
		if err := rows.Scan(&d.TutorID, &plan, &d.PendingPlan, &d.CardToken, &d.PeriodEnd); err != nil {
			return nil, err
		}
		if plan != nil {
			d.Plan = *plan
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *subscriptionRepository) Cancel(ctx context.Context, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE subscriptions
		 SET autopay = FALSE, card_token = NULL, pending_plan = NULL, updated_at = now()
		 WHERE tutor_id = $1`,
		tutorID,
	)
	return err
}

func (r *subscriptionRepository) SetPendingPlan(ctx context.Context, tutorID, plan string) error {
	tag, err := r.conn.Exec(ctx,
		`UPDATE subscriptions SET pending_plan = $2, updated_at = now()
		 WHERE tutor_id = $1`,
		tutorID, plan,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("subscription not found")
	}
	return nil
}
```

- [ ] **Step 3: Написать integration-тест атомарности (build tag `integration`)**

Файл `repository/subscription_integration_test.go` — проверяет главный money-инвариант, который моки НЕ видят. Пропускается в `go test ./...` (нет тега / нет БД).

```go
//go:build integration

package repository_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"tutorgo/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Требует применённой migration 019 и репетитора-фикстуры.
// Запуск: go test -tags=integration ./repository/ -run TestClaim
func connect(t *testing.T) *pgxpool.Pool {
	url := os.Getenv("DB_URL")
	if url == "" {
		t.Skip("DB_URL не задан — пропускаем integration-тест")
	}
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func TestClaim_InsertPending_OnlyOneWinsConcurrently(t *testing.T) {
	pool := connect(t)
	repo := repository.NewSubscriptionRepository(pool)
	ctx := context.Background()
	tutorID := seedTutorWithSubscription(t, pool) // helper: INSERT tutor + subscription, вернуть id
	orderID := tutorID + ":claim-test"
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM subscription_payments WHERE order_id=$1`, orderID) })

	const n = 8
	var wg sync.WaitGroup
	claims := make([]bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			claimed, err := repo.InsertPendingPayment(ctx, tutorID, orderID, "monthly", 10000)
			assert.NoError(t, err)
			claims[i] = claimed
		}(i)
	}
	wg.Wait()

	won := 0
	for _, c := range claims {
		if c {
			won++
		}
	}
	assert.Equal(t, 1, won, "ровно один инстанс должен выиграть insert-claim")
}

func TestClaim_MarkSuccess_OnlyOneActivatesConcurrently(t *testing.T) {
	pool := connect(t)
	repo := repository.NewSubscriptionRepository(pool)
	ctx := context.Background()
	tutorID := seedTutorWithSubscription(t, pool)
	orderID := tutorID + ":success-test"
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM subscription_payments WHERE order_id=$1`, orderID) })
	_, err := repo.InsertPendingPayment(ctx, tutorID, orderID, "monthly", 10000)
	require.NoError(t, err)

	const n = 8
	var wg sync.WaitGroup
	acts := make([]bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			activated, err := repo.MarkPaymentSuccess(ctx, orderID, "pp-"+orderID)
			assert.NoError(t, err)
			acts[i] = activated
		}(i)
	}
	wg.Wait()

	won := 0
	for _, a := range acts {
		if a {
			won++
		}
	}
	assert.Equal(t, 1, won, "ровно один вызов должен активировать период")
}
```

> `seedTutorWithSubscription` — маленький helper в этом же файле: `INSERT INTO tutors ...; INSERT INTO subscriptions (tutor_id, plan, period_end) VALUES ($1,'monthly', now())`, возвращает UUID, регистрирует cleanup. Написать по образцу существующих INSERT в migration 018.

- [ ] **Step 4: Добавить stub-методы в `mockSubRepo` (чтобы service-тесты компилировались)**

В `service/subscription_test.go` в `mockSubRepo` дописать заглушки для всех новых методов интерфейса (реальную настройку `.On(...)` добавит T4/T5):

```go
func (m *mockSubRepo) InsertPendingPayment(ctx context.Context, tutorID, orderID, plan string, amount int) (bool, error) {
	args := m.Called(ctx, tutorID, orderID, plan, amount)
	return args.Bool(0), args.Error(1)
}
func (m *mockSubRepo) MarkPaymentSuccess(ctx context.Context, orderID, ppid string) (bool, error) {
	args := m.Called(ctx, orderID, ppid)
	return args.Bool(0), args.Error(1)
}
func (m *mockSubRepo) MarkPaymentFailed(ctx context.Context, orderID string) error {
	return m.Called(ctx, orderID).Error(0)
}
func (m *mockSubRepo) GetPaymentByOrderID(ctx context.Context, orderID string) (*models.SubscriptionPayment, error) {
	args := m.Called(ctx, orderID)
	p, _ := args.Get(0).(*models.SubscriptionPayment)
	return p, args.Error(1)
}
func (m *mockSubRepo) StartPaidPeriod(ctx context.Context, tutorID, plan, token string, pe time.Time) error {
	return m.Called(ctx, tutorID, plan, token, pe).Error(0)
}
func (m *mockSubRepo) RenewPeriod(ctx context.Context, tutorID, plan string, pe time.Time) error {
	return m.Called(ctx, tutorID, plan, pe).Error(0)
}
func (m *mockSubRepo) ListDueAutopay(ctx context.Context, now time.Time) ([]models.DueSubscription, error) {
	args := m.Called(ctx, now)
	d, _ := args.Get(0).([]models.DueSubscription)
	return d, args.Error(1)
}
func (m *mockSubRepo) Cancel(ctx context.Context, tutorID string) error {
	return m.Called(ctx, tutorID).Error(0)
}
func (m *mockSubRepo) SetPendingPlan(ctx context.Context, tutorID, plan string) error {
	return m.Called(ctx, tutorID, plan).Error(0)
}
```

- [ ] **Step 5: Скомпилировать и прогнать тесты**

Run: `go build ./...` (integration-тест исключён тегом) — без ошибок.
Run: `go test ./...` — PASS (существующие service-тесты компилируются с расширенным моком).
Run (если есть локальная БД с применённой 019): `go test -tags=integration ./repository/ -run TestClaim -v` — PASS. Без БД: `t.Skip`.

- [ ] **Step 6: Commit**

```bash
git add models/subscription.go repository/subscription.go repository/subscription_integration_test.go service/subscription_test.go
git commit -m "feat(subscriptions): payment repo methods + atomicity integration test"
```

---

### Task 4: Service — провайдер в конструкторе + Checkout (CIT)

**Files:**
- Modify: `service/subscription.go`
- Modify: `router/router.go:48` (конструктор)
- Modify: `service/subscription_test.go` (обновить конструкторы в существующих тестах)

**Interfaces:**
- Consumes: `service.PaymentProvider` (Task 2), `SubscriptionRepository.InsertPendingPayment` (Task 3).
- Produces:
  - `NewSubscriptionService(repo repository.SubscriptionRepository, provider PaymentProvider) SubscriptionService`
  - `Checkout(ctx, tutorID, plan string) (checkoutURL string, err error)` — генерит `order_id=uuid`, вставляет pending, зовёт `provider.InitPayment`.
  - хелперы `planAmount(plan string) int`, `planDays(plan string) int`.

- [ ] **Step 1: Написать тест Checkout (мок repo + provider)**

Добавить в `service/subscription_test.go`. Сначала — `fakeProvider`:

```go
type fakeProvider struct {
	initURL     string
	initErr     error
	chargeID    string
	chargeErr   error
	statusValue string // ответ CheckStatus (reconciliation)
	statusErr   error
	callback    service.Callback
	callbackErr error
	lastCharge  struct {
		orderID, token string
		amount         int
	}
}

func (f *fakeProvider) InitPayment(_ context.Context, _, _, _ string, _ int) (string, error) {
	return f.initURL, f.initErr
}
func (f *fakeProvider) Charge(_ context.Context, orderID, token string, amount int) (string, error) {
	f.lastCharge.orderID, f.lastCharge.token, f.lastCharge.amount = orderID, token, amount
	return f.chargeID, f.chargeErr
}
func (f *fakeProvider) CheckStatus(_ context.Context, _ string) (string, error) {
	return f.statusValue, f.statusErr
}
func (f *fakeProvider) ParseCallback(_ *http.Request) (service.Callback, error) {
	return f.callback, f.callbackErr
}

func TestCheckout_InsertsPendingAndReturnsURL(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{initURL: "https://pay.freedom/redirect"}
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).
		Return(true, nil)
	svc := service.NewSubscriptionService(repo, prov)

	url, err := svc.Checkout(context.Background(), "t1", "monthly")
	assert.NoError(t, err)
	assert.Equal(t, "https://pay.freedom/redirect", url)
	repo.AssertExpectations(t)
}
```

> `mockSubRepo` уже имеет stub-методы для всех новых репо-методов (добавлены в T3). Здесь только `.On("InsertPendingPayment", ...)` в самом тесте + `fakeProvider`.

Обновить существующие `service.NewSubscriptionService(repo)` в файле на `service.NewSubscriptionService(repo, &fakeProvider{})` (тесты State/GetStatus/Confirm).

- [ ] **Step 2: Запустить тест — должен упасть на компиляции (нет provider-аргумента / Checkout старый)**

Run: `go test ./service/ -run TestCheckout -v`
Expected: FAIL (compile error: too few arguments / метод не тот).

- [ ] **Step 3: Реализовать в `service/subscription.go`**

```go
import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"tutorgo/models"
	"tutorgo/repository"
)

type subscriptionService struct {
	repo     repository.SubscriptionRepository
	provider PaymentProvider
}

func NewSubscriptionService(repo repository.SubscriptionRepository, provider PaymentProvider) SubscriptionService {
	return &subscriptionService{repo: repo, provider: provider}
}

func planAmount(plan string) int {
	if plan == "yearly" {
		return PriceYearly
	}
	return PriceMonthly
}

func planDays(plan string) int {
	if plan == "yearly" {
		return 365
	}
	return 30
}

func (s *subscriptionService) Checkout(ctx context.Context, tutorID, plan string) (string, error) {
	orderID := uuid.NewString()
	amount := planAmount(plan)
	claimed, err := s.repo.InsertPendingPayment(ctx, tutorID, orderID, plan, amount)
	if err != nil {
		return "", err
	}
	if !claimed {
		return "", fmt.Errorf("duplicate order_id %s", orderID) // uuid-коллизия ~ невозможна
	}
	return s.provider.InitPayment(ctx, orderID, tutorID, plan, amount)
}
```

Убрать старую заглушку `Checkout`. `Confirm` пока НЕ трогать (удалим в Task 7). Проверить, что `github.com/google/uuid` уже в `go.mod` (используется в проекте — JWT/rooms); если нет — `go get github.com/google/uuid`.

- [ ] **Step 4: Обновить конструктор в router**

`router/router.go:48`:
```go
	subscriptionService := service.NewSubscriptionService(subscriptionRepo, service.StubProvider{})
```

- [ ] **Step 5: Запустить тесты**

Run: `go test ./service/... && go build ./...`
Expected: PASS + сборка зелёная.

- [ ] **Step 6: Commit**

```bash
git add service/subscription.go service/subscription_test.go router/router.go
git commit -m "feat(subscriptions): provider-backed Checkout (CIT), provider in constructor"
```

---

### Task 5: Service — HandleWebhook, RenewDue, Cancel, ChangePlan

**Files:**
- Modify: `service/subscription.go`
- Modify: `service/subscription_test.go`

**Interfaces:**
- Consumes: репо-методы Task 3, `PaymentProvider.Charge`/`CheckStatus`/`ParseCallback`.
- Produces (на `SubscriptionService`):
  - `HandleWebhook(ctx, r *http.Request) error` — ParseCallback → dedup → активация. Возвращает `ErrBadSignature` на плохой подписи.
  - `RenewDue(ctx) (int64, error)` — тик шедулера: по-дневный claim + Charge + продление. Возвращает число продлённых.
  - `Cancel(ctx, tutorID string) error`
  - `ChangePlan(ctx, tutorID, plan string) error`
  - `var ErrBadSignature = errors.New("bad webhook signature")`

- [ ] **Step 1: Написать тесты (мок repo + fakeProvider)**

```go
func TestHandleWebhook_Success_StartsPeriod(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{callback: service.Callback{
		OrderID: "o1", ProviderPaymentID: "pp1", Status: "success", CardToken: "tok1",
	}}
	repo.On("GetPaymentByOrderID", mock.Anything, "o1").
		Return(&models.SubscriptionPayment{TutorID: "t1", OrderID: "o1", Plan: "monthly", Amount: 10000, Status: "pending"}, nil)
	repo.On("MarkPaymentSuccess", mock.Anything, "o1", "pp1").Return(true, nil)
	repo.On("StartPaidPeriod", mock.Anything, "t1", "monthly", "tok1", mock.AnythingOfType("time.Time")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	err := svc.HandleWebhook(context.Background(), httptest.NewRequest("POST", "/subscription/webhook", nil))
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestHandleWebhook_Dedup_NoOp(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{callback: service.Callback{OrderID: "o1", ProviderPaymentID: "pp1", Status: "success"}}
	repo.On("GetPaymentByOrderID", mock.Anything, "o1").
		Return(&models.SubscriptionPayment{TutorID: "t1", Plan: "monthly", Status: "success"}, nil)
	repo.On("MarkPaymentSuccess", mock.Anything, "o1", "pp1").Return(false, nil) // уже success
	svc := service.NewSubscriptionService(repo, prov)

	err := svc.HandleWebhook(context.Background(), httptest.NewRequest("POST", "/subscription/webhook", nil))
	assert.NoError(t, err)
	repo.AssertNotCalled(t, "StartPaidPeriod", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestHandleWebhook_BadSignature(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{callbackErr: errors.New("bad sig")}
	svc := service.NewSubscriptionService(repo, prov)

	err := svc.HandleWebhook(context.Background(), httptest.NewRequest("POST", "/subscription/webhook", nil))
	assert.ErrorIs(t, err, service.ErrBadSignature)
}

func TestRenewDue_ChargesAndExtends(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{chargeID: "pp-mit"}
	pe := time.Now().Add(-time.Hour)
	repo.On("ListDueAutopay", mock.Anything, mock.AnythingOfType("time.Time")).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: pe}}, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(true, nil)
	repo.On("MarkPaymentSuccess", mock.Anything, mock.AnythingOfType("string"), "pp-mit").Return(true, nil)
	repo.On("RenewPeriod", mock.Anything, "t1", "monthly", mock.AnythingOfType("time.Time")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)
	repo.AssertExpectations(t)
}

func TestRenewDue_AppliesPendingPlan(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{chargeID: "pp-mit"}
	yearly := "yearly"
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", PendingPlan: &yearly, CardToken: "tok1", PeriodEnd: time.Now()}}, nil)
	// эффективный план — yearly: claim и charge на yearly-сумму, продление на yearly
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "yearly", service.PriceYearly).Return(true, nil)
	repo.On("MarkPaymentSuccess", mock.Anything, mock.AnythingOfType("string"), "pp-mit").Return(true, nil)
	repo.On("RenewPeriod", mock.Anything, "t1", "yearly", mock.AnythingOfType("time.Time")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)
	repo.AssertExpectations(t)
}

func TestRenewDue_ChargeFails_MarksFailedNoExtend(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{chargeErr: errors.New("declined")}
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: time.Now()}}, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(true, nil)
	repo.On("MarkPaymentFailed", mock.Anything, mock.AnythingOfType("string")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	repo.AssertNotCalled(t, "RenewPeriod", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRenewDue_ChargeErrButReconcileSuccess_Extends(t *testing.T) {
	repo := new(mockSubRepo)
	// Charge упал (таймаут), но CheckStatus говорит success → трактуем как оплату.
	prov := &fakeProvider{chargeErr: errors.New("timeout"), statusValue: "success"}
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: time.Now()}}, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(true, nil)
	repo.On("MarkPaymentSuccess", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(true, nil)
	repo.On("RenewPeriod", mock.Anything, "t1", "monthly", mock.AnythingOfType("time.Time")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)
	repo.AssertNotCalled(t, "MarkPaymentFailed", mock.Anything, mock.Anything)
}

func TestRenewDue_ChargeErrReconcileFailed_MarksFailed(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{chargeErr: errors.New("declined"), statusValue: "failed"}
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: time.Now()}}, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(true, nil)
	repo.On("MarkPaymentFailed", mock.Anything, mock.AnythingOfType("string")).Return(nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	repo.AssertNotCalled(t, "RenewPeriod", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRenewDue_NotClaimed_SkipsCharge(t *testing.T) {
	repo := new(mockSubRepo)
	prov := &fakeProvider{}
	repo.On("ListDueAutopay", mock.Anything, mock.Anything).
		Return([]models.DueSubscription{{TutorID: "t1", Plan: "monthly", CardToken: "tok1", PeriodEnd: time.Now()}}, nil)
	repo.On("InsertPendingPayment", mock.Anything, "t1", mock.AnythingOfType("string"), "monthly", service.PriceMonthly).Return(false, nil)
	svc := service.NewSubscriptionService(repo, prov)

	n, err := svc.RenewDue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	assert.Equal(t, "", prov.lastCharge.token) // Charge не вызывался
}

func TestCancel(t *testing.T) {
	repo := new(mockSubRepo)
	repo.On("Cancel", mock.Anything, "t1").Return(nil)
	svc := service.NewSubscriptionService(repo, &fakeProvider{})
	assert.NoError(t, svc.Cancel(context.Background(), "t1"))
	repo.AssertExpectations(t)
}

func TestChangePlan(t *testing.T) {
	repo := new(mockSubRepo)
	repo.On("SetPendingPlan", mock.Anything, "t1", "yearly").Return(nil)
	svc := service.NewSubscriptionService(repo, &fakeProvider{})
	assert.NoError(t, svc.ChangePlan(context.Background(), "t1", "yearly"))
	repo.AssertExpectations(t)
}
```

- [ ] **Step 2: Запустить — упадёт (методов нет)**

Run: `go test ./service/ -run 'TestHandleWebhook|TestRenewDue|TestCancel|TestChangePlan' -v`
Expected: FAIL (compile: методы не определены).

- [ ] **Step 3: Реализовать**

Добавить в `service/subscription.go` (импорты `errors`, `net/http`, `time`):

```go
var ErrBadSignature = errors.New("bad webhook signature")

func (s *subscriptionService) HandleWebhook(ctx context.Context, r *http.Request) error {
	cb, err := s.provider.ParseCallback(r)
	if err != nil {
		return ErrBadSignature
	}
	pay, err := s.repo.GetPaymentByOrderID(ctx, cb.OrderID)
	if err != nil {
		return err
	}
	if pay == nil {
		return nil // неизвестный order_id — не наш, no-op (хендлер отдаст 200 + log)
	}
	if cb.Status != "success" {
		return s.repo.MarkPaymentFailed(ctx, cb.OrderID)
	}
	activated, err := s.repo.MarkPaymentSuccess(ctx, cb.OrderID, cb.ProviderPaymentID)
	if err != nil {
		return err
	}
	if !activated {
		return nil // повторный webhook — уже активировано
	}
	periodEnd := time.Now().AddDate(0, 0, planDays(pay.Plan))
	return s.repo.StartPaidPeriod(ctx, pay.TutorID, pay.Plan, cb.CardToken, periodEnd)
}

func (s *subscriptionService) RenewDue(ctx context.Context) (int64, error) {
	due, err := s.repo.ListDueAutopay(ctx, time.Now())
	if err != nil {
		return 0, err
	}
	today := time.Now().Format("2006-01-02")
	var renewed int64
	for _, d := range due {
		plan := d.Plan
		if d.PendingPlan != nil {
			plan = *d.PendingPlan // отложенная смена тарифа применяется на этом списании
		}
		orderID := fmt.Sprintf("%s:%s:%s", d.TutorID, d.PeriodEnd.Format(time.RFC3339), today)
		claimed, err := s.repo.InsertPendingPayment(ctx, d.TutorID, orderID, plan, planAmount(plan))
		if err != nil {
			return renewed, err
		}
		if !claimed {
			continue // сегодня уже пытались (другой инстанс / повтор тика)
		}
		ppid, err := s.provider.Charge(ctx, orderID, d.CardToken, planAmount(plan))
		if err != nil {
			// Reconciliation: g2g не дедупит, а Charge мог реально пройти (таймаут).
			// Спрашиваем статус; MarkFailed только при подтверждённом не-успехе.
			if st, sErr := s.provider.CheckStatus(ctx, orderID); sErr == nil && st == "success" {
				ppid = orderID // платёж прошёл; provider_payment_id недоступен — пишем orderID
			} else {
				_ = s.repo.MarkPaymentFailed(ctx, orderID) // остаётся в grace, ретрай завтра
				continue
			}
		}
		activated, err := s.repo.MarkPaymentSuccess(ctx, orderID, ppid)
		if err != nil {
			return renewed, err
		}
		if !activated {
			continue
		}
		newEnd := d.PeriodEnd.AddDate(0, 0, planDays(plan)) // аддитивно от старого period_end
		if err := s.repo.RenewPeriod(ctx, d.TutorID, plan, newEnd); err != nil {
			return renewed, err
		}
		renewed++
	}
	return renewed, nil
}

func (s *subscriptionService) Cancel(ctx context.Context, tutorID string) error {
	return s.repo.Cancel(ctx, tutorID)
}

func (s *subscriptionService) ChangePlan(ctx context.Context, tutorID, plan string) error {
	return s.repo.SetPendingPlan(ctx, tutorID, plan)
}
```

Дописать методы в `SubscriptionService` interface: `HandleWebhook`, `RenewDue`, `Cancel`, `ChangePlan`.

- [ ] **Step 4: Запустить тесты**

Run: `go test ./service/... -v`
Expected: PASS (все новые + старые).

- [ ] **Step 5: Commit**

```bash
git add service/subscription.go service/subscription_test.go
git commit -m "feat(subscriptions): webhook activation, MIT renew scheduler, cancel, change-plan"
```

---

### Task 6: Handlers + routes — webhook (public), cancel, change-plan

**Files:**
- Modify: `handlers/subscription.go`
- Modify: `router/router.go` (routes)
- Test: `handlers/subscription_test.go` (создать — тестов у хендлера подписки сейчас нет)

**Interfaces:**
- Consumes: `SubscriptionService.HandleWebhook/Cancel/ChangePlan` (Task 5), `service.ErrBadSignature`.
- Produces: `Webhook(c)`, `Cancel(c)`, `ChangePlan(c)` на `*SubscriptionHandler`; локальный `mockSubscriptionService` в `handlers/subscription_test.go`.

- [ ] **Step 1: Создать `handlers/subscription_test.go` с моком сервиса и тестами**

`mockSubscriptionService` реализует ВЕСЬ интерфейс `service.SubscriptionService` (на этот момент он ещё содержит `Confirm` — уберётся в Task 7). Мок живёт в этом же test-файле (паттерн codebase — `mocks_test.go` не трогаем):

```go
type mockSubscriptionService struct{ mock.Mock }

func (m *mockSubscriptionService) GetStatus(ctx context.Context, tutorID string) (models.SubscriptionStatus, error) {
	args := m.Called(ctx, tutorID)
	st, _ := args.Get(0).(models.SubscriptionStatus)
	return st, args.Error(1)
}
func (m *mockSubscriptionService) State(ctx context.Context, tutorID string) (string, error) {
	args := m.Called(ctx, tutorID)
	return args.String(0), args.Error(1)
}
func (m *mockSubscriptionService) Checkout(ctx context.Context, tutorID, plan string) (string, error) {
	args := m.Called(ctx, tutorID, plan)
	return args.String(0), args.Error(1)
}
func (m *mockSubscriptionService) Confirm(ctx context.Context, tutorID, plan string) error {
	return m.Called(ctx, tutorID, plan).Error(0)
}
func (m *mockSubscriptionService) HandleWebhook(ctx context.Context, r *http.Request) error {
	return m.Called(ctx, r).Error(0)
}
func (m *mockSubscriptionService) RenewDue(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockSubscriptionService) Cancel(ctx context.Context, tutorID string) error {
	return m.Called(ctx, tutorID).Error(0)
}
func (m *mockSubscriptionService) ChangePlan(ctx context.Context, tutorID, plan string) error {
	return m.Called(ctx, tutorID, plan).Error(0)
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
```

Тесты webhook-маппинга кодов:

```go
func TestWebhook_BadSignature_401(t *testing.T) {
	svc := new(mockSubscriptionService)
	svc.On("HandleWebhook", mock.Anything, mock.Anything).Return(service.ErrBadSignature)
	h := handlers.NewSubscriptionHandler(svc, testLogger())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/subscription/webhook", nil)
	h.Webhook(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWebhook_OK_200(t *testing.T) {
	svc := new(mockSubscriptionService)
	svc.On("HandleWebhook", mock.Anything, mock.Anything).Return(nil)
	h := handlers.NewSubscriptionHandler(svc, testLogger())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/subscription/webhook", nil)
	h.Webhook(c)
	assert.Equal(t, http.StatusOK, w.Code)
}
```

Импорты файла: `context`, `io`, `log/slog`, `net/http`, `net/http/httptest`, `testing`, `tutorgo/models`, `tutorgo/service`, gin, testify `assert`/`mock`.

- [ ] **Step 2: Запустить — упадёт (методов нет)**

Run: `go test ./handlers/ -run TestWebhook -v`
Expected: FAIL (compile).

- [ ] **Step 3: Реализовать хендлеры**

Добавить в `handlers/subscription.go` (импорт `errors`):

```go
// POST /subscription/webhook — публичный, без JWT (шлёт провайдер).
func (h *SubscriptionHandler) Webhook(c *gin.Context) {
	err := h.svc.HandleWebhook(c.Request.Context(), c.Request)
	switch {
	case err == nil:
		c.Status(http.StatusOK)
	case errors.Is(err, service.ErrBadSignature):
		h.log.Warn("rejected subscription webhook: bad signature")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid webhook"})
	default:
		h.log.Error("subscription webhook", slog.String("error", err.Error()))
		c.Status(http.StatusInternalServerError) // провайдер передоставит, dedup спасёт
	}
}

func (h *SubscriptionHandler) Cancel(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.svc.Cancel(c.Request.Context(), tutorID); err != nil {
		h.log.Error("cancel subscription", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SubscriptionHandler) ChangePlan(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.SubscriptionPlanRequest
	if !bindAndValidate(c, &req) {
		return
	}
	if err := h.svc.ChangePlan(c.Request.Context(), tutorID, req.Plan); err != nil {
		h.log.Error("change plan", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 4: Зарегистрировать роуты**

В `router/router.go`: webhook — на top-level `r` рядом с `/webhooks/livekit`:
```go
	r.POST("/subscription/webhook", subscriptionHandler.Webhook)
```
В группе `open` (после `checkout`):
```go
		open.POST("/subscription/cancel", subscriptionHandler.Cancel)
		open.POST("/subscription/change-plan", subscriptionHandler.ChangePlan)
```

- [ ] **Step 5: Запустить тесты + сборку**

Run: `go test ./handlers/... && go build ./...`
Expected: PASS + сборка зелёная (`Confirm` ещё жив, ничего не сломано).

- [ ] **Step 6: Commit**

```bash
git add handlers/subscription.go handlers/subscription_test.go router/router.go
git commit -m "feat(subscriptions): webhook/cancel/change-plan handlers + routes"
```

---

### Task 7: Удалить `Confirm`-заглушку (закрыть self-grant дыру)

**Files:**
- Modify: `service/subscription.go` (убрать `Confirm` из interface + impl)
- Modify: `handlers/subscription.go` (убрать `Confirm`)
- Modify: `handlers/subscription_test.go` (убрать `Confirm` из `mockSubscriptionService`, созданного в Task 6)
- Modify: `router/router.go` (убрать роут `/subscription/confirm`)
- Modify: `service/subscription_test.go` (удалить `TestSubscriptionService_Confirm_*`)

**Interfaces:**
- Removes: `SubscriptionService.Confirm`, `SubscriptionHandler.Confirm`, роут `POST /subscription/confirm`.

- [ ] **Step 1: Удалить метод из сервиса**

В `service/subscription.go` убрать `Confirm` из interface `SubscriptionService` и удалить метод `func (s *subscriptionService) Confirm(...)`.

- [ ] **Step 2: Удалить хендлер и роут**

В `handlers/subscription.go` удалить `func (h *SubscriptionHandler) Confirm(...)`.
В `router/router.go` удалить строку `open.POST("/subscription/confirm", subscriptionHandler.Confirm)`.

- [ ] **Step 3: Удалить Confirm из моков и тестов**

В `handlers/subscription_test.go` убрать метод `Confirm` из `mockSubscriptionService`.
В `service/subscription_test.go` удалить `TestSubscriptionService_Confirm_ActivatesMonthly` и `TestSubscriptionService_Confirm_ActivatesYearly`.

- [ ] **Step 4: Сборка + тесты**

Run: `go build ./... && go test ./...`
Expected: PASS, ноль ссылок на `Confirm` (проверить: `grep -rn "Confirm" service/ handlers/ router/` → пусто).

- [ ] **Step 5: Commit**

```bash
git add service/subscription.go handlers/subscription.go handlers/mocks_test.go router/router.go service/subscription_test.go
git commit -m "refactor(subscriptions): remove Confirm stub (closes free self-grant)"
```

---

### Task 8: Шедулер автопродления — проводка в main.go

**Files:**
- Modify: `main.go` (обобщить loop-функцию, запустить renew-loop)
- Modify: `router/router.go` (`Setup` возвращает сервис подписок)
- Test: `main_test.go` (создать — минимальный тест loop-функции)

**Interfaces:**
- Consumes: `SubscriptionService.RenewDue` (Task 5).
- Produces: `router.Setup(pool, log, cfg) (*gin.Engine, service.SubscriptionService)`; `runIntervalLoop(ctx, interval, name, job, log)`.

- [ ] **Step 1: `Setup` возвращает сервис подписок**

В `router/router.go` изменить сигнатуру:
```go
func Setup(pool *pgxpool.Pool, log *slog.Logger, cfg *config.Config) (*gin.Engine, service.SubscriptionService) {
```
В конце функции (где сейчас `return r`) → `return r, subscriptionService`.

- [ ] **Step 2: Обобщить loop и запустить renew**

В `main.go` заменить `runAutoCompleteLoop` на обобщённый:
```go
func runIntervalLoop(ctx context.Context, interval time.Duration, name string, job func(context.Context) (int64, error), log *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			count, err := job(ctx)
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Error(name+" failed", slog.String("error", err.Error()))
			} else if count > 0 {
				log.Info(name+" done", slog.Int64("count", count))
			}
		case <-ctx.Done():
			return
		}
	}
}
```
В `main()`:
```go
	r, subscriptionService := router.Setup(pool, log, &cfg)
	...
	bgWg.Go(func() {
		runIntervalLoop(bgCtx, 1*time.Minute, "auto-complete lessons", lessonRepo.AutoComplete, log)
	})
	bgWg.Go(func() {
		runIntervalLoop(bgCtx, 24*time.Hour, "subscription renew", subscriptionService.RenewDue, log)
	})
```

- [ ] **Step 3: Тест loop-функции (без сети/БД)**

`main_test.go`:
```go
package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunIntervalLoop_CallsJobThenStopsOnCancel(t *testing.T) {
	var calls atomic.Int64
	job := func(context.Context) (int64, error) {
		calls.Add(1)
		return 1, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	done := make(chan struct{})
	go func() {
		runIntervalLoop(ctx, 5*time.Millisecond, "test", job, log)
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("loop не завершился после cancel")
	}
	assert.GreaterOrEqual(t, calls.Load(), int64(1))
	_ = errors.Is // держим импорт, если понадобится
}
```

- [ ] **Step 4: Сборка + тесты**

Run: `go build ./... && go test ./...`
Expected: PASS. Проверить, что нет других вызовов `runAutoCompleteLoop` (`grep -rn runAutoCompleteLoop .` → пусто).

- [ ] **Step 5: Commit**

```bash
git add main.go main_test.go router/router.go
git commit -m "feat(subscriptions): daily MIT renew scheduler wired into main"
```

---

## Carry-forward в freedompay-план (из финального ревью 2026-07-08, ДО реальных денег)

1. **Important — ограничить dunning grace-окном.** `ListDueAutopay` не имеет нижней
   границы по `period_end` → ретраи идут вечно после grace. Сценарий: карта ожила на
   30-й день → списание за период, полностью прожитый в blocked, и назавтра ещё одно.
   Фикс: нижняя граница `period_end > now() - (GraceDays+1) days` в запросе, либо
   `autopay=false` при выходе из grace.
2. Лог/метрика на ambiguous/застрявшие pending-строки (default-ветки RenewDue) — для ops.
3. `ParseCallback` обязан маппить в `Callback.Status` ТОЛЬКО терминальные статусы
   провайдера: не-терминальный «processing» в текущей семантике закроет строку как
   failed навсегда (service/subscription.go: `cb.Status != "success"` → MarkPaymentFailed).
4. Minor: первый тик шедулера через 24ч после старта — при частых рестартах продления
   голодают; один прогон при старте цикла. Minor: мёртвый `Activate` в репо-интерфейсе
   (обходит оплату) — удалить. Minor: переименовать `TestRenewDue_ChargeFails_MarksFailedNoExtend`
   (реально тестирует ambiguous-путь).

## Follow-up (отдельные планы, вне этого)

1. **freedompay-адаптер** — `payment/freedompay` реализует `service.PaymentProvider`: `InitPayment` (`POST /init_payment`, hosted CIT, `pg_idempotency_key`), `Charge` (`POST /g2g/payment` + `pg_card_token`, синхронный XML), `CheckStatus` (`status_v2` для reconcile), `ParseCallback` (проверка `pg_sig` = MD5(script_name + поля по алфавиту c `pg_salt` + `secret_key`); подтвердить `script_name` для callback-URL). Свапнуть `service.StubProvider{}` в `router.go:48` на реальный конструктор с `merchant_id`/секретом из `config`. Гейт: аппрув Freedom Pay (активация `pg_card_token` менеджером).
2. **Frontend** — убрать клиентский `confirm`-POST; success/fail-страницы с поллингом `GET /subscription`; кнопки cancel/change-plan. **Деплоить РАНЬШЕ этого backend** (Task 7 убирает `/subscription/confirm`).
3. **change-card** — tokenize-only флоу (verification-сумма), перезапись `card_token`. Зависит от способа токенизации без списания у Freedom Pay.

## Self-Review

- **Spec coverage:** миграция 019 (T1) ✓; PaymentProvider интерфейс (T2) ✓; идемпотентность pending-INSERT→conditional-UPDATE (T3 репо + integration-тест) ✓; CIT checkout (T4) ✓; webhook активация + dedup + маппинг кодов 401/200/5xx (T5+T6) ✓; MIT по-дневный claim + аддитивное продление + pending_plan (T5) ✓; cancel (T5+T6) ✓; change-plan (T5+T6) ✓; удаление confirm-заглушки (T7) ✓; шедулер в main (T8) ✓; атомарность честно помечена как покрытая только integration-тестом (T3) ✓. Отложено намеренно: freedompay-адаптер, frontend, change-card (follow-up).
- **Атомарность:** money-инвариант «ровно один claim» проверяется реальным-Postgres тестом (T3, build tag `integration`), а не моками — оговорка спеки соблюдена.
- **Type consistency:** `InsertPendingPayment`/`MarkPaymentSuccess` возвращают `(bool, error)` во всех задачах; `RenewDue` — `(int64, error)` (согласовано с `runIntervalLoop`); `order_id` MIT = `tutorID:period_end(RFC3339):today` во всех местах.
