# Kanban Widget — Design Spec
**Date:** 2026-06-21

## Goal

Replace the task list widget on the dashboard with a full-width Kanban board. Tasks have a priority-based status (`not_urgent | urgent | very_urgent | done`) instead of a boolean `done` field. Cards are draggable between columns. Each card has a colored left-border accent matching its column.

## Columns

| Status | Label | Accent color |
|--------|-------|-------------|
| `not_urgent` | Несрочно | green (`var(--success)`) |
| `urgent` | Срочно | yellow/warning (`var(--warning)`) |
| `very_urgent` | Очень срочно | red (`var(--destructive)`) |
| `done` | Выполнено | muted (`var(--muted-foreground)`) |

## Backend Changes

### Migration `016_kanban_status.sql`
```sql
-- +goose Up
ALTER TABLE tasks ADD COLUMN status TEXT NOT NULL DEFAULT 'not_urgent';
UPDATE tasks SET status = 'done' WHERE done = true;
ALTER TABLE tasks DROP COLUMN done;

-- +goose Down
ALTER TABLE tasks ADD COLUMN done BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE tasks SET done = true WHERE status = 'done';
ALTER TABLE tasks DROP COLUMN status;
```

### `models/task.go`
- Replace `Done bool json:"done"` → `Status string json:"status"`
- `CreateTaskRequest`: add `Status string json:"status" validate:"omitempty,oneof=not_urgent urgent very_urgent done"` (defaults to `not_urgent` if empty)
- `UpdateTaskRequest`: replace `Done bool` with `Status string validate:"required,oneof=not_urgent urgent very_urgent done"`

### `repository/task.go`
- `GetByRange`: no change (still filters by `scheduled_at` range)
- `Create`: insert `status` instead of `done`
- `Update`: update `status` instead of `done`
- `ToggleDone` method: **delete** — no longer needed

### `service/task.go`
- `ToggleDone` method: **delete**
- All other methods: update field references `done` → `status`

### `handlers/task.go`
- `ToggleDone` method: **delete**

### `router/router.go`
- Remove: `auth.PATCH("/tasks/:id/done", taskHandler.ToggleDone)`

## Frontend Changes

### `frontend/src/types/api.ts`
Replace:
```ts
done: boolean
```
With:
```ts
status: 'not_urgent' | 'urgent' | 'very_urgent' | 'done'
```

### `frontend/src/lib/api/tasks.ts`
- `TaskInput`: add optional `status?: string` (defaults handled by backend)
- `TaskUpdateInput`: replace `done: boolean` → `status: string`
- Remove `toggleDone` method from `tasksApi`

### `frontend/src/lib/hooks/useTasks.ts`
- Remove `useToggleTask` — no longer needed
- `useRescheduleTask` already handles full task updates including `status` — reuse it for drag & drop column moves

### `frontend/src/app/(dashboard)/calendar/page.tsx`
- Replace `useToggleTask` import with `useRescheduleTask`
- On task click (currently `toggleTask.mutate(arg.event.id)`): find the task object from the loaded tasks array by id, call:
```ts
rescheduleTask.mutate({ id: task.id, data: { title: task.title, scheduled_at: task.scheduled_at, duration_minutes: task.duration_minutes, status: task.status === 'done' ? 'not_urgent' : 'done' } })
```
This toggles between `done` and `not_urgent` — same UX as before.

### New: `frontend/src/components/tasks/KanbanWidget.tsx`
- Replaces `TasksWidget.tsx` (delete old file)
- Install: `npm install @dnd-kit/core @dnd-kit/utilities`
- `DndContext` wraps all 4 columns
- Each column: `useDroppable({ id: status })`
- Each card: `useDraggable({ id: task.id })`
- `onDragEnd`: call `useRescheduleTask` with `{ id: task.id, data: { ...task, status: overId } }`
- Cards filtered per column by `task.status`
- Card style: background matching column, left `3px solid` accent border
- `[+]` button → Sheet (create mode, status dropdown defaults to `not_urgent`)
- Click on card → Sheet (edit mode, status dropdown shows current status)
- Sheet fields: Название, Статус (select), Дата и время, Длительность
- No checkbox — "done" is achieved by dragging to the Выполнено column

### `frontend/src/app/(dashboard)/dashboard/page.tsx`
- Replace `<TasksWidget />` import with `<KanbanWidget />`
- Wrap in `<div style={{ maxWidth: 900 }}>` — matches the header's max-width constraint, not full browser width

## Out of Scope

- Sorting within a column
- Custom column names
- Mobile drag & drop (touch events — dnd-kit handles basic touch, but no special mobile treatment)
- Filtering by date or search
