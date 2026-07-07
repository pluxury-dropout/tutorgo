# Подписки v1b: токенизация + автосписание (Freedom Pay)

Дата: 2026-07-07. Статус: design (одобрен владельцем, план ещё не написан).
Связано: `2026-07-06-subscriptions-payment-provider.md` (выбор провайдера),
`2026-07-02-subscriptions-design.md`, `docs/superpowers/plans/2026-07-02-subscriptions.md`.

## Контекст и решение объёма

Backend подписок (фаза 1) и frontend в `main`. Enforcement
(`middleware.RequireActiveSubscription`) включён: `state==blocked` → 402 → paywall.

Оплата — заглушка (`service/subscription.go:57-71`): `Checkout` отдаёт фейковый
URL, `POST /subscription/confirm` активирует платный период **без проверки
оплаты** (любой залогиненный репетитор выдаёт себе подписку бесплатно). Закрыть
в первую очередь.

**Владелец выбрал v1b:** токенизация + автосписание с первого дня (retention
важнее скорости). Провайдер — **Freedom Pay** (см. предыдущий спек). Цены
актуальны: `PriceMonthly=10000`, `PriceYearly=90000` KZT.

### Решения владельца (2026-07-07)

1. **Движок автосписания:** свой шедулер + токен (не провайдер-рекуррент).
   Полный контроль над retries/grace/тарифами, вся логика у нас в коде и тестах.
2. **Dunning при неудачном списании:** grace 7 дней (существующий `GraceDays`) с
   ежедневными ретраями; на 8-й день → `blocked`.
3. **Самообслуживание:** полное управление — отмена + смена карты + смена тарифа.
4. **Смена тарифа:** отложенная (`schedule change`) — новый тариф применяется на
   следующем списании, без возвратов и перерасчёта середины периода.

## Подтверждение capability (Freedom Pay)

Модель «свой шедулер + токен» требует merchant-initiated списания по токену.
Подтверждено: Freedom Pay даёт Gateway API `g2g/payment` с `pg_card_token` и
явную модель CIT/MIT (customer- vs merchant-initiated). Оговорка: токен-метод
включается на уровне мерчант-аккаунта («contact your manager») — **подтвердить
активацию до реализации** (см. Контингентность).

Источники: freedompay.uz/docs-en/gateway-api/pay, freedompay.kg/docs-en/merchant-api/pay,
docs.recurly.com/recurly-subscriptions/docs/freedompay.

## Архитектура

Ложится на существующую цепочку `handlers → services → repositories → pgxpool`.
Провайдер прячется за одним интерфейсом (оправданный внешний I/O-seam;
мокается в тестах как репозитории сейчас):

```go
type PaymentProvider interface {
    // CIT: hosted-страница для первой оплаты, возвращает redirect URL
    InitPayment(ctx context.Context, orderID, tutorID, plan string, amount int) (redirectURL string, err error)
    // MIT: списание по сохранённому токену (g2g/payment + pg_card_token)
    Charge(ctx context.Context, orderID, token string, amount int) (providerPaymentID string, err error)
    // разбор + проверка подписи входящего webhook
    ParseCallback(r *http.Request) (Callback, error)
}
```

Одна реализация `freedompay` сейчас. Заглушки `Checkout`/`Confirm` удаляются.

## Схема БД (migration 019)

Токен — колонкой на `subscriptions` (одна карта на репетитора, YAGNI на
мультикарту). Отдельный лог платежей = таблица идемпотентности + аудит:

```sql
-- +goose Up
ALTER TABLE subscriptions
  ADD COLUMN card_token   TEXT,           -- NULL = автопродление выключено
  ADD COLUMN autopay      BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN pending_plan TEXT;           -- отложенная смена тарифа

CREATE TABLE subscription_payments (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tutor_id            UUID NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
  provider_payment_id TEXT UNIQUE,        -- дедуп повторных webhook
  order_id            TEXT NOT NULL,
  plan                TEXT NOT NULL,
  amount              INTEGER NOT NULL,
  status              TEXT NOT NULL,      -- pending|success|failed
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE subscription_payments;
ALTER TABLE subscriptions
  DROP COLUMN card_token, DROP COLUMN autopay, DROP COLUMN pending_plan;
```

**Идемпотентность:** активация периода делается только при успешной вставке
строки с новым `provider_payment_id`. Повторный «success»-webhook упирается в
unique-конфликт → no-op. Реккурент + ежедневные ретраи гарантированно
передоставят webhook — этот механизм не даёт растянуть период дважды. Тот же
паттерн, что идемпотентный `EndRoomByID` в livekit-webhook.

