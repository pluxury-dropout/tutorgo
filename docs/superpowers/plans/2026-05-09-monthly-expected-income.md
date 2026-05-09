# Monthly Expected Income Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить эндпоинт `GET /payments/monthly-expected` и отобразить ожидаемый доход на страницах Dashboard и Payments.

**Architecture:** Новый метод `GetMonthlyExpected` добавляется по всей цепочке repo→service→handler. Frontend получает два значения (фактический + ожидаемый доход) и отображает их в одном KPI-сегменте.

**Tech Stack:** Go, pgxpool, Gin, React/Next.js, TanStack Query

---

## File Map

| Файл | Что меняем |
|---|---|
| `repository/payment.go` | + метод `GetMonthlyExpected` в интерфейс и реализацию |
| `service/payment.go` | + метод `GetMonthlyExpected` в интерфейс и реализацию |
| `service/payment_test.go` | + mock-метод + 2 теста |
| `handlers/payment.go` | + метод `GetMonthlyExpected` |
| `handlers/mocks_test.go` | + mock-метод `GetMonthlyExpected` |
| `handlers/payment_test.go` | + маршрут в роутер + 3 теста |
| `router/router.go` | + маршрут `GET /payments/monthly-expected` |
| `frontend/src/lib/api/payments.ts` | + функция `monthlyExpected` |
| `frontend/src/lib/hooks/usePayments.ts` | + ключ + хук `useMonthlyExpected` |
| `frontend/src/app/(dashboard)/dashboard/page.tsx` | обновить сегмент «Доход» |
| `frontend/src/app/(dashboard)/payments/page.tsx` | заполнить сегмент «Ожидается» |

---

### Task 1: Repository — интерфейс + реализация

**Files:**
- Modify: `repository/payment.go`

- [ ] **Step 1: Добавить метод в интерфейс `PaymentRepository`**

В `repository/payment.go`, в блок `type PaymentRepository interface`, добавить строку после `GetMonthlyIncome`:

```go
GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error)
```

- [ ] **Step 2: Реализовать метод**

В конец `repository/payment.go` добавить:

```go
func (r *paymentRepository) GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error) {
	var total float64
	err := r.conn.QueryRow(ctx,
		`SELECT COALESCE(SUM(c.price_per_lesson * lc.cnt), 0)
		 FROM courses c
		 JOIN (
		     SELECT course_id, COUNT(*) AS cnt
		     FROM lessons
		     WHERE status IN ('scheduled', 'completed', 'missed')
		       AND scheduled_at >= date_trunc('month', NOW())
		       AND scheduled_at <  date_trunc('month', NOW()) + interval '1 month'
		     GROUP BY course_id
		 ) lc ON lc.course_id = c.id
		 WHERE c.tutor_id = $1`,
		tutorID,
	).Scan(&total)
	return total, err
}
```

- [ ] **Step 3: Убедиться, что проект компилируется**

```
go build ./...
```

Ожидание: нет ошибок (интерфейс и реализация совпадают).

- [ ] **Step 4: Commit**

```
git add repository/payment.go
git commit -m "feat(repo): add GetMonthlyExpected to PaymentRepository"
```

---

### Task 2: Service — TDD

**Files:**
- Modify: `service/payment.go`
- Modify: `service/payment_test.go`

- [ ] **Step 1: Добавить mock-метод в `mockPaymentRepo`**

В `service/payment_test.go`, после метода `GetMonthlyIncome` у `mockPaymentRepo`, добавить:

```go
func (m *mockPaymentRepo) GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).(float64), args.Error(1)
}
```

- [ ] **Step 2: Написать тесты**

В конец `service/payment_test.go` добавить:

```go
// GetMonthlyExpected

func TestPaymentGetMonthlyExpected_Success(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo)

	payRepo.On("GetMonthlyExpected", mock.Anything, tutorID).Return(75000.0, nil)

	result, err := svc.GetMonthlyExpected(context.Background(), tutorID)

	assert.NoError(t, err)
	assert.Equal(t, 75000.0, result)
	payRepo.AssertExpectations(t)
}

func TestPaymentGetMonthlyExpected_RepoError(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo)

	payRepo.On("GetMonthlyExpected", mock.Anything, tutorID).Return(0.0, errors.New("db error"))

	result, err := svc.GetMonthlyExpected(context.Background(), tutorID)

	assert.Error(t, err)
	assert.Equal(t, 0.0, result)
	payRepo.AssertExpectations(t)
}
```

- [ ] **Step 3: Запустить тесты — убедиться, что падают**

