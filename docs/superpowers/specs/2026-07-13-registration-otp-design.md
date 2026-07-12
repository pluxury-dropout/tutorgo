# Регистрация с OTP-верификацией + защита от опечатки пароля

**Дата:** 2026-07-13
**Статус:** Утверждён, готов к плану реализации

## Проблема

Процесс регистрации тьютора остался незавершённым:
- нет подтверждения email (любой может зарегистрировать чужой/несуществующий адрес);
- нет повтора пароля (опечатка → пользователь заперт снаружи своего аккаунта);
- нет кнопки «показать пароль».

Сейчас `POST /auth/register` сразу создаёт `tutor` + trial-подписку в транзакции (`TutorService.Register`), без какой-либо верификации.

## Цели

1. Двухшаговая регистрация с 6-значным OTP-кодом на email.
2. Реальный аккаунт создаётся **только после** подтверждения кода (verify-before-create) — нет мусорных аккаунтов и левых trial-подписок в основной таблице.
3. Frontend: повтор пароля (ловит опечатку) + toggle «показать пароль».
4. Отправка почты через провайдер, масштабируемый без изменений кода (**Resend**).

## Не входит в scope (YAGNI)

- Password-reset flow — отдельный спек, переиспользует пакет `email` и OTP-механику.
- Абстракция `EmailProvider` с интерфейсом — одна функция `Send()`; смена провайдера = правка одной функции.
- CAPTCHA — добавляем, только когда увидим ботов; пока хватает rate-limit.
- Повтор пароля на сервере — валидируется на клиенте, серверу не нужен.

## Архитектура

### Пакет отправки почты (`email/`)

Одна функция, без SDK:

```go
// email/email.go
func Send(ctx context.Context, to, subject, htmlBody string) error
```

- Реализация: `POST https://api.resend.com/emails` через `net/http`, заголовок `Authorization: Bearer $RESEND_API_KEY`, тело `{from, to, subject, html}`.
- Конфиг (в `config.Load`): `RESEND_API_KEY`, `EMAIL_FROM`.
- **Dev-режим:** если `RESEND_API_KEY` пуст — логируем код через `slog` вместо отправки, `err = nil`. Позволяет разрабатывать без Resend-аккаунта.
  `// ponytail: пустой ключ = dev-режим, код в лог; прод обязан задать ключ`

Resend выбран за простоту (1 HTTP-запрос, 3k писем/мес бесплатно) и потолок по объёму — при росте меняется только тариф, не код. Альтернатива AWS SES отклонена из-за первичной возни (verified domain, sandbox-выход) при том же итоговом потолке.

### Данные — `migrations/024_pending_registrations.sql`

```sql
CREATE TABLE pending_registrations (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text        NOT NULL UNIQUE,     -- один pending на email
    password_hash text        NOT NULL,            -- bcrypt пароля
    first_name    text        NOT NULL,
    last_name     text        NOT NULL,
    phone         text        NOT NULL DEFAULT '',
    code_hash     text        NOT NULL,            -- bcrypt 6-значного кода
    attempts      int         NOT NULL DEFAULT 0,
    resend_at     timestamptz NOT NULL,            -- когда можно повторно отправить (кулдаун)
    expires_at    timestamptz NOT NULL,            -- created_at + 10 мин
    created_at    timestamptz NOT NULL DEFAULT now()
);
```

- `email UNIQUE` → повторный `POST /auth/register` тем же адресом перезаписывает pending (upsert).
- Код хранится как **bcrypt-хеш**: 6 цифр = 10⁶ вариантов, это пароль на 10 минут, в открытом виде в БД не держим.
- Просроченные строки чистит фоновый goroutine (см. ниже).

### Слои

Новый **`RegistrationService`** (отдельно от `TutorService`, чтобы OTP-логика не смешивалась с CRUD тьютора):

```go
type RegistrationService interface {
    // Start: проверяет занятость email, upsert pending, генерит код, шлёт письмо.
    Start(ctx, req StartRegistrationRequest) error
    // Verify: проверяет код; при успехе создаёт tutor+trial (via TutorService.Register),
    //         удаляет pending, возвращает созданного tutor.
    Verify(ctx, email, code string) (models.Tutor, error)
    // Resend: если прошёл кулдаун — новый код + письмо.
    Resend(ctx, email string) error
}
```