## Order-корреляция (webhook без JWT)

Webhook приходит **без JWT** (его шлёт провайдер). Личность несёт `order_id`,
сгенерённый на checkout и записанный в `subscription_payments` заранее
(`order_id = uuid`). Webhook делает lookup по строке → `(tutor_id, plan,
amount)`. Ничего не парсим из «структуры» order_id — просто поиск в БД.

## Потоки

**Первая оплата (CIT):**
```
POST /subscription/checkout → создать pending-строку в subscription_payments,
     InitPayment → вернуть hosted-URL (карта/Kaspi, галка "сохранить карту")
webhook success → проверить подпись → dedup-insert → сохранить card_token,
     autopay=true, period_end = now()+interval
```

**Автопродление (MIT, свой шедулер):** фоновая горутина в `main.go` (как
auto-complete уроков), раз в сутки: берёт подписки с
`autopay=true AND now() >= period_end`, для каждой `Charge(token)`. Успех →
`period_end += interval` (аддитивно, дней не теряем) + применить `pending_plan`,
если задан. Провал → остаётся в grace, ретрай на следующем тике.

**Dunning:** `EffectiveState` уже даёт `grace` 7 дней после `period_end`. В grace
фронт показывает баннер «оплата не прошла, обновите карту». На 8-й день →
`blocked` (402). Логику `EffectiveState` менять не требуется — шедулер просто
продолжает ретраить внутри окна.

**Самообслуживание:**
- `POST /subscription/cancel` → `autopay=false`, `card_token=NULL`; доступ до
  `period_end`, затем blocked.
- `POST /subscription/change-card` → новый CIT-checkout, перезапись токена;
  период не трогаем.
- `POST /subscription/change-plan` → `pending_plan=<new>`; применится на
  следующем списании (ноль возвратов/перерасчёта).

## Обработка ошибок

- Webhook: плохая подпись → 401; неизвестный `order_id` → 200 + log (не ретраить
  чужое); ошибка активации → 5xx (провайдер передоставит, dedup спасёт от дубля).
- UX возврата: юзер возвращается на success-страницу раньше, чем отработал
  webhook → фронт **поллит** `GET /subscription` с «Обрабатываем оплату…», пока
  `state` не станет активным.
- `Charge` упал: записать `subscription_payments(status=failed)`, оставить в grace.

## Маршруты вне гейта

`/subscription`, `/subscription/checkout`, `/subscription/webhook`,
`/subscription/cancel`, `/subscription/change-card`, `/subscription/change-plan`
— все ВНЕ `RequireActiveSubscription` (иначе заблокированный не сможет оплатить).
`webhook` дополнительно вне JWT-middleware (публичный, как `/webhooks/livekit`).

## Тесты (service-layer, мок PaymentProvider + SubscriptionRepository)

- успешный CIT → сохранён токен, autopay=true, период выставлен;
- dedup: повторный «success»-webhook с тем же `provider_payment_id` → no-op;
- успешный MIT-продление → `period_end += interval`;
- провал списания → остаётся в grace → `blocked` на 8-й день;
- cancel → останавливает списание, доступ до `period_end`;
- pending_plan применяется на следующем продлении;
- плохая подпись webhook → 401.

## Секвенсирование (для плана)

Полное управление — большая поверхность до первого реального платежа. План
строит сначала revenue-путь, потом расширения отдельными задачами:

1. Revenue-путь: migration 019 → `PaymentProvider` интерфейс + `freedompay`
   импл → checkout (CIT) → webhook (подпись + dedup + активация) → шедулер
   автопродления → cancel. Удалить `confirm`-заглушку.
2. Расширения: change-card, change-plan (pending_plan).
3. Frontend: убрать клиентский `confirm`-POST, success/fail страницы с
   поллингом статуса, кнопки cancel/change-card/change-plan.

## Контингентность (подтвердить ДО реализации)

1. Freedom Pay: в мерчант-аккаунте включён `pg_card_token` / MIT (гейтится
   менеджером).
2. Точная схема подписи callback (`pg_sig`) из доков Freedom Pay — образец из
   livekit-webhook НЕ подходит (валидирует подпись LiveKit своим ключом), пишем
   проверку с нуля.
3. Открыть ИП + получить `merchant_id` + секрет подписи (в `config.go`/`.env`).
