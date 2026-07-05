# Subscriptions Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Дать заблокированному по подписке репетитору видимый paywall-UI вместо молчаливых 402: guard в layout, страница тарифа, grace-баннер, 402-интерсептор.

**Architecture:** Единый источник состояния — client-guard в `(dashboard)/layout.tsx` (зеркалит серверный `RequireActiveSubscription`): на маунте дёргает `GET /subscription`, по чистой функции `decideAccess(state, path)` решает redirect / render / render+banner. Страницы состояние не читают. Интерсептор 402 в axios — сеть безопасности от гонок. Fail-closed: ошибка fetch трактуется как `blocked`.

**Tech Stack:** Next.js 16 (App Router, `'use client'`), TypeScript, axios (`lib/api/client.ts`), `@tanstack/react-query` (в проекте, но guard держит состояние через `useState`/`useEffect` — так проще и локальнее), `sonner` toast, `lucide-react` иконки, `StatusBadge` из дизайн-системы. Тесты: `node --test --experimental-strip-types` (Node 24, native TS-strip).

## Global Constraints

- Backend уже готов и совпадает по форме: `GET /subscription` (открыт) → `{ state, plan, period_end, prices:{ monthly, yearly, currency } }`; `POST /subscription/checkout` и `POST /subscription/confirm` (открыты, заглушки) принимают тело `{ plan }`. Значения: `state ∈ {active,grace,blocked}`, `plan ∈ {monthly,yearly}|null`, цены `monthly=5000`, `yearly=48000`, `currency="KZT"`.
- Все новые запросы к подписке идут через существующий `api` из `@/lib/api/client` (несёт JWT-интерсептор). Не создавать второй axios-инстанс.
- Путь paywall-страницы строго `/subscription` — это же строковое значение сравнивается в guard и в интерсепторе. Не хардкодить его в разных местах по-разному: сравнение идёт по `usePathname()` (без trailing slash).
- Тесты пишутся как `*.test.ts` рядом с исходником, запускаются `node --test --experimental-strip-types <file>`. Импорты — с расширением `.ts` (native strip требует `allowImportingTsExtensions`, уже настроено).
- Язык UI — русский (проект русскоязычный).
- `ponytail:` — без Zustand/стора для подписки, без персиста dismiss баннера (сессии достаточно), без реального редиректа на провайдера.

---

### Task 1: API-клиент подписки

**Files:**
- Create: `frontend/src/lib/api/subscription.ts`

**Interfaces:**
- Consumes: `api` из `./client`.
- Produces:
  - `type SubState = 'active' | 'grace' | 'blocked'`
  - `type Plan = 'monthly' | 'yearly'`
  - `interface Subscription { state: SubState; plan: Plan | null; period_end: string | null; prices: { monthly: number; yearly: number; currency: string } }`
  - `subscriptionApi.get(): Promise<Subscription>`
  - `subscriptionApi.checkout(plan: Plan): Promise<void>`
  - `subscriptionApi.confirm(plan: Plan): Promise<void>`

- [ ] **Step 1: Создать файл API-клиента**

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
}

export const subscriptionApi = {
  get: () => api.get<Subscription>('/subscription').then((r) => r.data),
  // checkout возвращает { checkout_url } — заглушку отбрасываем (провайдера ещё нет).
  checkout: (plan: Plan) =>
    api.post('/subscription/checkout', { plan }).then(() => undefined),
  confirm: (plan: Plan) =>
    api.post('/subscription/confirm', { plan }).then(() => undefined),
}
```

- [ ] **Step 2: Проверить компиляцию типов**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок в `subscription.ts` (могут быть прежние — важно, что новых нет).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api/subscription.ts
git commit -m "feat(subscriptions): add subscription API client"
```

---

### Task 2: Чистая функция решения guard + тест (TDD)

Ядро guard вынесено из компонента в тестируемую чистую функцию. Ошибка fetch трактуется вызывающим кодом как `blocked` (fail-closed) ещё до вызова, поэтому функция принимает только три состояния.

