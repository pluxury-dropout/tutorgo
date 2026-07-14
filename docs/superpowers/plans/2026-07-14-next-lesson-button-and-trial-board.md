# Кнопка следующего урока + вкладка «Пробный урок» с общей доской — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Кнопка в сайдбаре запускает следующий урок по расписанию (появляется за 10 минут до старта, с названием курса), безымянные комнаты переезжают во вкладку «Пробный урок» и получают одну общую доску на препода.

**Architecture:** Выбор урока — чистая функция `pickActiveLesson` над данными, которые сайдбар уже получает из `useCalendar`; новых эндпоинтов для этого не нужно, клик ведёт на существующую `/lessons/[id]/call`. Пробная доска — та же таблица `boards`, но с `course_id IS NULL`: nullable-колонка + частичный уникальный индекс `UNIQUE (tutor_id) WHERE course_id IS NULL` даёт инвариант «одна пробная доска у препода» на уровне БД. Быстрые комнаты сегодня рендерят голый `<VideoConference />`, поэтому доски в них нет вообще — обе страницы (препод `/room/[id]` и гость `/join/room/[id]`) переводятся на `CallRoom`, который уже умеет доску, чат и тулбар.

**Tech Stack:** Go 1.x + Gin + pgx + goose; Next.js 15 + React + axios + LiveKit + Excalidraw; тесты — testify/mock (Go), `node:test` (frontend).

**Spec:** `docs/superpowers/specs/2026-07-14-next-lesson-button-and-trial-board-design.md`

## Global Constraints

- Окно показа кнопки урока: `[scheduled_at − 10 мин, scheduled_at + duration_minutes]`. Вне окна — кнопки нет.
- Только уроки со `status === 'scheduled'`.
- Пробная доска — ровно одна на препода; инвариант держит частичный уникальный индекс, не код.
- `models.Board.CourseID` остаётся `string`; для пробной доски он `""` (в SQL — `COALESCE(course_id::text, '')`). Модель и типы фронта не меняются.
- Все новые REST-роуты — в группе `auth` (`middleware.Auth` + `RequireActiveSubscription`), скоуп по `tutorID := c.GetString("tutorID")` с nil-guard (401, если пусто).
- Ошибки сервиса — только `service.ErrNotFound / ErrForbidden / ErrConflict / ErrBadRequest`, хендлер отдаёт их через `handleServiceError(c, err)`.
- Русский язык в UI-текстах и комментариях.
- Никаких новых зависимостей.

## Parallelization

| Агент | Задачи | Зависимости |
|---|---|---|
| **A (backend)** | Task 1 | нет |
| **B (сайдбар)** | Task 2 | нет |
| **C (страница /trial)** | Task 3 | нет |
| **интеграция** | Task 4 | нужны 1 и 3 |

Задачи 1–3 не пересекаются по файлам. Task 4 трогает `CallRoom.tsx`, `whiteboard.ts` (api), обе страницы комнаты.

## File Structure

**Создать (backend):**
- `migrations/026_trial_board.sql` — nullable `course_id` + частичный уникальный индекс.

**Изменить (backend):**
- `repository/whiteboard.go` — `GetOrCreateTrialBoard` + `COALESCE` в трёх существующих запросах.
- `service/whiteboard.go` — `GetOrCreateTrialBoard`.
- `service/whiteboard_test.go` — тест на владение пробной доской.
- `handlers/whiteboard.go` — `GetTrialBoard`.
- `router/router.go` — роут `GET /boards/trial`.

**Создать (frontend):**
- `frontend/src/components/layout/nextLesson.ts` — `pickActiveLesson`.
- `frontend/src/components/layout/nextLesson.test.ts` — `node:test`.
- `frontend/src/app/(dashboard)/trial/page.tsx` — вкладка «Пробный урок».

**Изменить (frontend):**
- `frontend/src/components/layout/Sidebar.tsx` — кнопка урока, пункт NAV, диапазон `useCalendar`, тикер.
- `frontend/src/lib/api/whiteboard.ts` — `getTrialBoard`.
- `frontend/src/components/call/CallRoom.tsx` — проп `trial`.
- `frontend/src/components/call/CallToolbar.tsx` — проп `showHomework`.
- `frontend/src/app/(call)/room/[id]/page.tsx` — переход на `CallRoom` (роль `tutor`, `trial`).
- `frontend/src/app/join/room/[id]/page.tsx` — переход на `CallRoom` (роль `guest`).

---

## Контракты (общие для всех агентов)

### REST

```
GET /boards/trial → 200 BoardWithPages
```

Тело ответа — ровно то же, что у `GET /boards/course/:courseId`: `{ board: {...}, pages: [...] }`.
У пробной доски `board.course_id === ""`.

### Функция выбора урока

