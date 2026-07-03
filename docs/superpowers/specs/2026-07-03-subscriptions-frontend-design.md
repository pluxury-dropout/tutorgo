# Фронтенд подписок (billing UI для репетиторов)

**Дата:** 2026-07-03
**Статус:** дизайн утверждён, готов к плану
**Backend-спека:** `docs/superpowers/specs/2026-07-02-subscriptions-design.md` (фаза 1 смержена, миграция применена)

## Проблема и цель

Backend подписок готов: `GET /subscription` (открыт), `POST /subscription/checkout`
и `POST /subscription/confirm` (открыты, заглушки), а все бизнес-ручки под
`middleware.RequireActiveSubscription` отдают `402 {"error":"subscription_required"}`
при `blocked`. Фронта нет — заблокированный репетитор упрётся в молчаливые 402 по
всему приложению. Нужен UI: paywall-страница, guard, grace-баннер, обработка 402.

Enforcement нельзя выкатывать без этого UI.

## Утверждённые решения

| Вопрос | Решение |
|---|---|
| Enforcement при `blocked` | Guard в `(dashboard)/layout.tsx`: редирект на `/subscription`, кроме самого этого пути; интерсептор 402 как страховка |
| Расположение страницы тарифа | Внутри `(dashboard)` (с сайдбаром), доступна из профиля препода; guard исключает свой путь |
| Поток оплаты (пока нет провайдера) | Кнопка: `checkout()` → `confirm()` мгновенно → refetch → разблок. При CloudPayments меняется одна `onPay` |
| Grace-баннер | Только на `state === 'grace'`, снизу слева (toast-стиль), закрывается крестиком |
| «Триал истекает через N дней» (active) | **НЕ делаем** — вторая итерация (как в backend-спеке) |

## Архитектура

Единый источник состояния на входе — **guard в layout** (как серверный
`RequireActiveSubscription` — одна точка на всю группу). Страницы состояние не
проверяют. Интерсептор 402 — только сеть безопасности от гонок.

### 1. API-клиент — `lib/api/subscription.ts`

Три функции под готовые ручки (паттерн: файл на домен, как остальные `lib/api/*`):

```ts
type SubState = 'active' | 'grace' | 'blocked'
type Plan = 'monthly' | 'yearly'
interface Subscription {
  state: SubState
  plan: Plan | null
  period_end: string | null
  prices: { monthly: number; yearly: number; currency: string }
}

getSubscription(): Promise<Subscription>   // GET /subscription
checkout(plan: Plan): Promise<void>        // POST /subscription/checkout — fake-URL отбрасываем
confirm(plan: Plan): Promise<void>         // POST /subscription/confirm — активация (заглушка-вебхук)
```

### 2. Страница `/subscription`

**Внутри `(dashboard)`-группы** (`(dashboard)/subscription`) — с сайдбаром, как
обычный экран. Доступна двумя путями: ссылкой из профиля препода (добровольный
просмотр/смена тарифа) и принудительным редиректом guard при `blocked`. Чтобы не
было цикла, guard **исключает свой путь** (см. §3). Страница дёргает только
открытый `GET /subscription`, поэтому рендерится и при `blocked`.

Содержит: текущее состояние (badge active/grace/blocked), две карточки тарифов
(месяц / год-со-скидкой, цены и валюта из `prices`), кнопку «Оплатить» на каждой.

`onPay(plan)`: `checkout(plan)` → `confirm(plan)` → refetch `getSubscription()` →
при `active` редирект в dashboard (или toast «оплачено» + обновление badge, если
пришли из профиля). Ошибки — тост/inline. Дизраблить кнопку на время запроса.

**Точка входа из профиля:** на странице профиля препода — секция/ссылка
«Подписка» → `/subscription`.

### 3. Guard — `(dashboard)/layout.tsx`

Client-компонент. На маунте `getSubscription()`:
- загрузка → спиннер, children НЕ рендерим;
- `blocked` **и путь ≠ `/subscription`** → `router.replace('/subscription')`;
- `blocked` **на `/subscription`** → рендер children (paywall-экран сам себя
  показывает, без редиректа — иначе цикл);
- `active` / `grace` → рендер children;
- **ошибка запроса → не пускаем** (fail-closed: неизвестное состояние = блок,
  как инвариант backend «нет строки = blocked»), кроме пути `/subscription`.

### 4. Grace-баннер

В том же layout, при `state === 'grace'`: toast-стиль **снизу слева** (fixed
bottom-left), «Тариф истёк, оплатите — доступ скоро закроется» + ссылка на
`/subscription` + крестик. Закрытие крестиком скрывает баннер **на текущую
сессию** (локальный state компонента; на перезагрузке появится снова, пока
состояние `grace`). `ponytail:` без персиста dismiss — сессии достаточно.

### 5. Интерсептор 402 — `lib/api/client.ts`

В существующем `api.interceptors.response.use` (рядом с обработкой 401): при
`status === 402` → `window.location.href = '/subscription'`. Страховка от гонки,
когда состояние протухло между guard-проверкой и запросом. Не трогает 401/refresh.

## Поток данных

```
dashboard layout (mount) ──GET /subscription──> state
   ├─ blocked & path≠/subscription → redirect /subscription (paywall)
   ├─ blocked & path=/subscription → render (self-paywall, без цикла)
   ├─ grace   → bottom-left toast-banner + children
   └─ active  → children

профиль ──ссылка «Подписка»──> /subscription (добровольный просмотр, с сайдбаром)

любой gated-запрос ──402──> interceptor ──> redirect /subscription   (страховка)

/subscription: onPay ──checkout+confirm──> refetch ──active──> back to dashboard
```

## Обработка ошибок

- Guard fail-closed при ошибке `getSubscription()` — не раздаём доступ вслепую.
- `onPay` ошибки — inline/тост, кнопка снова активна, состояние не меняется.
- 402-интерсептор не конфликтует с 401-refresh (разные статусы, ранний выход).

## Тестирование

Frontend без vitest/jest (тесты на `node:test`). Поэтому:
- **Юнит** (node:test): маппинг `(state, path)` → решение guard
  (`blocked`+обычный путь→redirect, `blocked`+`/subscription`→render,
  `grace`→banner+children, `active`→children, ошибка→не пускаем) как чистая
  функция, вынесенная из компонента.
- **Руками в браузере** (UI): сквозной сценарий trial → grace → «оплатил» → active;
  402-редирект; вход из профиля; отсутствие цикла на `/subscription`; крестик
  grace-баннера.

## YAGNI (осознанно НЕ делаем)

- Countdown «триал истекает через N дней» в `active` — вторая итерация.
- Реальный редирект на провайдера — до подключения CloudPayments.
- Отмена подписки, история платежей, чеки — нет карты → нечего.
- Отдельный state-store (Zustand и т.п.) для подписки — guard держит состояние
  локально, страницы его не читают. Не нужен.