**Files:**
- Create: `frontend/src/lib/subscriptionGuard.ts`
- Test: `frontend/src/lib/subscriptionGuard.test.ts`

**Interfaces:**
- Consumes: `SubState` из `./api/subscription`.
- Produces:
  - `const PAYWALL_PATH = '/subscription'`
  - `type GuardDecision = { action: 'redirect' } | { action: 'render'; banner: boolean }`
  - `decideAccess(state: SubState, path: string): GuardDecision`

- [ ] **Step 1: Написать падающий тест**

```ts
// frontend/src/lib/subscriptionGuard.test.ts
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { decideAccess, PAYWALL_PATH } from './subscriptionGuard.ts'

test('active → render без баннера на любом пути', () => {
  assert.deepEqual(decideAccess('active', '/dashboard'), { action: 'render', banner: false })
  assert.deepEqual(decideAccess('active', PAYWALL_PATH), { action: 'render', banner: false })
})

test('grace → render с баннером на любом пути', () => {
  assert.deepEqual(decideAccess('grace', '/students'), { action: 'render', banner: true })
  assert.deepEqual(decideAccess('grace', PAYWALL_PATH), { action: 'render', banner: true })
})

test('blocked на обычном пути → redirect', () => {
  assert.deepEqual(decideAccess('blocked', '/dashboard'), { action: 'redirect' })
})

test('blocked на самом paywall → render без цикла', () => {
  assert.deepEqual(decideAccess('blocked', PAYWALL_PATH), { action: 'render', banner: false })
})
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `cd frontend && node --test --experimental-strip-types src/lib/subscriptionGuard.test.ts`
Expected: FAIL — `Cannot find module './subscriptionGuard.ts'`.

- [ ] **Step 3: Реализовать чистую функцию**

```ts
// frontend/src/lib/subscriptionGuard.ts
import type { SubState } from './api/subscription'

export const PAYWALL_PATH = '/subscription'

export type GuardDecision =
  | { action: 'redirect' }
  | { action: 'render'; banner: boolean }

// Зеркалит серверный RequireActiveSubscription. blocked вне paywall → на paywall;
// на самом paywall всегда render (иначе цикл). grace → пускаем, но с баннером.
export function decideAccess(state: SubState, path: string): GuardDecision {
  if (state === 'blocked' && path !== PAYWALL_PATH) return { action: 'redirect' }
  return { action: 'render', banner: state === 'grace' }
}
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `cd frontend && node --test --experimental-strip-types src/lib/subscriptionGuard.test.ts`
Expected: PASS — `pass 4`, `fail 0`.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/subscriptionGuard.ts frontend/src/lib/subscriptionGuard.test.ts
git commit -m "feat(subscriptions): add decideAccess guard function with tests"
```

---

### Task 3: Страница тарифа `/subscription`

Внутри `(dashboard)`-группы (с сайдбаром). Дёргает только открытый `GET /subscription` → рендерится и при `blocked`. Badge состояния + две карточки тарифов + `onPay`.

**Files:**
- Create: `frontend/src/app/(dashboard)/subscription/page.tsx`

**Interfaces:**
- Consumes: `subscriptionApi`, `Subscription`, `SubState`, `Plan` из `@/lib/api/subscription`; `Button` из `@/components/ui/button`; `PageHeader` из `@/components/common/PageHeader`; `toast` из `sonner`. (Активацию завершаем жёсткой навигацией `window.location.href`, поэтому `useRouter` не нужен.)
- Produces: default-export React-компонент страницы (Next.js route `/subscription`).

**Замечание про StatusBadge:** он принимает `status` из фиксированного union (`active|ended|LessonStatus`), НЕ совпадающего с `active|grace|blocked`. Поэтому для состояния подписки используем локальный inline-badge (не переиспользуем StatusBadge, чтобы не расширять его union ради трёх значений). `ponytail:` inline-badge вместо правки общего компонента.

- [ ] **Step 1: Создать страницу**

```tsx
// frontend/src/app/(dashboard)/subscription/page.tsx
'use client'