```ts
function pickActiveLesson(
  lessons: CalendarLesson[],
  now: Date,
  leadMinutes?: number,   // default 10
): CalendarLesson | null
```

---

## Task 1: Пробная доска на бэкенде (агент A)

**Files:**
- Create: `migrations/026_trial_board.sql`
- Modify: `repository/whiteboard.go`
- Modify: `service/whiteboard.go`
- Modify: `service/whiteboard_test.go`
- Modify: `handlers/whiteboard.go`
- Modify: `router/router.go`

**Interfaces:**
- Consumes: существующие `models.Board`, `models.BoardWithPages`, `service.ErrNotFound`, `handleServiceError`.
- Produces:
  - `repository.WhiteboardRepository.GetOrCreateTrialBoard(ctx context.Context, tutorID string) (models.Board, error)`
  - `service.WhiteboardService.GetOrCreateTrialBoard(ctx context.Context, tutorID string) (models.BoardWithPages, error)`
  - `handlers.WhiteboardHandler.GetTrialBoard(c *gin.Context)` на роуте `GET /boards/trial`.

- [ ] **Step 1: Миграция**

Создать `migrations/026_trial_board.sql`:

```sql
-- +goose Up
ALTER TABLE boards ALTER COLUMN course_id DROP NOT NULL;
CREATE UNIQUE INDEX idx_boards_trial ON boards (tutor_id) WHERE course_id IS NULL;

-- +goose Down
DROP INDEX idx_boards_trial;
DELETE FROM boards WHERE course_id IS NULL;
ALTER TABLE boards ALTER COLUMN course_id SET NOT NULL;
```

- [ ] **Step 2: Применить миграцию**

Run: `make migrate-up`
Expected: `OK   026_trial_board.sql`

- [ ] **Step 3: Падающий тест сервиса**

В `service/whiteboard_test.go` мок репозитория уже есть — дописать в него метод и добавить два теста.
Метод мока (добавить рядом с остальными методами мока репозитория; имя мок-структуры взять из файла — оно уже определено):

```go
func (m *mockWhiteboardRepo) GetOrCreateTrialBoard(ctx context.Context, tutorID string) (models.Board, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).(models.Board), args.Error(1)
}
```

Тесты:

```go
// Пробная доска препода отдаётся вместе с первой страницей.
func TestGetOrCreateTrialBoard_CreatesFirstPage(t *testing.T) {
	repo := new(mockWhiteboardRepo)
	board := models.Board{ID: "b1", CourseID: "", TutorID: "me"}
	repo.On("GetOrCreateTrialBoard", mock.Anything, "me").Return(board, nil)
	repo.On("GetPagesByBoard", mock.Anything, "b1").Return([]models.BoardPage{}, nil)
	repo.On("CreatePage", mock.Anything, "b1", "Страница 1", 0).
		Return(models.BoardPage{ID: "p1", BoardID: "b1", Title: "Страница 1"}, nil)

	svc := service.NewWhiteboardService(repo)
	got, err := svc.GetOrCreateTrialBoard(context.Background(), "me")

	assert.NoError(t, err)
	assert.Equal(t, "b1", got.Board.ID)
	assert.Len(t, got.Pages, 1)
}

// Доска, вернувшаяся с чужим tutor_id, наружу не уходит.
func TestGetOrCreateTrialBoard_ForeignBoard(t *testing.T) {
	repo := new(mockWhiteboardRepo)
	repo.On("GetOrCreateTrialBoard", mock.Anything, "me").
		Return(models.Board{ID: "b1", TutorID: "other"}, nil)

	svc := service.NewWhiteboardService(repo)
	_, err := svc.GetOrCreateTrialBoard(context.Background(), "me")

	assert.ErrorIs(t, err, service.ErrNotFound)
	repo.AssertNotCalled(t, "GetPagesByBoard", mock.Anything, mock.Anything)
}
```

- [ ] **Step 4: Убедиться, что тест падает**

Run: `go test ./service/ -run TestGetOrCreateTrialBoard`
Expected: FAIL — компиляция падает, `svc.GetOrCreateTrialBoard undefined`.

- [ ] **Step 5: Репозиторий**

В `repository/whiteboard.go`:

1. В интерфейс `WhiteboardRepository`, рядом с `GetOrCreateBoard`, добавить:

```go
	GetOrCreateTrialBoard(ctx context.Context, tutorID string) (models.Board, error)
```

2. В трёх существующих запросах (`GetBoardByID`, `GetOrCreateBoard`, `GetBoardByInvite`) заменить в списке колонок `course_id` на `COALESCE(course_id::text, '')` — иначе NULL у пробной доски не влезет в `models.Board.CourseID string`. Конкретно:

```go
func (r *whiteboardRepository) GetBoardByID(ctx context.Context, boardID string) (models.Board, error) {
	var b models.Board
	err := r.conn.QueryRow(ctx,
		`SELECT id, COALESCE(course_id::text, ''), tutor_id, created_at FROM boards WHERE id = $1`,
		boardID,
	).Scan(&b.ID, &b.CourseID, &b.TutorID, &b.CreatedAt)
	return b, err
}

func (r *whiteboardRepository) GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.Board, error) {
	var b models.Board
	err := r.conn.QueryRow(ctx,
		`INSERT INTO boards (course_id, tutor_id)
         VALUES ($1, $2)
         ON CONFLICT (course_id) DO UPDATE SET course_id = EXCLUDED.course_id
         RETURNING id, COALESCE(course_id::text, ''), tutor_id, created_at`,
		courseID, tutorID,
	).Scan(&b.ID, &b.CourseID, &b.TutorID, &b.CreatedAt)
	return b, err
}

func (r *whiteboardRepository) GetBoardByInvite(ctx context.Context, inviteID string) (models.Board, error) {
	var b models.Board
	err := r.conn.QueryRow(ctx,
		`SELECT bo.id, COALESCE(bo.course_id::text, ''), bo.tutor_id, bo.created_at
         FROM boards bo
         JOIN board_invites bi ON bi.board_id = bo.id
         WHERE bi.id = $1`,
		inviteID,
	).Scan(&b.ID, &b.CourseID, &b.TutorID, &b.CreatedAt)
	return b, err
}
```

3. Добавить реализацию рядом с `GetOrCreateBoard`:

```go
// GetOrCreateTrialBoard — доска для пробных уроков: одна на препода, вне курсов.
// ON CONFLICT целится в частичный индекс idx_boards_trial (tutor_id) WHERE
// course_id IS NULL — предикат в конфликте обязателен, иначе Postgres не поймёт,
// какой индекс имеется в виду.
func (r *whiteboardRepository) GetOrCreateTrialBoard(ctx context.Context, tutorID string) (models.Board, error) {
	var b models.Board
	err := r.conn.QueryRow(ctx,
		`INSERT INTO boards (course_id, tutor_id)
         VALUES (NULL, $1)
         ON CONFLICT (tutor_id) WHERE course_id IS NULL
         DO UPDATE SET tutor_id = EXCLUDED.tutor_id
         RETURNING id, COALESCE(course_id::text, ''), tutor_id, created_at`,
		tutorID,
	).Scan(&b.ID, &b.CourseID, &b.TutorID, &b.CreatedAt)
	return b, err
}
```

- [ ] **Step 6: Сервис**

В `service/whiteboard.go`:

1. В интерфейс `WhiteboardService`, рядом с `GetOrCreateBoard`, добавить:

```go
	GetOrCreateTrialBoard(ctx context.Context, tutorID string) (models.BoardWithPages, error)
```

2. Реализация — рядом с `GetOrCreateBoard`. Проверки курса нет: курса у пробной доски не существует, владение проверяем по `tutor_id` вернувшейся доски.

```go
func (s *whiteboardService) GetOrCreateTrialBoard(ctx context.Context, tutorID string) (models.BoardWithPages, error) {
	board, err := s.repo.GetOrCreateTrialBoard(ctx, tutorID)
	if err != nil {
		return models.BoardWithPages{}, err
	}
	// Upsert при конфликте не трогает tutor_id — на всякий случай убеждаемся,
	// что вернулась доска именно этого препода (та же проверка, что в GetOrCreateBoard).
	if board.TutorID != tutorID {
		return models.BoardWithPages{}, ErrNotFound
	}
	pages, err := s.repo.GetPagesByBoard(ctx, board.ID)
	if err != nil {
		return models.BoardWithPages{}, err
	}
	if len(pages) == 0 {
		first, err := s.repo.CreatePage(ctx, board.ID, "Страница 1", 0)
		if err != nil {
			return models.BoardWithPages{}, err
		}
		pages = []models.BoardPage{first}
	}
	return models.BoardWithPages{Board: board, Pages: pages}, nil
}
```

- [ ] **Step 7: Тесты зелёные**

Run: `go test ./service/ -run TestGetOrCreateTrialBoard -v`
Expected: PASS, оба теста.

- [ ] **Step 8: Хендлер**

В `handlers/whiteboard.go`, рядом с `GetBoardByCourse`:

```go
// GET /boards/trial — общая доска всех пробных уроков препода.
func (h *WhiteboardHandler) GetTrialBoard(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	result, err := h.svc.GetOrCreateTrialBoard(c.Request.Context(), tutorID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
```

- [ ] **Step 9: Роут**

В `router/router.go`, в блоке «Whiteboard protected routes» (рядом с `auth.GET("/boards/course/:courseId", ...)`), **выше** него, добавить:

```go
		auth.GET("/boards/trial", whiteboardHandler.GetTrialBoard)
```

