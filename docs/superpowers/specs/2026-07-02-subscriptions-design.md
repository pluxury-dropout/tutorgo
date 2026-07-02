# Система подписок (SaaS-биллинг для репетиторов)

**Дата:** 2026-07-02
**Статус:** дизайн утверждён, готов к плану

## Проблема и цель

TutorGo — SaaS, где пользователи (клиенты) — это **репетиторы**. Сейчас доступ
бесплатный и безлимитный. Нужно монетизировать: новый репетитор получает
бесплатный пробный месяц, после чего выбирает платный тариф — помесячно или
годовой со скидкой.

Важно не путать две «оплаты»:
- Существующая таблица `payments` — деньги **учеников репетитору** (учёт внутри продукта). Не трогаем.
- Новая подписка — деньги **репетитора платформе** (монетизация SaaS). Отдельная таблица и логика.

## Утверждённые решения

| Вопрос | Решение |
|---|---|
| Кто платит | Репетитор (scoped по `tutorID`, как всё остальное) |
| Рынок | KZ (KZT) сейчас → СНГ потом |
| Провайдер оплаты | **Заглушка** сейчас; сменный слой под CloudPayments позже |
| Пробный период | 30 дней, **без карты**, стартует при регистрации |
| Тарифы | **Один** тариф, выбор только периода: месяц / год-со-скидкой |
| Enforcement | Грейс-период **7 дней** → блокировка |
| Старые репетиторы | **Grandfather** — бесплатно навсегда |

## Модель данных

Одна таблица `subscriptions`, строка 1:1 с репетитором.

| колонка | тип | смысл |
|---|---|---|
| `tutor_id` | uuid PK, FK → tutors ON DELETE CASCADE | владелец |
| `plan` | text NULL | `NULL` = пробный период; иначе `monthly` / `yearly` |
| `period_end` | timestamptz NULL | конец текущего периода; `NULL` только для grandfather |
| `grandfathered` | boolean NOT NULL DEFAULT false | старые юзеры — бесплатно навсегда |
| `created_at` | timestamptz NOT NULL DEFAULT now() | |
| `updated_at` | timestamptz NOT NULL DEFAULT now() | |

**Нет хранимого `status`. Нет крона.** Состояние доступа вычисляется на чтении
из `period_end` + текущего времени. Это несущее упрощение дизайна.

Инвариант: **отсутствие строки = доступ заблокирован** (защита от битой
регистрации, которая иначе молча раздавала бы бесплатный доступ).

## Функция доступа (ядро — пишется первой, через TDD)

```
effectiveState(sub, now) → "active" | "grace" | "blocked":
  if sub == nil            → "blocked"   # нет строки = блок
  if sub.grandfathered     → "active"    # short-circuit ДО сравнения дат (period_end == NULL)
  if now <  period_end               → "active"   # trial или оплаченный период
  if now <  period_end + 7 дней      → "grace"
  else                               → "blocked"
```

- `active` и `grace` → доступ разрешён. Разница только для фронта: в `grace`
  показывается баннер «оплатите, доступ скоро закроется».
- `blocked` → доступ к бизнес-ручкам закрыт (402).
- Grace = **7 дней** (константа).

Это чистая функция без БД и без времени внутри (`now` передаётся аргументом) —
полностью юнит-тестируемая. **Тесты границ обязательны:** непосредственно до
`period_end`, внутри grace, после grace, grandfathered (с `period_end == NULL`),
`sub == nil`.

## Enforcement (middleware)

Сегодняшняя единая группа `auth` (в `router/router.go`) делится на две подгруппы
под тем же `middleware.Auth`:

**Открыто всегда (даже при `blocked`)** — чтобы заблокированный репетитор мог
увидеть счёт и заплатить:
- `GET  /subscription`
- `POST /subscription/checkout`
- `POST /subscription/confirm` (заглушка; позже → вебхук)
- `GET  /tutors/:id`
- `PUT  /tutors/:id`
- `POST /auth/logout` (уже публичный — остаётся как есть)

**Гейтится `middleware.RequireActiveSubscription`** — всё остальное под `auth`:
students, courses, lessons, payments, tasks, enrollments, attendance, calls,
whiteboard-ручки и т.д. При `effectiveState == "blocked"` → `402 Payment Required`
с телом `{"error":"subscription_required"}`.

