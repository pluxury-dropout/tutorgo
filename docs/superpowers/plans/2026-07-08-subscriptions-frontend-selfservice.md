# Frontend подписок v1b: hosted-checkout + self-service — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Перевести фронт подписок на hosted-checkout (redirect → webhook-активация → success-поллинг) и добавить self-service (cancel, change-plan), убрав вызов удалённого `/subscription/confirm`.

**Architecture:** Backend отдаёт `autopay`/`pending_plan` в статусе, чтобы UI корректно показывал управление. Фронт: `onPay` редиректит на `checkout_url` провайдера; новая `/subscription/success` поллит `GET /subscription` до `active`; страница подписки получает блок cancel/change-plan. Cancel/change-plan хендлеры и роуты на бэке уже существуют — трогаем только status-ответ.

**Tech Stack:** Go (Gin, pgx), Next.js 16 / React (client components), axios, тесты фронта на `node:test` (Node 24 TS-strip).

## Global Constraints

- Порядок выката: **frontend → backend** (backend Task 7 удалил `/subscription/confirm`; задеплоенный раньше фронта backend 404-ит клиента). Этот план — фронт + минимальный backend-довесок status; деплой всё равно фронт первым.
- Активация подписки — только через webhook (сервер-сайд). Клиент НЕ подтверждает оплату (закрытая self-grant дыра).
- `GET /subscription` пути: статус `{state, plan, period_end, prices, autopay, pending_plan}`; `POST /subscription/checkout` → `{checkout_url}`; `POST /subscription/cancel` → 204; `POST /subscription/change-plan` → 204.
- Тарифы ровно два: `monthly`, `yearly`. «Сменить тариф» = переключить на другой.
- Реального `checkout_url` пока нет (`StubProvider` → `/checkout` отдаёт 5xx). «Оплатить» падает тостом — это ожидаемо до freedompay-адаптера.
- Frontend-тесты: `node --test` по `*.test.ts`, импорт с расширением `.ts` (`allowImportingTsExtensions`). Тестируем чистые хелперы, не React-компоненты.

---

### Task 1: Backend — вывести `autopay` и `pending_plan` в статусе подписки

**Files:**
- Modify: `models/subscription.go` (struct `Subscription`, `SubscriptionStatus`)
- Modify: `repository/subscription.go` (`GetByTutor` SELECT)
- Modify: `service/subscription.go` (`GetStatus` populate)
- Test: `service/subscription_test.go`

**Interfaces:**
- Produces: `SubscriptionStatus` JSON с полями `autopay bool` (`json:"autopay"`), `pending_plan *string` (`json:"pending_plan"`) — потребляет фронт (Task 2).

- [ ] **Step 1: Расширить модели**

В `models/subscription.go` добавить поля в `Subscription`:

```go
// Subscription — подписка репетитора на SaaS (1:1 с tutor).
type Subscription struct {
	TutorID       string     `json:"-"`
	Plan          *string    `json:"plan"`       // nil = пробный период
	PeriodEnd     *time.Time `json:"period_end"` // nil только для grandfathered
	Grandfathered bool       `json:"-"`
	Autopay       bool       `json:"-"`
	PendingPlan   *string    `json:"-"`
}
```

И в `SubscriptionStatus`:

```go
type SubscriptionStatus struct {
	State       string     `json:"state"`
	Plan        *string    `json:"plan"`
	PeriodEnd   *time.Time `json:"period_end"`
	Prices      Prices     `json:"prices"`
	Autopay     bool       `json:"autopay"`
	PendingPlan *string    `json:"pending_plan"`
}
```

- [ ] **Step 2: Обновить `GetByTutor` SELECT**

В `repository/subscription.go`, метод `GetByTutor` — добавить 2 колонки в SELECT и Scan (колонки из migration 019: `autopay BOOLEAN NOT NULL`, `pending_plan TEXT`):

```go
err := r.conn.QueryRow(ctx,
	`SELECT tutor_id, plan, period_end, grandfathered, autopay, pending_plan
	 FROM subscriptions WHERE tutor_id = $1`,
	tutorID,
).Scan(&s.TutorID, &s.Plan, &s.PeriodEnd, &s.Grandfathered, &s.Autopay, &s.PendingPlan)
```

- [ ] **Step 3: Populate в `GetStatus`**

В `service/subscription.go`, метод `GetStatus` — прокинуть новые поля внутри `if sub != nil`:

```go
if sub != nil {
	st.Plan = sub.Plan
	st.PeriodEnd = sub.PeriodEnd
	st.Autopay = sub.Autopay
	st.PendingPlan = sub.PendingPlan
}
```

