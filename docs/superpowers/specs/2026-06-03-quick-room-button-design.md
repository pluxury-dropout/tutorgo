# Кнопка «Начать урок» — Quick Room

**Дата:** 2026-06-03

## Суть фичи

В сайдбаре под мини-календарём и мини-расписанием добавляется кнопка «Начать урок».
Нажатие создаёт одноразовую видеокомнату (без привязки к конкретному уроку из БД),
сразу открывает LiveKit-сессию и показывает кнопку копирования ссылки для ученика.

## Пользовательский поток

1. Репетитор нажимает «Начать урок» в сайдбаре
2. Фронтенд вызывает `POST /calls/quick` → получает `{room_id, token, server_url}`
3. Токен и server_url сохраняются в `sessionStorage` по ключу `quick-room-{room_id}`
4. Переход на `/room/{room_id}` — страница читает токен из sessionStorage и сразу подключается к LiveKit
5. Внутри комнаты: кнопка «Ссылка для ученика» копирует `/join/room/{room_id}`
6. Ученик открывает ссылку → `/join/room/{room_id}` → вводит имя → ждёт активной комнаты → входит
7. При выходе репетитора: вызов `POST /calls/quick/{id}/end`, комната удаляется из LiveKit

## Архитектурные решения

**Хранение состояния:** in-memory map в `CallHandler` (`map[string]*quickRoom` + `sync.RWMutex`).
Миграции не нужны. При рестарте сервера активные комнаты сбрасываются — допустимо для одноразовых сессий.

**Имя комнаты в LiveKit:** `quick-{room_id}` (UUID) — изолировано от урочных комнат `lesson-{id}`.

**Передача токена на страницу:** через `sessionStorage` — избегает лишнего запроса к бэкенду при загрузке страницы.

## Бэкенд — новые эндпоинты

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| POST | `/calls/quick` | JWT | Создать комнату, вернуть `{room_id, token, server_url}` |
| POST | `/calls/quick/:id/end` | JWT | Завершить комнату, удалить из LiveKit |
| GET | `/public/quick/:id/status` | — | Статус комнаты (`active`/`ended`) |
| GET | `/public/quick/:id/guest-token` | — | Токен для ученика (только если `active`) |

### Структура данных

```go
type quickRoom struct {
    tutorID string
    active  bool
}
// В CallHandler:
quickRooms map[string]*quickRoom
quickMu    sync.RWMutex
```

## Фронтенд — новые файлы

| Файл | Назначение |
|------|-----------|
| `src/lib/api/calls.ts` | +4 функции: `startQuickRoom`, `endQuickRoom`, `getQuickRoomStatus`, `getQuickGuestToken` |
| `src/app/api/quick-status/[roomId]/route.ts` | Next.js прокси → `/public/quick/:id/status` |
| `src/app/api/quick-guest-token/[roomId]/route.ts` | Next.js прокси → `/public/quick/:id/guest-token` |
| `src/app/(dashboard)/room/[id]/page.tsx` | Страница репетитора: читает sessionStorage → сразу LiveKit |
| `src/app/join/room/[id]/page.tsx` | Страница ученика: идентична `/join/[lessonId]` но через quick-эндпоинты |

## Изменения в существующих файлах

| Файл | Изменение |
|------|-----------|
| `handlers/call.go` | +4 метода, +`quickRooms` map + mutex в struct |
| `router/router.go` | +4 маршрута (2 публичных, 2 защищённых) |
| `components/layout/Sidebar.tsx` | Кнопка в `CalendarSidebarPanel` под `TodayList` |

## Дизайн кнопки (вариант A)

Минималистичный статус-бар — органично вписывается в сайдбар:

```
┌──────────────────────────────────────┐
│ ● Начать урок                      → │
└──────────────────────────────────────┘
```

- Зелёная пульсирующая точка (CSS `animation`)
- Фон `var(--sidebar-hover-bg)`, граница `var(--border)`
- При клике: кнопка переходит в состояние загрузки (`Connecting...`) пока не завершится `startQuickRoom`
- После ответа: переход на `/room/{room_id}`

## Граничные случаи

- **sessionStorage пуст при загрузке `/room/[id]`** → редирект на `/dashboard` (прямое открытие ссылки)
- **`startQuickRoom` упала** → показать `toast` с ошибкой, кнопка возвращается в исходное состояние
- **LiveKit не настроен** → бэкенд вернёт 503, обрабатывается как ошибка
- **Ученик открывает ссылку до входа репетитора** → статус `ended` (комната ещё не создана в памяти) → показать «Урок ещё не начался»

> **Примечание:** Последний кейс — ученик приходит до репетитора — вернёт `ended` вместо `waiting`,
> так как комнаты нет в памяти до вызова `POST /calls/quick`. Это ограничение in-memory подхода,
> приемлемо для одноразовых сессий.