```
go test ./service/ -run TestPaymentGetMonthlyExpected -v
```

Ожидание: FAIL — `GetMonthlyExpected undefined`.

- [ ] **Step 4: Добавить метод в интерфейс и реализацию `service/payment.go`**

В `type PaymentService interface` добавить строку после `GetMonthlyIncome`:

```go
GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error)
```

В конец файла добавить реализацию:

```go
func (s *paymentService) GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error) {
	return s.repo.GetMonthlyExpected(ctx, tutorID)
}
```

- [ ] **Step 5: Запустить тесты — убедиться, что проходят**

```
go test ./service/ -run TestPaymentGetMonthlyExpected -v
```

Ожидание: PASS оба теста.

- [ ] **Step 6: Прогнать все тесты сервиса**

```
go test ./service/ -v
```

Ожидание: все PASS.

- [ ] **Step 7: Commit**

```
git add service/payment.go service/payment_test.go
git commit -m "feat(service): add GetMonthlyExpected to PaymentService"
```

---

### Task 3: Handler + Router — TDD

**Files:**
- Modify: `handlers/payment.go`
- Modify: `handlers/mocks_test.go`
- Modify: `handlers/payment_test.go`
- Modify: `router/router.go`

- [ ] **Step 1: Добавить mock-метод в `mockPaymentService`**

В `handlers/mocks_test.go`, после метода `GetMonthlyIncome` у `mockPaymentService`, добавить:

```go
func (m *mockPaymentService) GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).(float64), args.Error(1)
}
```

- [ ] **Step 2: Обновить роутер в тестах**

В `handlers/payment_test.go`, в функции `newPaymentRouter`, добавить маршрут:

```go
func newPaymentRouter(svc *mockPaymentService, tutorID string) *gin.Engine {
	r := gin.New()
	h := handlers.NewPaymentHandler(svc, slog.Default())
	r.Use(withTutorID(tutorID))
	r.GET("/payments", h.GetAll)
	r.POST("/payments", h.Create)
	r.GET("/payments/balance", h.GetBalance)
	r.GET("/payments/monthly-expected", h.GetMonthlyExpected)
	return r
}
```

- [ ] **Step 3: Написать тесты для хендлера**

В конец `handlers/payment_test.go` добавить:

```go
// GetMonthlyExpected

func TestPaymentGetMonthlyExpected_Success(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	svc.On("GetMonthlyExpected", mock.Anything, testTutorID).Return(75000.0, nil)

	w := makeRequest(t, r, http.MethodGet, "/payments/monthly-expected", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got map[string]float64
	decodeJSON(t, w, &got)
	assert.Equal(t, 75000.0, got["total"])
	svc.AssertExpectations(t)
}

func TestPaymentGetMonthlyExpected_Unauthorized(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, "")

	w := makeRequest(t, r, http.MethodGet, "/payments/monthly-expected", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "GetMonthlyExpected")
}

func TestPaymentGetMonthlyExpected_ServiceError(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	svc.On("GetMonthlyExpected", mock.Anything, testTutorID).Return(0.0, errors.New("db error"))

	w := makeRequest(t, r, http.MethodGet, "/payments/monthly-expected", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}
```

- [ ] **Step 4: Запустить тесты — убедиться, что падают**

```
go test ./handlers/ -run TestPaymentGetMonthlyExpected -v
```

Ожидание: FAIL — `GetMonthlyExpected undefined` (хендлер ещё не написан).

- [ ] **Step 5: Добавить хендлер в `handlers/payment.go`**

В конец `handlers/payment.go` добавить:

```go
func (h *PaymentHandler) GetMonthlyExpected(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	total, err := h.service.GetMonthlyExpected(c.Request.Context(), tutorID)
	if err != nil {
		h.log.Error("Failed to get monthly expected", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total})
}
```

- [ ] **Step 6: Зарегистрировать маршрут в `router/router.go`**

В `router/router.go`, после строки с `monthly-income`, добавить:

```go
auth.GET("/payments/monthly-expected", paymentHandler.GetMonthlyExpected)
```

- [ ] **Step 7: Запустить тесты — убедиться, что проходят**

```
go test ./handlers/ -run TestPaymentGetMonthlyExpected -v
```

Ожидание: PASS все 3 теста.

- [ ] **Step 8: Прогнать все тесты**

```
go test ./...
```

Ожидание: все PASS.

- [ ] **Step 9: Commit**

```
git add handlers/payment.go handlers/mocks_test.go handlers/payment_test.go router/router.go
git commit -m "feat(handler): add GET /payments/monthly-expected endpoint"
```

---

