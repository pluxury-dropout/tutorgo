# Whiteboard — Интерактивная доска

**Дата:** 2026-06-10
**Статус:** Approved

## Обзор

Интерактивная доска для репетиторов и учеников, привязанная к курсу. Поддерживает совместную работу в реальном времени, постоянное хранение, доступ вне урока (подготовка, домашнее задание). Технология: **tldraw** (React-библиотека) + **Go WebSocket Hub** + **PostgreSQL**.

---

## 1. Требования

- Доска привязана к курсу (индивидуальному или групповому), создаётся автоматически при первом открытии
- Несколько страниц (pages) на доску — репетитор создаёт, переименовывает, удаляет
- Инструменты: pen (рисование от руки), фигуры (прямоугольник, круг, стрелка), текст, изображения (PNG/JPG), ластик, выделение и перемещение элементов
- Загрузка PDF: страницы конвертируются в PNG через `pdf.js` в браузере, затем вставляются на холст как изображения
- Совместная работа в реальном времени: репетитор и ученик рисуют одновременно, видят курсоры друг друга
- Ученик получает доступ по постоянной guest-ссылке (без логина), student auth будет добавлен позже
- Доступ вне урока: и репетитор, и ученик могут открывать доску в любое время

---

## 2. Архитектура

```
Браузер (tldraw)
    │ рисует локально мгновенно (оптимистично)
    │ отправляет diff по WebSocket
    ▼
Go Backend
    ├── WebSocket Hub (горутина на страницу)
    │   ├── получает diff → рассылает всем в комнате
    │   └── debounce 2с → сохраняет snapshot в PostgreSQL
    └── REST API (CRUD + upload + invite)
        ▼
PostgreSQL (boards, board_pages, board_assets, board_invites)
```

**Hub-паттерн:** одна горутина-координатор на активную страницу. Каждое WebSocket-соединение имеет две горутины (readPump + writePump). Общение через Go-каналы, без мьютексов на map клиентов. При отключении клиента — немедленный flush snapshot в БД (не ждать debounce).

---

## 3. Схема БД

```sql
-- Доска курса (1:1 с course)
boards
  id         UUID PRIMARY KEY
  course_id  UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE
  tutor_id   UUID NOT NULL REFERENCES tutors(id)
  created_at TIMESTAMPTZ DEFAULT now()

-- Страницы доски
board_pages
  id         UUID PRIMARY KEY
  board_id   UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE
  title      VARCHAR(100) NOT NULL DEFAULT 'Страница 1'
  snapshot   JSONB        -- tldraw state
  position   INTEGER NOT NULL DEFAULT 0
  created_at TIMESTAMPTZ DEFAULT now()
  updated_at TIMESTAMPTZ DEFAULT now()

-- Загруженные файлы (PNG, PDF→PNG страницы)
board_assets
  id         UUID PRIMARY KEY
  board_id   UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE
  file_path  VARCHAR NOT NULL   -- путь на диске
  mime_type  VARCHAR(50) NOT NULL
  size_bytes INTEGER NOT NULL
  created_at TIMESTAMPTZ DEFAULT now()

-- Guest-ссылки для учеников
board_invites
  id         UUID PRIMARY KEY   -- сам UUID = токен ссылки
  board_id   UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE
  created_at TIMESTAMPTZ DEFAULT now()
```

Миграция: `015_whiteboard.sql`.

---

## 4. Backend API

### REST (защищённые — JWT репетитора)

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/boards/course/:courseId` | Получить/создать доску курса |
| GET | `/boards/:boardId/pages` | Список страниц |
| POST | `/boards/:boardId/pages` | Создать страницу |
| PUT | `/board-pages/:pageId` | Переименовать / изменить порядок |
| DELETE | `/board-pages/:pageId` | Удалить страницу |
| POST | `/boards/:boardId/invite` | Создать guest-ссылку (один активный инвайт на доску — предыдущий удаляется) |
| DELETE | `/boards/:boardId/invite` | Отозвать guest-ссылку |
| POST | `/boards/:boardId/assets` | Загрузить файл (multipart, лимит 20MB) |

### REST (публичные — по guest-токену)

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/public/board/join/:token` | Валидировать ссылку → вернуть boardId |
| GET | `/public/board-assets/:id` | Отдать файл ученику |

