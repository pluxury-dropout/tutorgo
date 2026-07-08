# Frontend подписок v1b: hosted-checkout + self-service

**Дата:** 2026-07-08
**Статус:** дизайн одобрен, готов к плану

## Проблема

Backend billing-core v1b смержен в `main`. Task 7 удалил роут `POST /subscription/confirm`,
но фронт всё ещё его дёргает (`api/subscription.ts` → `subscription/page.tsx:45`).
Деплой backend'а раньше фронта → клиент 404-ит на confirm. Порядок выката: **frontend → backend**.

Текущий `onPay` — антипаттерн hosted-платежей: клиент сам «подтверждает» оплату
(`checkout()` + `confirm()`). Правильно: redirect на hosted-страницу провайдера →
активация через webhook (сервер-сайд) → возврат на success-страницу с поллингом.

## Scope

Полный self-service против **фейкового** провайдера (реальный `checkout_url` даёт
freedompay-адаптер — отдельный план). Включает: redirect-checkout, success-страницу
с поллингом, cancel, change-plan.

**НЕ входит (осознанно):**
- freedompay-адаптер (`/checkout` пока отдаёт 5xx `payment provider not configured` —
  «Оплатить» падает с тостом, это ожидаемо до подключения Freedom Pay).
- `change-card` (tokenize-only, зависит от способа токенизации у Freedom Pay).
- Настройка `pg_success_url` / `pg_failure_url` — параметры бэкового `InitPayment`
  внутри freedompay-адаптера, не фронта.

## Дизайн

### 1. Backend-довесок: расширить статус подписки

`GET /subscription` (`models.SubscriptionStatus`) сейчас отдаёт `{state, plan, period_end, prices}`.
Добавить 2 поля, чтобы UI мог корректно показывать состояние управления:

- `autopay bool` — включено ли автопродление (прятать «Отменить» после отмены).
- `pending_plan string|null` — отложенная смена тарифа (показать «со следующего периода: X»).

Без них UI слепнет: после `cancel` статус выглядит идентично (active + plan) → кнопка
«Отменить» не исчезнет до перезагрузки; pending-смена тарифа не видна вовсе.

**Затрагивает:**
- `models.Subscription` — +`Autopay bool`, +`PendingPlan *string`.
- `repository.GetByTutor` — SELECT +2 колонки (`autopay`, `pending_plan`).
- `models.SubscriptionStatus` — +`Autopay bool json:"autopay"`, +`PendingPlan *string json:"pending_plan"`.
- `service.GetStatus` — populate из `sub.Autopay` / `sub.PendingPlan`.
- Обновить repo/service-тесты (новые колонки в моках/фикстурах).

### 2. `frontend/src/lib/api/subscription.ts`

- `checkout(plan)` → возвращает `{ checkout_url: string }` (сейчас выбрасывает — исправить).
- **Удалить** `confirm()` (роута нет).
- Добавить `cancel()` → `POST /subscription/cancel`.
- Добавить `changePlan(plan)` → `POST /subscription/change-plan`.
- Тип `Subscription` — +`autopay: boolean`, +`pending_plan: Plan | null`.

### 3. Страница `/subscription` (`app/(dashboard)/subscription/page.tsx`)

**`onPay(plan)`:** `checkout(plan)` → `window.location.href = checkout_url`. Убрать
`confirm()` и блок «активировано / hard-nav на dashboard» (активация теперь webhook).
Ошибка checkout → тост «Не удалось начать оплату».

**Блок управления** (рендерить когда `plan !== null`):
- `autopay === true` → кнопка «Отменить подписку» с confirm-диалогом → `cancel()` → refetch.
  Текст диалога: доступ сохраняется до `period_end`, затем блокировка.
- `pending_plan != null` → строка «Со следующего периода: {label(pending_plan)}».
- иначе (autopay on, нет pending) → кнопка «Перейти на {другой тариф}» → `changePlan(other)` → refetch.
  Тарифов два → «сменить» = переключить на другой.

Тарифные карточки с «Оплатить» показывать когда `plan === null` **или** `state !== 'active'`
(в grace/blocked — дать переоплатить).

### 4. Новая страница `/subscription/success` (`app/(dashboard)/subscription/success/page.tsx`)

Экран «Проверяем оплату…» + поллинг `GET /subscription`:
- интервал 2с, таймаут ~30с (15 попыток).
- `state === 'active'` → тост «Подписка активирована» → `window.location.href = '/dashboard'`
  (hard-nav: layout группы (dashboard) держит устаревший subState → нужен ремоунт; тот же
  нюанс, что задокументирован в текущем `subscription/page.tsx`).
- таймаут → «Оплата обрабатывается, обновите позже» + кнопка «В дашборд».

failure-URL Freedom Pay → `/subscription?status=failed`. Страница `/subscription` читает
query `status` и на `failed` показывает тост «Оплата не прошла, попробуйте ещё раз».

## Порядок выката

1. Смержить этот фронт в `main` и **задеплоить фронт**.
2. Только потом деплоить backend (иначе клиент 404-ит на удалённый `/subscription/confirm`).
3. freedompay-адаптер + активация Freedom Pay — отдельно, включает реальный приём денег.

## Тесты

- Backend: обновить существующие repo/service-тесты под новые поля; проверить, что
  `GetStatus` возвращает `autopay`/`pending_plan`.
- Frontend: тесты на `node:test` (проект без vitest/jest). Минимум — поведение поллинга
  success-страницы (active → редирект, таймаут → сообщение) как чистая функция/хелпер,
  если UI-компонент тестировать тяжело.