import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { subscriptionApi, Subscription, SubState, Plan } from '@/lib/api/subscription'
import { PageHeader } from '@/components/common/PageHeader'
import { Button } from '@/components/ui/button'

const STATE_META: Record<SubState, { label: string; cls: string }> = {
  active:  { label: 'Активна',  cls: 'bg-[var(--status-completed-bg)] text-[var(--status-completed-text)]' },
  grace:   { label: 'Истекла — оплатите', cls: 'bg-[var(--status-scheduled-bg)] text-[var(--status-scheduled-text)]' },
  blocked: { label: 'Заблокирована', cls: 'bg-[var(--status-cancelled-bg)] text-[var(--status-cancelled-text)]' },
}

function formatPrice(amount: number, currency: string): string {
  return new Intl.NumberFormat('ru-RU', {
    style: 'currency',
    currency,
    maximumFractionDigits: 0,
  }).format(amount)
}

export default function SubscriptionPage() {
  const [sub, setSub] = useState<Subscription | null>(null)
  const [paying, setPaying] = useState<Plan | null>(null)

  async function load() {
    try {
      setSub(await subscriptionApi.get())
    } catch {
      toast.error('Не удалось загрузить статус подписки')
    }
  }

  useEffect(() => { load() }, [])

  async function onPay(plan: Plan) {
    setPaying(plan)
    try {
      await subscriptionApi.checkout(plan)
      await subscriptionApi.confirm(plan)
      const fresh = await subscriptionApi.get()
      setSub(fresh)
      if (fresh.state === 'active') {
        toast.success('Подписка активирована')
        // ВАЖНО: жёсткая навигация, а не router.replace. Layout группы (dashboard)
        // остаётся смонтированным при клиентском переходе и держит устаревший
        // subState='blocked' → guard отбросил бы обратно на paywall. Hard-nav
        // перемонтирует layout и перечитывает статус. Побочно: кейс «пришёл из
        // профиля → остаться» схлопывается в редирект на dashboard — ок для MVP.
        window.location.href = '/dashboard'
      }
    } catch {
      toast.error('Оплата не прошла, попробуйте ещё раз')
    } finally {
      setPaying(null)
    }
  }

  if (!sub) {
    return (
      <>
        <PageHeader title="Подписка" />
        <div className="mt-6 text-sm text-muted-foreground">Загрузка…</div>
      </>
    )
  }

  const meta = STATE_META[sub.state]
  const { monthly, yearly, currency } = sub.prices
  const discount = Math.round((1 - yearly / (monthly * 12)) * 100)

  return (
    <>
      <PageHeader title="Подписка" />

      <div className="mt-6 max-w-2xl space-y-6">
        <div className="flex items-center gap-3">
          <span className="text-sm text-muted-foreground">Статус:</span>
          <span className={`inline-flex items-center rounded-[20px] px-[9px] py-[3px] text-xs font-semibold ${meta.cls}`}>
            {meta.label}
          </span>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          {/* Месяц */}
          <div className="border rounded-lg p-5 space-y-3">
            <h2 className="text-sm font-semibold">Помесячно</h2>
            <p className="text-2xl font-bold">{formatPrice(monthly, currency)}<span className="text-sm font-normal text-muted-foreground"> / мес</span></p>
            <Button className="w-full" disabled={paying !== null} onClick={() => onPay('monthly')}>
              {paying === 'monthly' ? 'Оплата…' : 'Оплатить'}
            </Button>
          </div>

          {/* Год */}
          <div className="border rounded-lg p-5 space-y-3 relative">
            {discount > 0 && (
              <span className="absolute top-3 right-3 rounded-[20px] bg-primary/10 text-primary px-2 py-px text-[11px] font-semibold">
                −{discount}%
              </span>
            )}
            <h2 className="text-sm font-semibold">На год</h2>
            <p className="text-2xl font-bold">{formatPrice(yearly, currency)}<span className="text-sm font-normal text-muted-foreground"> / год</span></p>
            <Button className="w-full" disabled={paying !== null} onClick={() => onPay('yearly')}>
              {paying === 'yearly' ? 'Оплата…' : 'Оплатить'}
            </Button>
          </div>
        </div>
      </div>
    </>
  )
}
```

- [ ] **Step 2: Проверить css-переменные статусов**

Run: `cd frontend && grep -n "status-scheduled-bg\|status-completed-bg\|status-cancelled-bg" src/app/globals.css`
Expected: все три переменные существуют. Если `--status-scheduled-bg` отсутствует — заменить `grace`-класс на существующую жёлтую/warning-переменную из вывода grep (например `--status-scheduled-*` может называться иначе). НЕ выдумывать имя: взять реальное из globals.css.

- [ ] **Step 3: Проверить компиляцию**

Run: `cd frontend && npx tsc --noEmit`
Expected: без новых ошибок.

- [ ] **Step 4: Commit**

```bash
git add "frontend/src/app/(dashboard)/subscription/page.tsx"
git commit -m "feat(subscriptions): add subscription paywall page"
```

---

### Task 4: Guard + grace-баннер в layout

Подвесить проверку подписки в `(dashboard)/layout.tsx` ПОСЛЕ существующего auth-guard: пока грузится — спиннер (children не рендерим); по `decideAccess` — redirect или render; при `grace` — toast-баннер снизу-слева с крестиком (dismiss на сессию). Ошибка fetch → `blocked` (fail-closed).

**Files:**
- Modify: `frontend/src/app/(dashboard)/layout.tsx`

**Interfaces:**
- Consumes: `subscriptionApi`, `SubState` из `@/lib/api/subscription`; `decideAccess`, `PAYWALL_PATH` из `@/lib/subscriptionGuard`; `X` из `lucide-react`; `Link` из `next/link`.
- Produces: тот же default-export layout, но с sub-guard.

- [ ] **Step 1: Добавить состояние подписки и fetch (fail-closed)**

В `frontend/src/app/(dashboard)/layout.tsx` заменить блок импортов и верх компонента. Текущие импорты (строки 3-8):

```tsx
import { useEffect, useState } from 'react'
import { useRouter, usePathname } from 'next/navigation'
import { Menu, GraduationCap } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { Sidebar } from '@/components/layout/Sidebar'
import { MobileBottomNav } from '@/components/layout/MobileBottomNav'
```

заменить на:

```tsx
import { useEffect, useState } from 'react'
import { useRouter, usePathname } from 'next/navigation'
import Link from 'next/link'
import { Menu, GraduationCap, X } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { Sidebar } from '@/components/layout/Sidebar'
import { MobileBottomNav } from '@/components/layout/MobileBottomNav'
import { subscriptionApi, SubState } from '@/lib/api/subscription'
import { decideAccess, PAYWALL_PATH } from '@/lib/subscriptionGuard'
```

- [ ] **Step 2: Добавить sub-state и эффект загрузки после auth-редиректа**

В теле `DashboardLayout`, сразу после существующего блока (строки 15-24, где `mounted`/`sidebarOpen` и auth-effect и `if (!mounted || !isAuthenticated) return null`) — но ДО `return null` нельзя добавлять хуки. Поэтому вставить новые хуки рядом с прочими `useState`/`useEffect` (сверху, до раннего `return`).

Добавить после `const [sidebarOpen, setSidebarOpen] = useState(false)`:

```tsx
  const [subState, setSubState] = useState<SubState | null>(null)
  const [bannerDismissed, setBannerDismissed] = useState(false)
