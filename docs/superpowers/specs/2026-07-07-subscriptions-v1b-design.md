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

## Подтверждение capability (Freedom Pay) — проверено по докам 2026-07-07

Изучены docs.freedompay.kz. Конкретика:
- **CIT (первая оплата + сохранение карты):** `POST https://api.freedompay.kz/init_payment`
  (hosted-страница). Параметры вкл. `pg_merchant_id`, `pg_amount`, `pg_order_id`,
  `pg_description`, `pg_salt`, `pg_sig`, `pg_idempotency_key` (есть!). Токен карты
  приходит в callback (Result URL).
- **MIT (списание по токену):** `POST https://api.freedompay.kz/g2g/payment`
  («Token Pay»), `pg_card_token`, **синхронный** — результат в XML-ответе
  (`<pg_status>ok</pg_status>`, `pg_payment_id`, `pg_order_id`, `pg_sig`).
- **Единицы:** `pg_amount` — десятичное в валютных единицах (пример `123.12`,
  min `0.01`), `pg_currency` = ISO 4217. Для KZT — **тенге, без ×100**. Наш
  `amount int` (10000) передаётся как есть.
- **Подпись `pg_sig`:** MD5 от конкатенации `script_name` + все поля по алфавиту
  (включая `pg_salt`) + `secret_key`; 32-символьный lowercase hex. Ловушка:
  `script_name` для callback = последний сегмент вашего Result URL, для g2g =
  `payment` — уточнить на импле.
- **Оговорка:** токен-метод включается на уровне мерчант-аккаунта
  («contact your manager») — подтвердить активацию (Контингентность п.1).

**Важно для идемпотентности MIT:** у `g2g/payment` **НЕТ** `pg_idempotency_key`
и НЕТ документированного дедупа по `pg_order_id` (только «recommended unique»).
Значит провайдер НЕ защищает от повторного списания при ложном провале (таймаут
ответа при реально прошедшем списании). Митигация — reconciliation через
`status_v2` (проверка статуса платежа, есть в post-payment API): при ошибке/таймауте
`Charge` сперва спросить статус по `pg_order_id`, и только если «не оплачено» —
`MarkFailed`. См. Потоки → Автопродление.

Источники: docs.freedompay.kz/api-11620859 (init_payment), docs.freedompay.kz/api-9888724
(g2g Token Pay), freedompay.kz/docs/gateway-api/afterpay (post-payment: status_v2/refund),
docs.freedompay.kz/tokenize-card-11621153e0 (tokenize card — для change-card).

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
    // reconciliation при ошибке Charge (status_v2): "success"|"failed"|"unknown"
    CheckStatus(ctx context.Context, orderID string) (status string, err error)
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
  provider_payment_id TEXT UNIQUE,        -- второй рубеж дедупа (NULL до success)
  order_id            TEXT NOT NULL UNIQUE, -- ключ корреляции CIT/MIT + ON CONFLICT
  plan                TEXT NOT NULL,
  amount              INTEGER NOT NULL,     -- единицы уточнить: тенге vs тийины (×100)
  status              TEXT NOT NULL,      -- pending|success|failed
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE subscription_payments;
ALTER TABLE subscriptions
  DROP COLUMN card_token, DROP COLUMN autopay, DROP COLUMN pending_plan;
```

**Идемпотентность (одна модель для CIT и MIT):** строка платежа создаётся
**заранее** со `status='pending'` и `provider_payment_id=NULL`
(checkout для CIT, шедулер перед `Charge` для MIT). Активация — это условный
UPDATE, а не вставка:

```sql
UPDATE subscription_payments
   SET provider_payment_id = $1, status = 'success'
 WHERE order_id = $2 AND status = 'pending'
```

Активация периода происходит **только если `rows affected = 1`**. Повторный
«success» (передоставленный webhook или повторный тик MIT) находит строку уже в
`status='success'` → `rows affected = 0` → no-op. `UNIQUE(provider_payment_id)`
остаётся вторым рубежом (несколько NULL в Postgres разрешены, pending-строки не
конфликтуют). Тот же принцип, что идемпотентный `EndRoomByID` в livekit-webhook,
но ключ корреляции — `order_id`, единый для обоих потоков.

## Order-корреляция (webhook без JWT)

Webhook приходит **без JWT** (его шлёт провайдер). Личность несёт `order_id`,
сгенерённый на checkout и записанный в `subscription_payments` заранее
(`order_id = uuid`). Webhook делает lookup по строке → `(tutor_id, plan,
amount)`. Ничего не парсим из «структуры» order_id — просто поиск в БД.

## Потоки

**Первая оплата (CIT):**
```
POST /subscription/checkout → INSERT pending-строку (order_id=uuid, pp_id=NULL),
     InitPayment → вернуть hosted-URL (карта/Kaspi, галка "сохранить карту")