- [ ] **Step 4: Написать тест на новые поля**

В `service/subscription_test.go` добавить тест (рядом с `TestSubscriptionService_GetStatus_IncludesPrices`):

```go
func TestSubscriptionService_GetStatus_ExposesAutopayAndPendingPlan(t *testing.T) {
	repo := new(mockSubRepo)
	pending := "yearly"
	repo.On("GetByTutor", mock.Anything, "t1").Return(&models.Subscription{
		Autopay:     true,
		PendingPlan: &pending,
	}, nil)
	svc := service.NewSubscriptionService(repo, service.StubProvider{})

	st, err := svc.GetStatus(context.Background(), "t1")

	assert.NoError(t, err)
	assert.True(t, st.Autopay)
	assert.Equal(t, &pending, st.PendingPlan)
}
```

- [ ] **Step 5: Запустить тесты — убедиться, что проходят**

Run: `go test ./service/ ./models/ -run GetStatus`
Expected: PASS (все GetStatus-тесты зелёные)

Затем полная сборка: `go build ./... && go vet ./...` — без ошибок (Scan новых полей компилируется).

- [ ] **Step 6: Commit**

```bash
git add models/subscription.go repository/subscription.go service/subscription.go service/subscription_test.go
git commit -m "feat(subscriptions): expose autopay + pending_plan in status"
```

---

### Task 2: Frontend — переписать api-клиент подписок

**Files:**
- Modify: `frontend/src/lib/api/subscription.ts`

**Interfaces:**
- Consumes: статус с полями `autopay`, `pending_plan` (Task 1).
- Produces: `subscriptionApi.checkout(plan) → Promise<{checkout_url: string}>`, `subscriptionApi.cancel() → Promise<void>`, `subscriptionApi.changePlan(plan) → Promise<void>`, `subscriptionApi.get() → Promise<Subscription>`; тип `Subscription` с `autopay: boolean`, `pending_plan: Plan | null`. Потребляют Task 3, Task 4.

- [ ] **Step 1: Переписать файл целиком**

Заменить содержимое `frontend/src/lib/api/subscription.ts`:

```ts
// frontend/src/lib/api/subscription.ts
import { api } from './client'

export type SubState = 'active' | 'grace' | 'blocked'
export type Plan = 'monthly' | 'yearly'

export interface Subscription {
  state: SubState
  plan: Plan | null
  period_end: string | null
  prices: { monthly: number; yearly: number; currency: string }
  autopay: boolean
  pending_plan: Plan | null
}

export const subscriptionApi = {
  get: () => api.get<Subscription>('/subscription').then((r) => r.data),
  // checkout → hosted-страница провайдера; редирект делает вызывающий.
  checkout: (plan: Plan) =>
    api.post<{ checkout_url: string }>('/subscription/checkout', { plan }).then((r) => r.data),
  cancel: () => api.post('/subscription/cancel').then(() => undefined),
  changePlan: (plan: Plan) =>
    api.post('/subscription/change-plan', { plan }).then(() => undefined),
}
```

- [ ] **Step 2: Проверить, что нет других потребителей `confirm`**

Run: `grep -rn "\.confirm(" frontend/src --include=*.ts --include=*.tsx`
Expected: пусто (единственный вызов был в `subscription/page.tsx`, его чиним в Task 4). Если что-то ещё есть — учесть в Task 4.

- [ ] **Step 3: Проверить сборку типов**

Run: `cd frontend && npx tsc --noEmit`
Expected: ошибки ТОЛЬКО в `subscription/page.tsx` (использует удалённый `confirm` и старый тип `checkout`) — их закрывает Task 4. Других ошибок нет.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api/subscription.ts
git commit -m "feat(subscriptions): api client — checkout_url, cancel, change-plan"
```

---

### Task 3: Frontend — success-страница с поллингом

**Files:**
- Create: `frontend/src/lib/subscriptionPoll.ts` (чистый хелпер решения)
- Create: `frontend/src/lib/subscriptionPoll.test.ts`
- Create: `frontend/src/app/(dashboard)/subscription/success/page.tsx`

**Interfaces:**
- Consumes: `SubState` (Task 2), `subscriptionApi.get` (Task 2).
- Produces: `pollDecision(state, attemptsLeft) → 'activated' | 'timeout' | 'wait'`.

- [ ] **Step 1: Написать падающий тест хелпера**

Создать `frontend/src/lib/subscriptionPoll.test.ts`:

```ts
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { pollDecision } from './subscriptionPoll.ts'

test('active → activated (даже при исчерпанных попытках)', () => {
  assert.equal(pollDecision('active', 5), 'activated')
  assert.equal(pollDecision('active', 0), 'activated')
})