Зависимости: `PendingRegistrationRepository`, `TutorService` (для финального `Register`), `email.Send`.

Новый **`PendingRegistrationRepository`** — CRUD по `pending_registrations` (Upsert, GetByEmail, IncrementAttempts, Delete, DeleteExpired).

## Флоу и эндпоинты

Все под `/auth/*` (публичные, без middleware.Auth).

```
POST /auth/register        {email, password, first_name, last_name, phone?}
  1. валидация (email, password min=6, имена min=2)
  2. email уже есть в tutors? → 409 {"error":"Email or phone is already taken"}
  3. bcrypt(password), генерим код (crypto/rand, 6 цифр), bcrypt(code)
  4. upsert pending: expires_at = now+10m, resend_at = now+60s, attempts=0
  5. email.Send(...)
  6. → 202 Accepted (кода в ответе НЕТ)

POST /auth/register/verify {email, code}
  1. GetByEmail → нет / expires_at < now → 400 {"error":"Code expired, register again"}
  2. attempts >= 5 → Delete pending → 400 {"error":"Too many attempts, register again"}
  3. bcrypt.Compare(code_hash, code) неверно → IncrementAttempts → 400 {"error":"Invalid code"}
  4. верно → tx: TutorService.Register (tutor + trial) ; Delete pending
  5. ставим refresh-cookie, → 201 {access_token}   (пользователь сразу залогинен)

POST /auth/register/resend {email}
  1. GetByEmail → нет → 400
  2. resend_at > now → 429 {"error":"Wait before requesting a new code"}
  3. новый код, bcrypt, update code_hash + resend_at=now+60s + expires_at=now+10m, attempts=0
  4. email.Send → 202
```

**Изменение контракта:** прежний `POST /auth/register` возвращал `201 + tutor`. Теперь возвращает `202` без тела; аккаунт появляется после `/verify`. Frontend обновляется соответственно.

## Rate limiting

Middleware на `POST /auth/register` и `POST /auth/register/resend`: по IP (`c.ClientIP()`), лимит **5/час**. In-memory счётчик с TTL-окном (паттерн quick-rooms: `sync.RWMutex` + map, фоновая чистка).
`// ponytail: in-memory, per-instance; вынести в Redis при горизонтальном масштабировании`

Защищает и от абьюза (рассылка по чужим адресам), и от порчи sender-репутации у Resend.

## Фоновая очистка

В `main.go` добавить в набор `runIntervalLoop`:
```go
bgWg.Go(func() {
    runIntervalLoop(bgCtx, 10*time.Minute, "cleanup pending registrations",
        pendingRepo.DeleteExpired, log)
})
```
`DeleteExpired(ctx) (int64, error)` = `DELETE FROM pending_registrations WHERE expires_at < now()`. Сигнатура совпадает с существующими job'ами.

## Frontend

Страница регистрации → двухшаговая:

**Шаг 1 — форма данных:**
- Поля: email, password, **confirm_password**, first_name, last_name, phone.
- `confirm_password` валидируется на равенство `password` на клиенте перед сабмитом (ловит опечатку). На сервер не отправляется.
- **Toggle «показать пароль»:** кнопка внутри инпута, переключает `type` `password`↔`text`. Нативно, без библиотек. Применяется к обоим полям пароля.
- Сабмит → `POST /auth/register` → при 202 переход на шаг 2.

**Шаг 2 — ввод кода:**
- Инпут на 6 цифр + подпись «Код отправлен на {email}».
- Кнопка «Подтвердить» → `POST /auth/register/verify`. При 201 сохраняем `access_token`, редирект в кабинет (как после логина).
- Кнопка «Отправить ещё раз» → `POST /auth/register/resend`, дизейбл с таймером 60 сек.
- Показ ошибок из ответов (истёк код / неверный код / слишком много попыток).

## Тесты

- `RegistrationService`: Start (email занят → ошибка; happy path), Verify (expired, attempts-exceeded, wrong code, success создаёт tutor), Resend (кулдаун → ошибка). Мок `PendingRegistrationRepository` + `TutorService` + фейк `email.Send`.
- Handler-тесты на коды ответов (409/202/201/400/429) в стиле существующих `auth_test.go`.

## Открытые вопросы

Нет. Password-reset — отдельный спек.
