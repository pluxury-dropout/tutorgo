# Whiteboard — техническое описание

Совместная интерактивная доска для занятий. Репетитор и ученик рисуют на одном
холсте в реальном времени. Frontend — [tldraw](https://tldraw.dev), backend —
Go/Gin + WebSocket-хаб, хранение снапшотов в PostgreSQL (JSONB).

## Доменная модель

Одна доска привязана к курсу (`UNIQUE(course_id)`), внутри неё — несколько
страниц-холстов.

| Сущность      | Назначение                                                        |
|---------------|-------------------------------------------------------------------|
| `Board`       | 1:1 с курсом, принадлежит тьютору                                  |
| `BoardPage`   | холст (одна вкладка), хранит `snapshot JSONB` + `position`         |
| `BoardAsset`  | загруженный файл (PDF/картинка) в S3-хранилище, метаданные в БД    |
| `BoardInvite` | UUID-инвайт для гостевого доступа ученика (`UNIQUE(board_id)`)     |

Миграция: `migrations/014_whiteboard.sql`. Все FK на `boards` — `ON DELETE CASCADE`,
поэтому удаление курса/доски вычищает страницы, ассеты и инвайты.

## Слои (по архитектуре проекта)

```
router → WhiteboardHandler / WbHubManager → WhiteboardService → WhiteboardRepository → pgxpool
```

- `models/whiteboard.go` — структуры + validate-теги на request-моделях.
- `repository/whiteboard.go` — SQL.
- `service/whiteboard.go` — бизнес-логика и проверки владения.
- `handlers/whiteboard.go` — REST.
- `handlers/whiteboard_ws.go` — WebSocket-хаб (реал-тайм).

## REST API

Защищённые (JWT, scope `tutorID`):

| Метод  | Путь                          | Действие                              |
|--------|-------------------------------|---------------------------------------|
| GET    | `/boards/course/:courseId`    | get-or-create доска + страницы         |
| POST   | `/boards/:boardId/pages`      | создать страницу                       |
| PUT    | `/board-pages/:pageId`        | переименовать / переместить            |
| DELETE | `/board-pages/:pageId`        | удалить страницу                       |
| POST   | `/boards/:boardId/invite`     | создать инвайт                         |
| DELETE | `/boards/:boardId/invite`     | отозвать инвайт                        |
| POST   | `/boards/:boardId/assets`     | загрузить файл (multipart, ≤20 МБ)     |

Публичные (без JWT):

| Метод | Путь                          | Действие                               |
|-------|-------------------------------|----------------------------------------|
| GET   | `/public/board/join/:token`   | вход ученика по инвайту → доска+страницы|
| GET   | `/public/board-assets/:id`    | presigned-редирект на файл в S3         |
| GET   | `/ws/board/:pageId?token=...` | WebSocket-апгрейд (см. ниже)            |

`GetOrCreateBoard` — идемпотентный: если доски нет, создаёт её и первую страницу
«Страница 1». Из-за `ON CONFLICT DO NOTHING` по `course_id` сервис дополнительно
сверяет `board.TutorID == tutorID`, чтобы upsert не вернул чужую доску.

Загрузка ассетов льёт объект в S3 (`storage.Client.Put`) под ключ
`board-assets/<uuid><ext>`; если запись метаданных в БД упала (например, доска не
принадлежит тьютору) — объект удаляется обратно (`store.Remove`). Лимит 20 МБ
навешен и через `http.MaxBytesReader`, и через проверку `header.Size`.

## Методы по слоям

Цепочка для каждой фичи: **handler → service → repository**. Handler никогда не
ходит в repo напрямую; вся авторизация владения — в service.

### Доска

| Слой    | Метод | Суть |
|---------|-------|------|
| handler | `GetBoardByCourse` | читает `tutorID` из контекста, делегирует, маппит ошибку |
| service | `GetOrCreateBoard` | (1) `CourseBelongsToTutor`, (2) upsert, (3) сверка `board.TutorID`, (4) создаёт «Страница 1» если страниц 0 |
| repo    | `GetOrCreateBoard` | `INSERT … ON CONFLICT (course_id) DO UPDATE … RETURNING` — `DO UPDATE` (не `DO NOTHING`) чтобы `RETURNING` вернул строку и при конфликте |
| repo    | `CourseBelongsToTutor`, `GetBoardByID` | вспомогательные проверки |

Двойная проверка владения (до upsert через курс + после через `board.TutorID`)
закрывает дыру: upsert игнорирует `tutor_id` при конфликте и мог бы вернуть чужую доску.

### Страницы

| Слой    | Метод | Суть |
|---------|-------|------|
| handler | `CreatePage` / `UpdatePage` / `DeletePage` | bind+validate, делегация |
| service | `CreatePage` | `verifyBoardOwnership` → `position = len(pages)` (в конец) |
| service | `UpdatePage` / `DeletePage` | грузит страницу → `verifyBoardOwnership` по её `board_id` → delegate |
| repo    | `UpdatePage` | `SET title = COALESCE($2, title)` — partial update: `nil`-указатель оставляет старое значение |
| repo    | `DeletePage` | условие `id = $1 AND board_id = $2` — второй фактор от удаления чужой страницы |
| repo    | `SaveSnapshot` / `GetPageSnapshot` | чтение/запись `snapshot JSONB`; вызывается из WS-хаба, не из REST |

### Инвайты

| Слой    | Метод | Суть |
|---------|-------|------|
| handler | `CreateInvite` / `DeleteInvite` | JWT, делегация |
| handler | `JoinByInvite` | **публичный**, без JWT — вход по токену |
| service | `CreateInvite` / `DeleteInvite` | `verifyBoardOwnership` + delegate |
| service | `ValidateInvite` | находит доску по инвайту, возвращает доску+страницы (без проверки владения — ссылка и есть пропуск) |
| repo    | `CreateInvite` | `ON CONFLICT (board_id) DO UPDATE SET id = gen_random_uuid()` — создать-или-ротировать; старая ссылка протухает |
| repo    | `GetBoardByInvite` | JOIN `board_invites` → `boards` |

### Ассеты

| Слой    | Метод | Суть |
|---------|-------|------|
| handler | `UploadAsset` | `MaxBytesReader` + `header.Size` → `store.Put` в S3 → `SaveAsset`; при ошибке БД — `store.Remove` (откат объекта) |
| handler | `ServeAsset` | **публичный**, presigned-редирект на S3 (TODO: TTL + формат ответа) |
| service | `SaveAsset` | `verifyBoardOwnership` + `CreateAsset` |
| service | `GetAsset` | **без** проверки владения — read публичный |
| repo    | `CreateAsset` / `GetAsset` | INSERT/SELECT по `board_assets` (`file_path` хранит S3 object key) |

Асимметрия осознанная: **запись** ассета проверяет владельца, **чтение** — нет, потому
что `ServeAsset` отдаёт картинки студенту без JWT. Безопасность держится на
неугадываемом UUID ассета + коротком TTL presigned-ссылки.

### Сквозные паттерны

- `verifyBoardOwnership` (service) — единая точка авторизации: грузит доску, сверяет
  `TutorID`, на несовпадении/отсутствии → `ErrNotFound` (не `Forbidden`, чтобы не
  раскрывать существование чужих досок).
- `PageBelongsToTutor` — проверка владения страницей (через её доску); используется
  WS-хабом при авторизации хендшейка.
- `handleServiceError` маппит доменные ошибки в HTTP-коды; handler не знает про статусы.

## Реал-тайм: WebSocket-хаб

Ядро — `WbHubManager` (один на приложение) и `wbHub` (один на `pageID`).

### Авторизация хендшейка

Браузер не может проставить заголовки при WS-апгрейде, поэтому маршрут публичный,
а токен приходит в `?token=`. `authorizeWS` принимает два вида токена:

1. **Tutor JWT** (та же схема, что `middleware.Auth`, HS256, claim `id`) →
   требует, чтобы страница принадлежала доске этого тьютора (`PageBelongsToTutor`).
2. **Invite UUID** → `ValidateInvite` находит доску, и `pageID` обязан быть среди
   её страниц.

`CheckOrigin` зеркалит CORS-allowlist роутера — защита от cross-site WS-hijacking.
Запросы без заголовка `Origin` (не-браузерные клиенты) пропускаются.

### Жизненный цикл хаба

```
ServeWS: авторизация → загрузить snapshot из БД → upgrade →
         getOrCreate(pageID, snapshot) → register → writePump (go) + readPump
```

`wbHub.run()` — единственная горутина, владеющая состоянием хаба (паттерн
«разделяемое состояние через каналы», без мьютексов на сам hub). На клиента —
2 горутины: `readPump` и `writePump`.

При подключении новому клиенту сразу шлётся полный `snapshot`. Когда уходит
последний клиент — хаб сохраняет снапшот в БД и самоуничтожается; манагер при этом
берёт `mu.Lock()`, перепроверяет пустоту и закрывает `done`, чтобы гонка
«register-after-close» завершилась повтором на свежем хабе (цикл в `ServeWS`).

### Протокол сообщений (`WbMsg`)

| `type`     | Направление        | Поведение хаба                                       |
|------------|--------------------|------------------------------------------------------|
| `snapshot` | клиент → хаб       | **сохранить** в память + debounce-запись в БД (2 с); НЕ ретранслировать |
| `update`   | клиент ↔ клиенты   | инкрементальный diff tldraw — ретранслировать дословно, **не** хранить |
| `cursor`   | клиент ↔ клиенты   | подставить `peerId` отправителя и ретранслировать    |

Ключевое разделение: **`update`** даёт мгновенную интерактивность (каждое
изменение летит всем сразу), а **`snapshot`** — источник истины для персистентности
(дебаунсится, чтобы не писать в БД на каждый штрих). Broadcast идёт всем, кроме
отправителя; «застрявший» клиент (полный `send`-буфер) выкидывается.

Таймауты: read-limit 512 КБ, read-deadline 60 с с pong-handler, ping каждые 30 с.

## Frontend (`useWhiteboardSync.ts`)

Хук на tldraw `TLStore`:

- **store пересоздаётся на каждый `pageId`** (`useMemo`), чтобы контент одной
  страницы не намешался на другую при переключении вкладок.
- Локальные изменения (`store.listen`, source `'user'`, scope `'document'`):
  сразу шлётся `update`-diff, и дебаунсится (1 с) полный `snapshot`.
- Входящие: `snapshot` → `loadSnapshot`, `update` → `put`/`remove` внутри
  `mergeRemoteChanges` (чтобы не зациклить эхо), `cursor` → стейт курсоров пиров.
- Реконнект: на `onclose` — retry через 2 с, но с защитой `closedRef` и сверкой
  `wsRef.current === ws`, чтобы «зомби»-сокет не ронял живое соединение
  (исторический баг этого модуля).
- Токен берётся через `getTokenAsync(token)` — ждёт in-flight refresh, чтобы WS не
  открылся со «протухшим» JWT (HTTP-интерсептор Axios сюда не достаёт).

## Известные ограничения

- `snapshot` хранится как один JSONB на страницу — нет истории версий/undo на
  сервере (undo живёт в tldraw на клиенте).
- Персистентность курсоров и `update`-diff отсутствует by design — пережили
  reconnect только через полный `snapshot`.
- Ассеты — в S3-совместимом хранилище (Supabase Storage) через `storage.Client`
  (`aws-sdk-go-v2`, path-style, presigned URL). См. `docs/p2-assets-object-storage.md`.
```