```

Добавить новый эффект после auth-эффекта:

```tsx
  // Guard подписки: грузим только когда аутентифицированы. Ошибка → blocked (fail-closed).
  useEffect(() => {
    if (!mounted || !isAuthenticated) return
    let alive = true
    subscriptionApi.get()
      .then((s) => { if (alive) setSubState(s.state) })
      .catch(() => { if (alive) setSubState('blocked') })
    return () => { alive = false }
  }, [mounted, isAuthenticated])

  // Редирект на paywall — в эффекте, а не в теле рендера (иначе "Cannot update
  // Router while rendering"). Тот же паттерн, что у auth-редиректа выше.
  useEffect(() => {
    if (subState && decideAccess(subState, pathname).action === 'redirect') {
      router.replace(PAYWALL_PATH)
    }
  }, [subState, pathname, router])
```

- [ ] **Step 3: Добавить решение guard перед основным return**

Заменить строку раннего возврата (строка 24):

```tsx
  if (!mounted || !isAuthenticated) return null
```

на:

```tsx
  if (!mounted || !isAuthenticated) return null

  // Ждём статус подписки — не мигаем содержимым.
  if (subState === null) {
    return (
      <div className="flex h-[100dvh] items-center justify-center bg-background">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-muted border-t-primary" />
      </div>
    )
  }

  // Навигацию выполняет эффект выше; здесь только не рендерим children,
  // пока идёт редирект (иначе мелькнёт защищённая страница).
  const decision = decideAccess(subState, pathname)
  if (decision.action === 'redirect') return null