webhook success → проверить подпись → conditional UPDATE (см. Идемпотентность);
     если rows=1 → сохранить card_token, autopay=true, period_end = now()+interval
```

**Автопродление (MIT, свой шедулер):** фоновая горутина в `main.go` (как
auto-complete уроков), раз в сутки: берёт подписки с
`autopay=true AND now() >= period_end`. Для каждой, **до** списания:
1. `order_id = tutorID + ':' + period_end + ':' + today` (**по-дневный** ключ,
   `today` = дата тика `YYYY-MM-DD`). `INSERT … ON CONFLICT (order_id) DO NOTHING`
   pending-строку и смотрим `RowsAffected()`: **вставилось (rows=1) → claim наш,
   вызываем `Charge`; rows=0 → сегодня уже кто-то попытался (другой инстанс или
   этот же после краша), пропускаем**.
2. `Charge(orderID, token)` → conditional UPDATE (та же модель, что webhook).
   **При ошибке/таймауте `Charge`** — сперва `CheckStatus(orderID)` (провайдерский
   `status_v2`): если платёж на самом деле прошёл → трактуем как успех; только если
   «не оплачено» → `MarkFailed`. Так закрываем ложный провал (g2g не дедупит).

Успех → `period_end += interval` (аддитивно, дней не теряем) + применить
`pending_plan`, если задан. Провал → `status=failed`; `period_end` не двигается →
на **следующий день** новый `order_id` (другая дата) → insert-claim проходит →
ретрай. Так реализуются «ежедневные ретраи» внутри grace-окна. Нужен
`UNIQUE(order_id)` в схеме.

**Почему по-дневный ключ, а не по-периодный:** при провале `period_end` не
меняется; ключ `tutorID:period_end` был бы константой все 7 дней grace →
insert-claim сработал бы один раз → ретраев бы не было. Дата в ключе даёт ровно
один claim **в день**: enable-ретрай + защита от двойного списания в пределах дня
(краш после `Charge`, повтор тика).

**Мультиинстанс:** `FOR UPDATE SKIP LOCKED` НЕ нужен — по-дневный
`INSERT ON CONFLICT DO NOTHING` сам и есть мьютекс: из N инстансов вставку
выигрывает один (`rows=1`), остальные получают `rows=0` и пропускают. Row-lock не
держится во время HTTP-`Charge` (что было бы плохо).

**Остаточный риск (ложный провал) — ЗАКРЫТ reconciliation.** `Charge` реально
прошёл, но ответ потерян (таймаут) → без защиты назавтра новый ключ → двойное
списание. g2g НЕ дедупит (проверено), поэтому на ошибке `Charge` вызываем
`CheckStatus(orderID)` (`status_v2`) и `MarkFailed` только при подтверждённом
не-оплачено. `CheckStatus` — метод интерфейса `PaymentProvider`, логика reconcile
живёт в нашем тестируемом сервисе; фейк-провайдер её мокает.

**Dunning:** `EffectiveState` уже даёт `grace` 7 дней после `period_end`. В grace
фронт показывает баннер «оплата не прошла, обновите карту». На 8-й день →
`blocked` (402). Логику `EffectiveState` менять не требуется — шедулер просто
продолжает ретраить внутри окна.

Осознанный компромисс: `period_end += interval` при успехе на 5-й день grace
сжигает эти 5 дней (новый период отсчитан от старого `period_end`, не от `now()`).
Задумано как «не терять дни при раннем продлении»; в grace работает против юзера,
но приемлемо — альтернатива (отсчёт от `now()`) даёт бесплатный дрейф периода.

**Самообслуживание:**
- `POST /subscription/cancel` → `autopay=false`, `card_token=NULL`; доступ до
  `period_end`, затем blocked.
- `POST /subscription/change-card` → tokenize-only флоу (нулевая/verification-
  сумма, НЕ полный charge периода), перезапись `card_token`; период не трогаем.
  Уточнить у Freedom Pay способ токенизации без списания (см. Контингентность).
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

## Тесты

**Service-layer (мок PaymentProvider + SubscriptionRepository)** — проверяют
оркестрацию:
- успешный CIT → сохранён токен, autopay=true, период выставлен;
- dedup: репо вернул `activated=false` (повторный webhook) → период НЕ трогаем;
- успешный MIT-продление → `period_end += interval`;
- MIT-claim: репо вернул `claimed=false` (сегодня уже пытались) → `Charge` не
  вызывается;
- провал `Charge` → `MarkFailed`, период не двигается (→ `blocked` на 8-й день);
- cancel → `autopay=false`, `card_token=NULL`;
- pending_plan применяется на следующем продлении;
- плохая подпись webhook → 401.

**⚠️ Атомарность — НЕ покрыта моками.** Главный money-инвариант (два конкурентных
webhook / два инстанса → ровно один `rows=1`) — это SQL-атомарность, невидимая
моку. Codebase раньше не имел concurrency-critical money-пути, поэтому «всё на
моках» тут недостаточно. **Один реальный-Postgres тест** (файл под
`//go:build integration`, коннектится к `DB_URL` если задан, иначе `t.Skip`; в
`go test ./...` без БД не запускается) на две claim-операции:
- conditional UPDATE (`MarkPaymentSuccess`): два параллельных вызова с одним
  `order_id` → ровно один вернул `activated=true`;