Порядок важен: у Gin статический сегмент и параметр в одной позиции не конфликтуют только при разных путях, а `/boards/trial` и `/boards/:boardId/pages` — разные шаблоны, поэтому конфликта нет; но ставим статический роут первым, чтобы не думать об этом при следующих правках.

- [ ] **Step 10: Сборка и тесты**

Run: `go build ./... && go test ./...`
Expected: сборка без вывода, все тесты PASS.

- [ ] **Step 11: Commit**

```bash
git add migrations/026_trial_board.sql repository/whiteboard.go service/whiteboard.go \
        service/whiteboard_test.go handlers/whiteboard.go router/router.go
git commit -m "feat(trial): shared trial board per tutor"
```

---

## Task 2: Кнопка следующего урока в сайдбаре (агент B)

**Files:**
- Create: `frontend/src/components/layout/nextLesson.ts`
- Create: `frontend/src/components/layout/nextLesson.test.ts`
- Modify: `frontend/src/components/layout/Sidebar.tsx`

**Interfaces:**
- Consumes: `CalendarLesson` из `@/types/api` (поля `id`, `scheduled_at`, `duration_minutes`, `status`, `subject`).
- Produces: `pickActiveLesson(lessons: CalendarLesson[], now: Date, leadMinutes?: number): CalendarLesson | null`.

- [ ] **Step 1: Падающий тест**

Создать `frontend/src/components/layout/nextLesson.test.ts` (стиль — как `src/components/whiteboard/mediaSync.test.ts`: `node:test` + `node:assert/strict`, импорт с расширением `.ts`):

```ts
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { pickActiveLesson } from './nextLesson.ts'
import type { CalendarLesson } from '@/types/api'

const NOW = new Date('2026-07-14T10:00:00Z')

function lesson(over: Partial<CalendarLesson>): CalendarLesson {
  return {
    id: 'l1',
    course_id: 'c1',
    scheduled_at: '2026-07-14T10:05:00Z',
    duration_minutes: 60,
    status: 'scheduled',
    notes: '',
    subject: 'Английский',
    student_name: 'Аня',
    is_group: false,
    ...over,
  }
}

test('урок через 5 минут — попадает в окно', () => {
  const l = lesson({})
  assert.equal(pickActiveLesson([l], NOW)?.id, 'l1')
})

test('урок через 11 минут — окна нет', () => {
  const l = lesson({ scheduled_at: '2026-07-14T10:11:00Z' })
  assert.equal(pickActiveLesson([l], NOW), null)
})

test('ровно −10 минут — граница включительно', () => {
  const l = lesson({ scheduled_at: '2026-07-14T10:10:00Z' })
  assert.equal(pickActiveLesson([l], NOW)?.id, 'l1')
})

test('урок уже идёт — всё ещё в окне', () => {
  const l = lesson({ scheduled_at: '2026-07-14T09:30:00Z', duration_minutes: 60 })
  assert.equal(pickActiveLesson([l], NOW)?.id, 'l1')
})

test('урок кончился минуту назад — окна нет', () => {
  const l = lesson({ scheduled_at: '2026-07-14T08:59:00Z', duration_minutes: 60 })
  assert.equal(pickActiveLesson([l], NOW), null)
})

test('не-scheduled игнорируются', () => {
  const cancelled = lesson({ status: 'cancelled' })
  const completed = lesson({ id: 'l2', status: 'completed' })
  assert.equal(pickActiveLesson([cancelled, completed], NOW), null)
})

test('из двух подходящих берётся ближайший по времени', () => {
  const later  = lesson({ id: 'late',  scheduled_at: '2026-07-14T10:09:00Z' })
  const sooner = lesson({ id: 'soon',  scheduled_at: '2026-07-14T10:02:00Z' })
  assert.equal(pickActiveLesson([later, sooner], NOW)?.id, 'soon')
})

test('пустой список — null', () => {
  assert.equal(pickActiveLesson([], NOW), null)
})
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `cd frontend && node --test --experimental-strip-types src/components/layout/nextLesson.test.ts`
Expected: FAIL — не может разрешить `./nextLesson.ts`.

Если запуск ругается на алиас `@/types/api` — посмотреть, как это решено в `src/components/whiteboard/mediaSync.test.ts`, и сделать так же (там тип импортируется из общего файла или объявлен локально; повторить приём, а не изобретать новый).

- [ ] **Step 3: Реализация**

Создать `frontend/src/components/layout/nextLesson.ts`:

```ts
import type { CalendarLesson } from '@/types/api'

/** За сколько минут до старта показываем кнопку. */
export const LEAD_MINUTES = 10

/**
 * Урок, который препод может начать прямо сейчас: ближайший запланированный,
 * чьё окно [старт − leadMinutes, старт + длительность] накрывает now.
 * Окно тянется до конца урока — препод мог опоздать или перезагрузить вкладку.
 */