test('не-active с оставшимися попытками → wait', () => {
  assert.equal(pollDecision('grace', 3), 'wait')
  assert.equal(pollDecision('blocked', 1), 'wait')
})

test('не-active без попыток → timeout', () => {
  assert.equal(pollDecision('grace', 0), 'timeout')
  assert.equal(pollDecision('blocked', 0), 'timeout')
})
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `cd frontend && node --test src/lib/subscriptionPoll.test.ts`
Expected: FAIL (модуль `subscriptionPoll.ts` не существует)

- [ ] **Step 3: Реализовать хелпер**

Создать `frontend/src/lib/subscriptionPoll.ts`:

```ts
import type { SubState } from './api/subscription.ts'

export type PollResult = 'activated' | 'timeout' | 'wait'

// active выигрывает всегда; иначе ждём, пока есть попытки, потом таймаут.
export function pollDecision(state: SubState, attemptsLeft: number): PollResult {
  if (state === 'active') return 'activated'
  if (attemptsLeft <= 0) return 'timeout'
  return 'wait'
}
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `cd frontend && node --test src/lib/subscriptionPoll.test.ts`
Expected: PASS (3 теста)

- [ ] **Step 5: Написать success-страницу**

Создать `frontend/src/app/(dashboard)/subscription/success/page.tsx`:

```tsx
// frontend/src/app/(dashboard)/subscription/success/page.tsx
'use client'

import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { subscriptionApi } from '@/lib/api/subscription'
import { pollDecision } from '@/lib/subscriptionPoll'
import { PageHeader } from '@/components/common/PageHeader'
import { Button } from '@/components/ui/button'

const MAX_ATTEMPTS = 15
const INTERVAL_MS = 2000

export default function SubscriptionSuccessPage() {
  const [timedOut, setTimedOut] = useState(false)
  const started = useRef(false)

  useEffect(() => {
    if (started.current) return // StrictMode dev-double-mount guard
    started.current = true

    let attempts = 0
    let timer: ReturnType<typeof setTimeout>

    async function poll() {
      attempts++
      const attemptsLeft = MAX_ATTEMPTS - attempts
      try {
        const sub = await subscriptionApi.get()
        const decision = pollDecision(sub.state, attemptsLeft)
        if (decision === 'activated') {
          toast.success('Подписка активирована')
          // hard-nav: layout группы (dashboard) держит устаревший subState → нужен
          // ремоунт, иначе guard отбросит на paywall. См. subscription/page.tsx.
          window.location.href = '/dashboard'
          return
        }
        if (decision === 'timeout') {
          setTimedOut(true)
          return
        }
      } catch {
        // сеть/5xx — не срываем поллинг, пробуем ещё, пока есть попытки
        if (attemptsLeft <= 0) {
          setTimedOut(true)
          return
        }
      }
      timer = setTimeout(poll, INTERVAL_MS)
    }

    poll()
    return () => clearTimeout(timer)
  }, [])

  return (
    <>
      <PageHeader title="Оплата" />
      <div className="mt-6 max-w-md space-y-4">
        {timedOut ? (
          <>
            <p className="text-sm text-muted-foreground">
              Оплата обрабатывается. Это может занять пару минут — обновите страницу позже.
            </p>
            <Button onClick={() => (window.location.href = '/dashboard')}>В дашборд</Button>
          </>
        ) : (
          <p className="text-sm text-muted-foreground">Проверяем оплату…</p>
        )}
      </div>
    </>
  )
}
```

- [ ] **Step 6: Проверить сборку типов**

Run: `cd frontend && npx tsc --noEmit`
Expected: ошибки только в `subscription/page.tsx` (Task 4). Новые файлы чисты.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/subscriptionPoll.ts frontend/src/lib/subscriptionPoll.test.ts "frontend/src/app/(dashboard)/subscription/success/page.tsx"
git commit -m "feat(subscriptions): success page with status polling"
```

---

### Task 4: Frontend — redirect-checkout + блок управления на странице подписки

**Files:**
- Modify: `frontend/src/app/(dashboard)/subscription/page.tsx`

**Interfaces:**
- Consumes: `subscriptionApi.checkout/cancel/changePlan/get`, тип `Subscription` с `autopay`/`pending_plan` (Task 2).

- [ ] **Step 1: Переписать `onPay` на redirect**

В `frontend/src/app/(dashboard)/subscription/page.tsx` заменить `onPay` — убрать `confirm()` и блок активации, редиректить на `checkout_url`:

```tsx
  async function onPay(plan: Plan) {
    setPaying(plan)
    try {
      const { checkout_url } = await subscriptionApi.checkout(plan)
      window.location.href = checkout_url // hosted-страница провайдера
    } catch {
      toast.error('Не удалось начать оплату, попробуйте ещё раз')
      setPaying(null) // при успехе не сбрасываем — уходим со страницы
    }
  }
```

- [ ] **Step 2: Показать тост при возврате с failure-URL**

Добавить чтение query `?status=failed` (провайдер редиректит сюда при отказе). В начало компонента, рядом с существующим `useEffect(() => { load() }, [])`:

```tsx
  useEffect(() => {
    if (new URLSearchParams(window.location.search).get('status') === 'failed') {
      toast.error('Оплата не прошла, попробуйте ещё раз')
      window.history.replaceState(null, '', '/subscription') // убрать query из URL
    }
  }, [])
```

- [ ] **Step 3: Добавить хендлеры cancel/change-plan**

Внутри компонента `SubscriptionPage`, рядом с `onPay`:

```tsx
  const [busy, setBusy] = useState(false)

  async function onCancel() {
    if (!confirm('Отменить подписку? Доступ сохранится до конца оплаченного периода, затем аккаунт будет заблокирован.')) return
    setBusy(true)
    try {
      await subscriptionApi.cancel()
      await load()
      toast.success('Автопродление отключено')
    } catch {
      toast.error('Не удалось отменить подписку')
    } finally {
      setBusy(false)
    }
  }

  async function onChangePlan(next: Plan) {
    setBusy(true)
    try {
      await subscriptionApi.changePlan(next)
      await load()
      toast.success('Тариф сменится со следующего периода')
    } catch {
      toast.error('Не удалось сменить тариф')
    } finally {
      setBusy(false)
    }
  }
```

- [ ] **Step 4: Отрисовать блок управления и условие тарифных карточек**

В JSX: тарифные карточки (сетка «Помесячно/На год») показывать когда `sub.plan === null || sub.state !== 'active'`. Ниже статуса добавить блок управления когда `sub.plan !== null`:

```tsx
        {sub.plan !== null && (
          <div className="border rounded-lg p-5 space-y-3">
            <h2 className="text-sm font-semibold">Управление</h2>
            {sub.pending_plan ? (
              <p className="text-sm text-muted-foreground">
                Со следующего периода: {sub.pending_plan === 'yearly' ? 'на год' : 'помесячно'}
              </p>
            ) : (
              <Button
                variant="outline"
                disabled={busy}
                onClick={() => onChangePlan(sub.plan === 'monthly' ? 'yearly' : 'monthly')}
              >
                Перейти на {sub.plan === 'monthly' ? 'годовой' : 'месячный'} тариф
              </Button>
            )}
            {sub.autopay && (
              <Button variant="ghost" className="text-destructive" disabled={busy} onClick={onCancel}>
                Отменить подписку
              </Button>
            )}
          </div>
        )}
```

Обернуть существующую сетку тарифов в `{(sub.plan === null || sub.state !== 'active') && ( ... )}`.

- [ ] **Step 5: Проверить сборку и типы**

Run: `cd frontend && npx tsc --noEmit`
Expected: PASS (0 ошибок — `confirm`-вызова больше нет, типы совпадают).

Run: `cd frontend && npm run build`
Expected: build успешен.

- [ ] **Step 6: Commit**

```bash
git add "frontend/src/app/(dashboard)/subscription/page.tsx"
git commit -m "feat(subscriptions): redirect checkout + cancel/change-plan controls"
```

---

## Self-Review

**Spec coverage:**
- Backend-довесок (autopay/pending_plan) → Task 1 ✓
- api-клиент (checkout_url, убрать confirm, cancel, changePlan, тип) → Task 2 ✓
- success-страница с поллингом → Task 3 ✓
- onPay redirect + failure-query + cancel/change-plan UI → Task 4 ✓
- Порядок выката (frontend→backend) → Global Constraints ✓
- Осознанные пропуски (freedompay-адаптер, change-card, pg_*_url) → в spec, вне плана ✓

**Placeholder scan:** нет TBD/TODO; весь код показан целиком.

**Type consistency:** `pollDecision(state, attemptsLeft)` одинаково в Task 3 определении, тесте и странице; `subscriptionApi.checkout` возвращает `{checkout_url}` в Task 2 и потребляется в Task 4; `Subscription.autopay/pending_plan` определены в Task 2, читаются в Task 3/4; backend `SubscriptionStatus` json-теги (`autopay`, `pending_plan`) в Task 1 совпадают с фронт-типом в Task 2.
