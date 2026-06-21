# Tasks Widget — Design Spec
**Date:** 2026-06-21

## Goal

Add a tasks widget to the dashboard with full CRUD: create, edit (via Sheet), toggle done, delete. No separate `/tasks` page — the widget is the primary UI.

## Scope

- One new component: `components/tasks/TasksWidget.tsx`
- Minor change to `app/(dashboard)/dashboard/page.tsx` to include the widget
- Backend is complete — no changes needed

## Data

The existing `useTasks(from, to)` hook is called with a wide fixed range (`2020-01-01T00:00:00Z` → `2035-01-01T00:00:00Z`). Completed tasks are filtered out client-side (`task.done === false`). This avoids any backend change.

Hooks used (all exist in `lib/hooks/useTasks.ts`):
- `useTasks` — fetch
- `useCreateTask` — create
- `useRescheduleTask` — update (title + scheduled_at + duration_minutes)
- `useToggleTask` — toggle done
- `useDeleteTask` — delete

## Layout

The widget sits **below** the existing 3-column row on the dashboard, occupying the first column of a new row (width 1/3, aligned with existing widgets). This avoids breaking the current 3-column grid.

## Widget Structure

```
┌─ Задачи ─────────────────── [+] ─┐
│ ☐  Подготовить тест          ✎  │
│ ☐  Купить маркеры            ✎  │
│ ☐  Позвонить родителям       ✎  │
└───────────────────────────────────┘
```

- Header matches existing widget style (`WIDGET_HEAD`, `WIDGET_TITLE`)
- **[+] button** — opens Sheet in "create" mode (empty form)
- **Checkbox** — calls `useToggleTask`; on success the task disappears from the list instantly (filtered to `done=false`)
- **✎ icon** — appears on row hover, opens Sheet in "edit" mode with prefilled fields
- Empty state: "Задач нет" (matches existing empty state style)
- No pagination — all incomplete tasks shown

## Sheet

Single Sheet component inside `TasksWidget`, shared for create and edit. Controlled by local state `{ open: boolean, task: Task | null }` — `task === null` means create mode.

Fields:
| Field | Input type | Validation |
|---|---|---|
| Название | `<Input>` | required, max 200 |
| Дата и время | `<input type="datetime-local">` | required |
| Длительность (мин) | `<Input type="number">` | required, > 0 |

Buttons:
- **Сохранить** — `useCreateTask` or `useRescheduleTask` depending on mode
- **Удалить** — `useDeleteTask`, only shown in edit mode
- **Отмена / X** — closes Sheet

## Styling

Follows existing dashboard conventions — inline `style` objects, CSS variables (`var(--border)`, `var(--muted-foreground)`, etc.), no new CSS files.

## Out of Scope

- Completed tasks history
- Filtering / sorting
- Separate `/tasks` page
- Due-date highlighting or overdue indicators