export function pickActiveLesson(
  lessons: CalendarLesson[],
  now: Date,
  leadMinutes: number = LEAD_MINUTES,
): CalendarLesson | null {
  const t = now.getTime()

  const inWindow = lessons.filter((l) => {
    if (l.status !== 'scheduled') return false
    const start = new Date(l.scheduled_at).getTime()
    const from  = start - leadMinutes * 60_000
    const to    = start + l.duration_minutes * 60_000
    return t >= from && t <= to
  })

  if (inWindow.length === 0) return null

  return inWindow.reduce((best, l) =>
    new Date(l.scheduled_at).getTime() < new Date(best.scheduled_at).getTime() ? l : best
  )
}
```

- [ ] **Step 4: Тесты зелёные**

Run: `cd frontend && node --test --experimental-strip-types src/components/layout/nextLesson.test.ts`
Expected: PASS, 8 тестов.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/layout/nextLesson.ts frontend/src/components/layout/nextLesson.test.ts
git commit -m "feat(sidebar): pickActiveLesson window logic"
```

- [ ] **Step 6: Сайдбар — данные и тикер**

В `frontend/src/components/layout/Sidebar.tsx`, в компоненте `CalendarSidebarPanel`:

1. Импорты (добавить к существующим):

```tsx
import { pickActiveLesson } from './nextLesson'
```

2. Диапазон `useCalendar` расширить на сутки вперёд — иначе в 23:55 урок на 00:05 не попадёт в выборку и кнопка не появится. `TodayList` фильтрует по `isSameLocalDay`, его вид от этого не меняется:

```tsx
  const todayRange = useMemo(() => {
    const n     = new Date()
    const start = new Date(n.getFullYear(), n.getMonth(), n.getDate())
    const end   = new Date(start)
    // +2 суток: список «Сегодня» фильтрует сам, а кнопке нужен урок,
    // который может начаться уже после полуночи.
    end.setDate(end.getDate() + 2)
    return { from: start.toISOString(), to: end.toISOString() }
  }, [])
```

3. Тикер: `now` в state, обновляется раз в 30 секунд, иначе кнопка не появится сама на открытой вкладке.

```tsx
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 30_000)
    return () => clearInterval(t)
  }, [])

  const activeLesson = useMemo(() => pickActiveLesson(todayLessons, now), [todayLessons, now])
  const started = activeLesson
    ? now.getTime() >= new Date(activeLesson.scheduled_at).getTime()
    : false
```

- [ ] **Step 7: Сайдбар — кнопка**

В том же `CalendarSidebarPanel` заменить `handleStartLesson` (он открывал безымянную комнату) на переход к уроку:

```tsx
  function handleStartLesson() {
    if (!activeLesson || starting) return
    setStarting(true)
    router.push(`/lessons/${activeLesson.id}/call`)
  }
```

Импорт `callsApi` из файла удалить, если он больше нигде в файле не используется (проверить `grep callsApi Sidebar.tsx`).

Блок с `<style>{...liveDot...}</style>` и кнопкой заменить на:

```tsx
      {activeLesson && (
        <>
          <style>{`
            @keyframes liveDot {
              0%,100% { opacity:1; box-shadow: 0 0 0 0 rgba(34,197,94,0.5); }
              50% { opacity:.75; box-shadow: 0 0 0 4px rgba(34,197,94,0); }
            }
          `}</style>
          <div className="shrink-0 rounded-[12px] border border-border bg-card p-2.5">
            <div className="text-[12px] font-semibold text-foreground truncate leading-tight">
              {activeLesson.subject}
            </div>
            <div className="text-[11px] text-[var(--sidebar-text)] tabular-nums mb-2">
              {formatTime(activeLesson.scheduled_at)}
            </div>
            <button
              onClick={handleStartLesson}
              disabled={starting}
              className="w-full flex items-center justify-center gap-2 px-3 py-2 rounded-[10px] transition-colors hover:brightness-110 disabled:opacity-50 disabled:cursor-not-allowed"
              style={{ background: 'var(--secondary)' }}
            >
              <span
                style={{
                  width: 7, height: 7, borderRadius: '50%', flexShrink: 0,
                  background: starting ? '#888' : '#2D9964',
                  animation: starting ? 'none' : 'liveDot 2s ease-in-out infinite',
                }}
              />
              <span style={{ fontSize: 13, color: 'var(--foreground)', fontWeight: 600 }}>
                {starting ? 'Подключение...' : started ? 'Присоединиться' : 'Начать урок'}
              </span>
            </button>
          </div>
        </>
      )}
```

- [ ] **Step 8: Сайдбар — пункт «Пробный урок»**

В том же файле, в массив `NAV` добавить пункт (иконка — `Video` из `lucide-react`, добавить в импорт иконок):