```

- [ ] **Step 4: Добавить grace-баннер в JSX**

Внутри корневого `<div className="flex flex-col overflow-hidden" ...>`, перед закрывающим `</div>` (после `<MobileBottomNav />`, строка 60), вставить:

```tsx
      {decision.banner && !bannerDismissed && (
        <div className="fixed bottom-4 left-4 z-50 max-w-xs rounded-lg border bg-card shadow-lg px-4 py-3 text-sm animate-in slide-in-from-bottom-2">
          <button
            onClick={() => setBannerDismissed(true)}
            className="absolute top-2 right-2 text-muted-foreground hover:text-foreground"
            aria-label="Закрыть"
          >
            <X className="h-4 w-4" />
          </button>
          <p className="pr-4 font-medium">Тариф истёк</p>
          <p className="pr-4 mt-0.5 text-muted-foreground">
            Оплатите — доступ скоро закроется.{' '}
            <Link href={PAYWALL_PATH} className="text-primary underline underline-offset-2">
              Перейти к оплате
            </Link>
          </p>
        </div>
      )}
```

- [ ] **Step 5: Проверить компиляцию**

Run: `cd frontend && npx tsc --noEmit`
Expected: без новых ошибок.

- [ ] **Step 6: Прогнать guard-тест (регрессия чистой функции)**

Run: `cd frontend && node --test --experimental-strip-types src/lib/subscriptionGuard.test.ts`
Expected: PASS — `pass 4`.

- [ ] **Step 7: Commit**

```bash
git add "frontend/src/app/(dashboard)/layout.tsx"
git commit -m "feat(subscriptions): guard dashboard layout + grace banner"
```

---

### Task 5: Интерсептор 402 (страховка от гонки)

В существующем `api.interceptors.response.use` добавить редирект при 402 — на случай, если состояние протухло между guard-проверкой и запросом.

**Files:**
- Modify: `frontend/src/lib/api/client.ts`

**Interfaces:**
- Consumes: существующий `error: AxiosError` в response-интерсепторе.
- Produces: побочный эффект `window.location.href = '/subscription'` при 402.

- [ ] **Step 1: Добавить обработку 402 рядом с 401**

В `frontend/src/lib/api/client.ts`, в теле response-интерсептора (после блока `if (error.response?.status === 401 ...)`, ДО построения `normalized`), вставить:

```tsx
    // 402 = подписка протухла между guard-проверкой и запросом. Страховка:
    // уводим на paywall. Не мешает 401/refresh (другой статус, ранний выход выше).
    if (error.response?.status === 402 && typeof window !== 'undefined') {
      if (window.location.pathname !== '/subscription') {
        window.location.href = '/subscription'
      }
    }