### Task 4: Frontend — API + Hook

**Files:**
- Modify: `frontend/src/lib/api/payments.ts`
- Modify: `frontend/src/lib/hooks/usePayments.ts`

- [ ] **Step 1: Добавить функцию в `payments.ts`**

В `frontend/src/lib/api/payments.ts`, в объект `paymentsApi`, после `monthlyIncome`, добавить:

```ts
monthlyExpected: () =>
  api.get<{ total: number }>('/payments/monthly-expected').then((r) => r.data.total),
```

- [ ] **Step 2: Добавить ключ и хук в `usePayments.ts`**

В `frontend/src/lib/hooks/usePayments.ts`, в объект `paymentKeys`, после `monthlyIncome`, добавить:

```ts
monthlyExpected: ['payments', 'monthly-expected'] as const,
```

После функции `useMonthlyIncome` добавить:

```ts
export function useMonthlyExpected() {
  return useQuery({
    queryKey: paymentKeys.monthlyExpected,
    queryFn:  paymentsApi.monthlyExpected,
  })
}
```

- [ ] **Step 3: Проверить компиляцию TypeScript**

```
cd frontend && npx tsc --noEmit
```

Ожидание: нет ошибок.

- [ ] **Step 4: Commit**

```
git add frontend/src/lib/api/payments.ts frontend/src/lib/hooks/usePayments.ts
git commit -m "feat(frontend): add monthlyExpected API function and useMonthlyExpected hook"
```

---

### Task 5: Dashboard — обновить сегмент «Доход»

**Files:**
- Modify: `frontend/src/app/(dashboard)/dashboard/page.tsx`

- [ ] **Step 1: Добавить хук**

В `frontend/src/app/(dashboard)/dashboard/page.tsx`:

1. В список импортов добавить `useMonthlyExpected`:

```ts
import { useRecentPayments, useMonthlyIncome, useMonthlyExpected } from '@/lib/hooks/usePayments'
```

2. В теле компонента `DashboardPage`, после строки с `useMonthlyIncome`, добавить:

```ts
const { data: monthlyExpected = 0, isLoading: expectedLoading } = useMonthlyExpected()
```

- [ ] **Step 2: Обновить сегмент «Доход»**

Найти в массиве `segments` объект с `id: 'revenue'` и заменить его целиком:

```ts
{
  id:       'revenue',
  label:    'Доход',
  value:    formatAmount(monthlyExpected),
  dotColor: 'var(--warning)',
  meta:     'получено ' + formatAmount(monthlyIncome),
  loading:  expectedLoading || incomeLoading,
},
```

- [ ] **Step 3: Запустить dev-сервер и проверить вручную**

```
cd frontend && npm run dev
```

Открыть `http://localhost:3000/dashboard`. Сегмент «Доход»:
- Крупное число — ожидаемый доход (₸ X)
- Мелкий текст — `получено ₸ Y`

- [ ] **Step 4: Commit**

```
git add frontend/src/app/(dashboard)/dashboard/page.tsx
git commit -m "feat(dashboard): show expected income as main value in revenue segment"
```

---

### Task 6: Payments — заполнить сегмент «Ожидается»

**Files:**
- Modify: `frontend/src/app/(dashboard)/payments/page.tsx`

- [ ] **Step 1: Добавить хук**

В `frontend/src/app/(dashboard)/payments/page.tsx`:

1. В список импортов добавить `useMonthlyExpected`:

```ts
import { usePaymentsPaged, useMonthlyIncome, useMonthlyExpected } from '@/lib/hooks/usePayments'
```

2. В теле компонента `PaymentsPageInner`, после строки с `useMonthlyIncome`, добавить:

```ts
const { data: monthlyExpected = 0, isLoading: expectedLoading } = useMonthlyExpected()
```

- [ ] **Step 2: Обновить сегмент «Ожидается»**

Найти в массиве `segments` объект с `id: 'pending'` и заменить его целиком:

```ts
{
  id:       'pending',
  label:    'Ожидается',
  value:    '₸ ' + monthlyExpected.toLocaleString('ru-RU'),
  dotColor: 'var(--purple)',
  meta:     'этот месяц',
  loading:  expectedLoading,
},
```

- [ ] **Step 3: Проверить вручную**

Открыть `http://localhost:3000/payments`. Сегмент «Ожидается»:
- Показывает ожидаемый доход (числовое значение вместо `—`)
- `meta` = `этот месяц`

- [ ] **Step 4: Commit**

```
git add frontend/src/app/(dashboard)/payments/page.tsx
git commit -m "feat(payments): fill expected income segment"
```