```tsx
const NAV = [
  { href: '/dashboard',  label: 'Главная',      icon: LayoutDashboard },
  { href: '/calendar',   label: 'Расписание',   icon: CalendarDays },
  { href: '/courses',    label: 'Курсы',        icon: BookOpen },
  { href: '/trial',      label: 'Пробный урок', icon: Video },
  { href: '/payments',   label: 'Платежи',      icon: CreditCard },
  { href: '/profile',    label: 'Профиль',      icon: User },
]
```

- [ ] **Step 9: Типы и сборка**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок. (Страница `/trial` появится в Task 3 — на типы это не влияет, ссылка `href` типизирована как строка.)

- [ ] **Step 10: Commit**

```bash
git add frontend/src/components/layout/Sidebar.tsx
git commit -m "feat(sidebar): start next scheduled lesson instead of quick room"
```

---

## Task 3: Страница «Пробный урок» (агент C)

**Files:**
- Create: `frontend/src/app/(dashboard)/trial/page.tsx`

**Interfaces:**
- Consumes: `callsApi.startQuickRoom()` из `@/lib/api/calls` (возвращает `{ room_id, token, server_url }`), `SectionCard` из `@/components/common/SectionCard`.
- Produces: страница по маршруту `/trial`.

- [ ] **Step 1: Посмотреть, как устроены соседние страницы**

Открыть `frontend/src/app/(dashboard)/courses/page.tsx` и посмотреть: как импортируется `SectionCard`, как оформлен заголовок страницы, какие отступы у корневого контейнера. Повторить тот же каркас — новых визуальных паттернов не изобретать.

- [ ] **Step 2: Страница**

Создать `frontend/src/app/(dashboard)/trial/page.tsx`:

```tsx
'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { Video } from 'lucide-react'

import { callsApi } from '@/lib/api/calls'
import { SectionCard } from '@/components/common/SectionCard'

export default function TrialPage() {
  const router = useRouter()
  const [starting, setStarting] = useState(false)

  async function handleStart() {
    if (starting) return
    setStarting(true)
    try {
      const { room_id, token, server_url } = await callsApi.startQuickRoom()
      sessionStorage.setItem(`quick-room-${room_id}`, JSON.stringify({ token, server_url }))
      router.push(`/room/${room_id}`)
    } catch {
      toast.error('Не удалось создать комнату')
      setStarting(false)
    }
  }

  return (
    <div className="space-y-4">
      <h1 className="font-heading text-xl font-bold tracking-tight">Пробный урок</h1>

      <SectionCard>
        <div className="flex flex-col items-start gap-3 p-1">
          <p className="text-sm text-muted-foreground max-w-prose">
            Пробный урок не привязан к курсу и не попадает в расписание — подойдёт для знакомства
            с учеником, которого ещё нет в базе. Ссылку для ученика можно скопировать внутри комнаты.
            Доска у всех пробных уроков общая: то, что вы на ней нарисовали, останется к следующему разу.
          </p>

          <button
            onClick={handleStart}
            disabled={starting}
            className="flex items-center gap-2 px-4 py-2.5 rounded-[10px] text-sm font-semibold text-primary-foreground bg-primary transition-colors hover:brightness-110 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Video className="h-4 w-4" />
            {starting ? 'Создаём комнату...' : 'Начать пробный урок'}
          </button>
        </div>
      </SectionCard>
    </div>
  )
}
```

- [ ] **Step 3: Типы и сборка**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок.

Если `SectionCard` требует пропсы (например, `title`) — открыть `frontend/src/components/common/SectionCard.tsx` и передать их так, как это делают `courses/page.tsx` и `payments/page.tsx`.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/\(dashboard\)/trial/page.tsx
git commit -m "feat(trial): trial lesson page"
```

---

## Task 4: Доска в пробной комнате (после 1 и 3)

**Files:**
- Modify: `frontend/src/lib/api/whiteboard.ts`
- Modify: `frontend/src/components/call/CallRoom.tsx`
- Modify: `frontend/src/components/call/CallToolbar.tsx`
- Modify: `frontend/src/app/(call)/room/[id]/page.tsx`
- Modify: `frontend/src/app/join/room/[id]/page.tsx`

**Interfaces:**
- Consumes: `GET /boards/trial` (Task 1), существующий `CallRoom` (`CallRoomProps`: `courseId?`, `serverUrl`, `token`, `role`, `enableMedia?`, `inviteUrl?`, `onDisconnected`).
- Produces: `whiteboardApi.getTrialBoard(): Promise<BoardWithPages>`; `CallRoomProps.trial?: boolean`; `CallToolbar` props: `showHomework?: boolean` (default `true`).

- [ ] **Step 1: API-клиент**

В `frontend/src/lib/api/whiteboard.ts`, рядом с `getBoardByCourse`:

```ts
  getTrialBoard: () =>
    api.get<BoardWithPages>('/boards/trial').then((r) => r.data),