- `INSERT … ON CONFLICT (order_id) DO NOTHING`: два параллельных вызова с одним
  `order_id` → ровно один `RowsAffected()=1`.

Без интеграционного окружения — как минимум явная строка в плане «атомарность
проверяется вручную goose-прогоном, не юнит-тестом», чтобы план не выдавал
непокрытый инвариант за покрытый.

## Секвенсирование (для плана)

Полное управление — большая поверхность до первого реального платежа. NB: этот
план строит **биллинг-стейт-машину** против фейкового `PaymentProvider` —
шипнуть его НЕ значит включить приём денег; реальный `freedompay`-адаптер и
frontend идут отдельными планами (гейтятся аппрувом Freedom Pay + доками).

1. Ядро (state machine): migration 019 → `PaymentProvider` интерфейс + `freedompay`
   импл → checkout (CIT) → webhook (подпись + dedup + активация) → шедулер
   автопродления → cancel. Удалить `confirm`-заглушку.
2. Расширения: change-card, change-plan (pending_plan).
3. Frontend: убрать клиентский `confirm`-POST, success/fail страницы с
   поллингом статуса, кнопки cancel/change-card/change-plan.

## Контингентность (подтвердить ДО реализации)

0. **Шаг 0 (юридический блокер всего запуска):** открыть ИП + получить
   `merchant_id` + секрет подписи (в `config.go`/`.env`). Без этого не стартует
   ничего, включая тестовую интеграцию.
1. Freedom Pay: в мерчант-аккаунте включён `pg_card_token` / MIT (гейтится
   менеджером) — активацию подтвердить с менеджером. **Capability по докам есть**
   (g2g/payment), но включение — по аккаунту.
2. ~~Способ токенизации без списания для `change-card`~~ — **есть эндпоинт
   «Tokenize card»** (docs.freedompay.kz/tokenize-card-11621153e0). Детали
   параметров — на импле change-card (отдельный план).
3. ~~Схема подписи `pg_sig`~~ — **выяснена:** MD5(`script_name` + поля по алфавиту
   c `pg_salt` + `secret_key`). Осталось подтвердить точное `script_name` для
   нашего callback-URL на импле freedompay-адаптера.
4. ~~Единицы суммы~~ — **решено:** `pg_amount` в тенге (десятичное), `×100` НЕ
   нужен. `amount int` KZT передаётся как есть.
5. ~~Дедупит ли `g2g/payment` по `order_id`?~~ — **выяснено: НЕТ** (ни
   `pg_idempotency_key`, ни документированного дедупа). Митигация принята:
   reconciliation через `status_v2` в `PaymentProvider.CheckStatus` (см. Потоки).