Middleware читает подписку по `tutorID` из контекста (PK-lookup на запрос — это
нормально, без кэша). Состояние **не** класть в JWT: оно протухнет после
`period_end`.

## Эндпоинты

### `GET /subscription`
Возвращает состояние текущего репетитора и цены:
```json
{
  "state": "active",           // active | grace | blocked
  "plan": null,                 // null | "monthly" | "yearly"
  "period_end": "2026-08-01T...Z",
  "prices": { "monthly": 5000, "yearly": 48000, "currency": "KZT" }
}
```

### `POST /subscription/checkout`  `{ "plan": "monthly" | "yearly" }`
Начинает оплату. **Заглушка:** возвращает `{ "checkout_url": "<fake-url>" }`.
Позже здесь будет создание платёжной сессии у CloudPayments.

### `POST /subscription/confirm`
**Заглушка вместо вебхука** — «оплата прошла». Активирует подписку:
- ставит `plan` из последнего checkout (или принимает `plan` в теле — упростим: принимает `{plan}` в теле),
- `period_end = now + 30 дней` (monthly) или `now + 365 дней` (yearly),
- период стартует **с текущего момента** (остаток триала сгорает).
Позже заменяется на `POST /webhooks/payments` с проверкой подписи провайдера.

## Цены

Константы в KZT в коде (не env, не отдельная таблица — один тариф × два периода
= две константы). Годовой — со скидкой относительно 12×месяц.

```
monthly = 5000 KZT   // ponytail: значения-плейсхолдеры, уточнить перед запуском
yearly  = 48000 KZT  // ~20% скидка (12*5000=60000 → 48000)
```

## Регистрация

В хендлере/сервисе регистрации (`POST /auth/register`) строка подписки создаётся
**в той же транзакции**, что и `tutor`:
```
INSERT tutor(...)                                    -- существующее
INSERT subscription(tutor_id, plan=NULL, period_end=now()+30d, grandfathered=false)
```
Если любой из INSERT падает — откатывается всё. Инвариант «нет строки = блок»
требует, чтобы строка появлялась атомарно.

## Миграция 018

1. `CREATE TABLE subscriptions (...)` по схеме выше.
2. Бэкфилл: для **каждого существующего** репетитора вставить строку
   `grandfathered=true, plan=NULL, period_end=NULL`.
   ```sql
   INSERT INTO subscriptions (tutor_id, grandfathered)
   SELECT id, true FROM tutors
   ON CONFLICT (tutor_id) DO NOTHING;
   ```
3. Проверка: `SELECT count(*) FROM tutors` == `SELECT count(*) FROM subscriptions`
   после миграции — иначе непокрытый старый юзер будет залочен правилом
   «нет строки = блок».

`down`-миграция: `DROP TABLE subscriptions`.

## Что осознанно НЕ делаем сейчас (YAGNI)

- **Крон/фоновая задача** — не нужны: состояние вычисляется на чтении.
  Понадобятся только для dunning-писем или автосписания с карты — тогда добавим.
- **Интерфейс `PaymentProvider`** — не заводим. Одна конкретная функция-заглушка;
  интерфейс извлечём, когда CloudPayments станет *вторым* провайдером.
- **Отмена подписки / `canceled_at`** — не нужны: нет карты → нет автосписания →
  отменять нечего. Лапс сам течёт trial/paid → grace → block.
- **Таблица тарифов / `GET /subscription/plans`** — не нужны: один тариф, цены
  отдаются прямо в `GET /subscription`.
- **Уровни тарифов (Basic/Pro) и feature-gating** — не нужны: тариф один.

## Порядок реализации (для плана)

1. Функция `effectiveState` + юнит-тесты границ (TDD, первой).
2. Миграция 018 (таблица + бэкфилл grandfather).
3. Model + repository (`GetByTutor`, `Upsert`/`Activate`, `CreateTrial` в транзакции регистрации).
4. Service (обёртка над repo + `effectiveState`, константы цен).
5. Middleware `RequireActiveSubscription`.
6. Handlers (`GET /subscription`, `checkout`, `confirm`) + разбиение групп в `router.go`.
7. Интеграция trial-строки в регистрацию (та же транзакция).