```

- [ ] **Step 2: CallRoom — проп trial**

В `frontend/src/components/call/CallRoom.tsx`:

1. `CallRoomInnerProps` и `CallRoomProps` получают поле:

```ts
  /** Пробный урок: доска берётся из общей trial-доски препода, курса нет. */
  trial?: boolean
```

2. `CallRoom` прокидывает его внутрь: `<CallRoomInner courseId={courseId} role={role} inviteUrl={inviteUrl} trial={trial} />`, а `CallRoomInner` принимает `{ courseId, role, inviteUrl, trial }`.

3. Внутри `CallRoomInner` завести один признак «доска доступна» и заменить им гейты по `courseId`:

```tsx
  // Раньше courseId играл роль флага «доска есть». С пробной доской источников два.
  const hasBoard = Boolean(courseId) || Boolean(trial)
```

4. В `handleToggle` заменить начало:

```tsx
    if (!hasBoard) {
      toast.error('Доска недоступна: урок не привязан к курсу')
      return
    }
```

и строку получения доски:

```tsx
      const board = tutorBoard ?? (trial
        ? await whiteboardApi.getTrialBoard()
        : await whiteboardApi.getBoardByCourse(courseId as string))
```

В массив зависимостей `useCallback` добавить `trial`: `[courseId, trial, mode, tutorBoard, room]`.

5. В эффекте автооткрытия доски заменить условие и зависимости:

```tsx
  useEffect(() => {
    if (role !== 'tutor' || !hasBoard) return
    if (connectionState !== ConnectionState.Connected) return
    if (autoOpenedRef.current) return
    autoOpenedRef.current = true
    handleToggle()
  }, [role, hasBoard, connectionState, handleToggle])
```

6. Домашка у пробного урока отсутствует по определению — гость не должен получать пустой диалог, а препод кнопку, которая ничего не делает. Передать в тулбар:

```tsx
      <CallToolbar
        role={role}
        inviteUrl={inviteUrl}
        boardActive={mode === 'board'}
        chatActive={chatOpen}
        chatUnread={unread}
        showHomework={!trial}
        onToggleBoard={handleToggle}
        onToggleChat={() => setChatOpen((v) => !v)}
        onHomework={() => setHomeworkOpen(true)}
        onLeave={() => room.disconnect()}
      />
```

и обернуть гостевой диалог:

```tsx
      {role === 'guest' && !trial && (
        <HomeworkViewDialog open={homeworkOpen} onClose={() => setHomeworkOpen(false)} />
      )}
```

(Блок препода уже под гейтом `role === 'tutor' && courseId` — у пробного урока `courseId` пуст, так что он и так не отрендерится.)

- [ ] **Step 3: CallToolbar — showHomework**

В `frontend/src/components/call/CallToolbar.tsx`:

1. В `interface Props` добавить:

```ts
  /** Домашка есть только у курсового урока. */
  showHomework?: boolean
```

2. В деструктуризации пропсов задать дефолт: `showHomework = true`.

3. Кнопку домашки (та, что с `title="Домашнее задание"` и `onClick={() => { setMoreOpen(false); onHomework() }}`) обернуть:

```tsx
        {showHomework && (
          <button onClick={() => { setMoreOpen(false); onHomework() }} title="Домашнее задание" style={btnBase({})}>
            {/* содержимое кнопки оставить как есть */}
          </button>
        )}
```

- [ ] **Step 4: Комната препода на CallRoom**

`frontend/src/app/(call)/room/[id]/page.tsx` сейчас рендерит `<LiveKitRoom><VideoConference /></LiveKitRoom>` и свою кнопку «Ссылка для ученика». Заменить всё это на `CallRoom` — он даёт доску, чат, тулбар и ту же кнопку приглашения (`inviteUrl`). Итоговый файл:

```tsx
'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import '@livekit/components-styles'
import { Loader2 } from 'lucide-react'

import { callsApi } from '@/lib/api/calls'
import { CallRoom } from '@/components/call/CallRoom'

type Stage = 'loading' | 'in-room' | 'error'