### WebSocket

```
GET /ws/board/:pageId?token=<jwt или invite_id>
```

При подключении сервер определяет тип участника:
- JWT в `token` → репетитор (полный доступ)
- UUID из `board_invites` в `token` → ученик (полный доступ на Phase 1)

### WebSocket протокол

```json
// Клиент → Сервер
{ "type": "update", "payload": { /* tldraw diff */ } }
{ "type": "cursor",  "x": 120.5, "y": 340.2 }

// Сервер → Клиент (при подключении)
{ "type": "snapshot", "payload": { /* полный tldraw state */ } }

// Сервер → Клиент (от другого участника)
{ "type": "update", "payload": { /* tldraw diff */ } }
{ "type": "cursor",  "peerId": "...", "x": 120.5, "y": 340.2 }
```

---

## 5. Frontend

### Маршруты

```
/boards/[courseId]       — доска репетитора (защищена auth)
/board/join/[token]      — вход ученика по guest-ссылке
```

### Структура компонентов

```
BoardPage
├── PageSidebar          — список страниц, кнопка "+ Страница"
├── BoardToolbar         — кнопка "Пригласить ученика" (копирует ссылку)
└── TldrawCanvas
    ├── useWhiteboardSync(pageId, token?)
    └── <Tldraw />       — библиотека
```

### Хук useWhiteboardSync

1. Подключиться к `/ws/board/:pageId?token=...`
2. Получить `snapshot` → передать в tldraw как начальный state
3. `tldraw.onChange` → отправить `update` по WS
4. `WS.onmessage(update)` → применить diff к tldraw state
5. `WS.onmessage(cursor)` → показать курсор через tldraw `<CollaboratorCursor>`
6. При закрытии WS → показать "⚠ Переподключение..." → retry через 2с

### PDF upload flow

1. Пользователь выбирает PDF файл
2. `pdf.js` рендерит выбранные страницы в PNG в браузере
3. `POST /boards/:boardId/assets` (multipart)
4. Сервер сохраняет в `uploads/board-assets/`, возвращает URL
5. URL вставляется в tldraw как image-элемент

---

## 6. Edge Cases

| Кейс | Обработка |
|------|-----------|
| Клиент отключился | Hub немедленно сохраняет pending snapshot в БД |
| Потеря соединения | Frontend показывает индикатор, автоматически переподключается |
| Одновременные изменения | tldraw CRDT разрешает автоматически, сервер не вмешивается |
| Доска не существует | `GET /boards/course/:courseId` создаёт доску + первую страницу |
| PDF > 20MB | Сервер возвращает 413, frontend показывает ошибку |
| Невалидный guest token | WS отклоняет соединение с кодом 4001 |

---

## 7. Файловая структура (новые файлы)

```
migrations/
  015_whiteboard.sql

handlers/
  whiteboard.go          — REST handlers
  whiteboard_ws.go       — WebSocket hub + client

repository/
  whiteboard.go          — CRUD для boards/pages/assets/invites

service/
  whiteboard.go          — бизнес-логика

frontend/src/
  app/(dashboard)/boards/[courseId]/page.tsx
  app/board/join/[token]/page.tsx
  components/whiteboard/
    TldrawCanvas.tsx
    PageSidebar.tsx
    BoardToolbar.tsx
    useWhiteboardSync.ts
```

---

## 8. Зависимости

**Frontend:**
- `@tldraw/tldraw` — canvas + инструменты
- `pdfjs-dist` — рендеринг PDF → PNG в браузере

**Backend:**
- `github.com/gorilla/websocket` — уже есть в `go.mod` как indirect, перевести в direct

**Не нужно:**
- Отдельный sync-сервер (hub встроен в Go-бэк)
- Redis / внешний брокер сообщений (масштаб одного репетитора не требует)