```

- [ ] **Step 2: Проверить компиляцию**

Run: `cd frontend && npx tsc --noEmit`
Expected: без новых ошибок.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api/client.ts
git commit -m "feat(subscriptions): redirect to paywall on 402 responses"
```

---

### Task 6: Ссылка «Подписка» в профиле

Добровольный вход на страницу тарифа из профиля препода.

**Files:**
- Modify: `frontend/src/app/(dashboard)/profile/page.tsx`

**Interfaces:**
- Consumes: `Link` из `next/link`.
- Produces: ссылка `/subscription` в разметке профиля.

- [ ] **Step 1: Добавить импорт Link**

В начало `frontend/src/app/(dashboard)/profile/page.tsx`, в блок импортов, добавить:

```tsx
import Link from 'next/link'
```

- [ ] **Step 2: Добавить секцию «Подписка»**

Внутри `<div className="mt-6 max-w-lg space-y-6">`, после блока `{/* Password form */}` (перед закрывающим `</div>` контейнера, т.е. в самом низу секций), вставить:

```tsx
        {/* Подписка */}
        <div className="border rounded-lg p-5 flex items-center justify-between">
          <div>
            <h2 className="text-sm font-semibold">Подписка</h2>
            <p className="text-xs text-muted-foreground mt-0.5">Тариф и оплата доступа</p>
          </div>
          <Link href="/subscription" className="text-sm text-primary underline underline-offset-2">
            Управлять
          </Link>
        </div>
```

- [ ] **Step 3: Проверить компиляцию**

Run: `cd frontend && npx tsc --noEmit`
Expected: без новых ошибок.

- [ ] **Step 4: Commit**

```bash
git add "frontend/src/app/(dashboard)/profile/page.tsx"
git commit -m "feat(subscriptions): link to subscription page from profile"
```

---

## Финальная проверка (руками в браузере)

После всех задач — сквозной прогон (spec §Тестирование, UI-часть). Backend enforcement для этого можно временно включить локально или подменить `state` в БД.

- [ ] `blocked` → любой путь редиректит на `/subscription`, сам `/subscription` рендерится без цикла.
- [ ] `grace` → приложение работает + баннер снизу-слева, крестик скрывает его до перезагрузки.
- [ ] `active` → приложение работает, баннера нет.
- [ ] «Оплата»: `blocked` → кнопка «Оплатить» → мгновенная активация → редирект в dashboard.
- [ ] Вход из профиля → `/subscription` открывается с сайдбаром.
- [ ] 402 от gated-запроса (сымитировать) → редирект на `/subscription`.

## Порядок выката (важно!)

Из backend-спеки: `middleware.RequireActiveSubscription` **нельзя включать в проде, пока этот фронтенд не задеплоен** — иначе grandfathered-преподы упрутся в молчаливые 402. Deploy-порядок: фронт → потом enforcement.

## Self-Review (сверка со спекой)

- §1 API-клиент → Task 1. ✔
- §2 Страница `/subscription` (badge, 2 карточки, onPay, редирект) → Task 3. ✔
- §2 Точка входа из профиля → Task 6. ✔
- §3 Guard (loading/blocked-redirect/blocked-self-render/active/grace/fail-closed) → Task 2 (чистая функция + тест) + Task 4 (интеграция, fail-closed через `.catch(blocked)`). ✔
- §4 Grace-баннер (bottom-left, крестик, dismiss на сессию) → Task 4. ✔
- §5 Интерсептор 402 → Task 5. ✔
- §Тест: юнит чистой функции (node:test) → Task 2; руками в браузере → финальная проверка. ✔
- §YAGNI: без стора, без персиста dismiss, без реального провайдера, без countdown — соблюдено. ✔