export default function QuickRoomPage() {
  const { id }  = useParams<{ id: string }>()
  const router  = useRouter()

  const [stage, setStage]         = useState<Stage>('loading')
  const [token, setToken]         = useState<string | null>(null)
  const [serverUrl, setServerUrl] = useState<string | null>(null)

  useEffect(() => {
    // LiveKit bug: placeholder→real track transition triggers a spurious console.error
    const orig = console.error.bind(console)
    console.error = (...args: unknown[]) => {
      if (typeof args[0] === 'string' && args[0].includes('Element not part of the array')) return
      orig(...args)
    }
    return () => { console.error = orig }
  }, [])

  useEffect(() => {
    const raw = sessionStorage.getItem(`quick-room-${id}`)
    if (!raw) { router.replace('/trial'); return }
    try {
      const { token: t, server_url: s } = JSON.parse(raw) as { token: string; server_url: string }
      setToken(t)
      setServerUrl(s)
      setStage('in-room')
    } catch {
      router.replace('/trial')
    }
  }, [id, router])

  async function handleDisconnected() {
    try { await callsApi.endQuickRoom(id) } catch {}
    sessionStorage.removeItem(`quick-room-${id}`)
    router.replace('/trial')
  }

  if (stage === 'loading') {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100dvh' }}>
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (stage === 'error' || !token || !serverUrl) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100dvh', gap: 16 }}>
        <p className="text-muted-foreground text-sm">Не удалось подключиться к комнате</p>
        <button
          onClick={() => router.replace('/trial')}
          style={{ padding: '8px 16px', borderRadius: 8, border: '1px solid var(--border)', cursor: 'pointer', fontSize: 13 }}
        >
          К пробным урокам
        </button>
      </div>
    )
  }

  const inviteUrl = `${window.location.origin}/join/room/${id}`

  return (
    <div style={{ height: '100dvh', position: 'relative' }}>
      <CallRoom
        trial
        serverUrl={serverUrl}
        token={token}
        role="tutor"
        inviteUrl={inviteUrl}
        onDisconnected={handleDisconnected}
      />
    </div>
  )
}
```

- [ ] **Step 5: Гостевая комната на CallRoom**

В `frontend/src/app/join/room/[id]/page.tsx` заменить только ветку `stage === 'in-room'` (форма ввода имени, опрос статуса и ветка «Урок завершён» остаются без изменений):

```tsx
  if (stage === 'in-room' && room) {
    return (
      <div style={{ height: '100dvh' }}>
        <CallRoom
          trial
          serverUrl={room.server_url}
          token={room.token}
          role="guest"
          enableMedia
          onDisconnected={handleDisconnected}
        />
      </div>
    )
  }
```

Импорты: убрать `LiveKitRoom, VideoConference` из `@livekit/components-react` (если они больше нигде в файле не нужны), добавить `import { CallRoom } from '@/components/call/CallRoom'`.

- [ ] **Step 6: Типы и сборка**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: без ошибок.

- [ ] **Step 7: Ручной прогон (обязателен — автотестов на этот путь нет)**

Поднять бэкенд (`make run`) и фронт:

1. Сайдбар без ближайшего урока — кнопки внизу нет, в навигации есть «Пробный урок».
2. Создать урок на 5 минут вперёд → в сайдбаре появилась карточка с названием курса и кнопкой «Начать урок» (не позже чем через 30 секунд — тикер). Клик → открылась комната урока.
3. Урок, начавшийся 5 минут назад → надпись «Присоединиться».
4. Урок, кончившийся час назад → кнопки нет.
5. `/trial` → «Начать пробный урок» → комната открылась, доска автоматически развернулась (у препода она открывается сразу после подключения).
6. Скопировать «Ссылка для ученика», открыть в другом окне (инкогнито), ввести имя → гость попал в комнату и видит ту же доску.
7. Нарисовать что-то, выйти, начать новый пробный урок → рисунок на месте (доска общая).
8. В тулбаре пробного урока нет кнопки «Домашнее задание».
9. Курсовой урок → доска и домашка работают как раньше (регрессия по `courseId`).

- [ ] **Step 8: Commit**

```bash
git add frontend/src/lib/api/whiteboard.ts frontend/src/components/call/CallRoom.tsx \
        frontend/src/components/call/CallToolbar.tsx \
        frontend/src/app/\(call\)/room/\[id\]/page.tsx \
        frontend/src/app/join/room/\[id\]/page.tsx
git commit -m "feat(trial): shared board in trial rooms"
```

---

## Self-Review (выполнено при написании плана)

- **Покрытие спеки:** окно выбора урока и `pickActiveLesson` → Task 2; расширение диапазона `useCalendar`, тикер, три состояния слота, пункт NAV → Task 2; страница `/trial` → Task 3; миграция, частичный индекс, `GetOrCreateTrialBoard`, роут → Task 1; проп `trial` в `CallRoom` и доска в пробных комнатах → Task 4. Раздел спеки «что осталось за бортом» задач не порождает.
- **Расхождение со спекой (осознанное):** спека говорила «`/room/[id]` передаёт `trial` для роли препода», умалчивая о гостевой странице. По факту обе страницы быстрой комнаты рендерят голый `<VideoConference />`, поэтому в Task 4 на `CallRoom` переводятся обе — иначе гость доску не увидит и фича не работает.
- **Согласованность имён:** `pickActiveLesson`, `GetOrCreateTrialBoard`, `getTrialBoard`, `trial`, `hasBoard`, `showHomework` употребляются одинаково во всех задачах.
