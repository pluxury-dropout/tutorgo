# Канбан-задачи без времени

## Цель
Создавать задачи для мини-канбан-доски без обязательного времени и длительности.
Канбан и календарь — один пул задач; время необязательное.

## Решение
- Задача без `scheduled_at` = только канбан, в календарь не попадает.
- Задача со временем = и в канбане, и в календаре (как сейчас).
- Канбан-UI редактирует только `title`/`status`; существующие `scheduled_at`/`duration_minutes` прокидываются без изменений (не затирая календарные задачи).

## Изменения

### Backend
1. **Миграция 017**: `scheduled_at`, `duration_minutes` → `DROP NOT NULL`.
2. **models/task.go**: `ScheduledAt *time.Time`, `DurationMinutes *int`; в Create/Update-реквестах `omitempty` вместо `required`.
3. **repository/task.go**: указатели в Scan/INSERT/UPDATE (pgx: `nil` → `NULL`); новый `GetAll(tutorID)` — все задачи тьютора. `GetByRange` без изменений (NULL `scheduled_at` отсеивается сравнением `>=`).
4. **service + handler + router**: `GET /tasks/board` → `GetAll`.

### Frontend
5. **KanbanWidget.tsx**: создание = `title` + `status`; убрать поля времени/длительности; при save/drag прокидывать существующие `scheduled_at`/`duration_minutes`.
6. **useTasks.ts**: хук `useBoardTasks` → `/tasks/board`.
7. Календарь и `TaskCreateDialog` — без изменений.

## Вне рамок (YAGNI)
Отдельная таблица; drag канбан→календарь; сортировка внутри колонок; тесты на тривиальный CRUD (один тест: Create без времени).
