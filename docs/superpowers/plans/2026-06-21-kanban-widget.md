# Kanban Widget Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the tasks list widget on the dashboard with a full-width Kanban board (4 columns: Несрочно / Срочно / Очень срочно / Выполнено) with drag-and-drop between columns.

**Architecture:** Backend replaces `done BOOLEAN` with `status TEXT` on the `tasks` table; all Go layers (model/repo/service/handler) updated accordingly, `PATCH /tasks/:id/done` endpoint removed. Frontend installs `@dnd-kit/core`, creates `KanbanWidget.tsx` (full-width, 4 draggable columns with colored left-border cards), and updates the calendar page to use `useRescheduleTask` for toggling task done state.

**Tech Stack:** Go + pgx, Next.js 14, TanStack Query, @dnd-kit/core + @dnd-kit/utilities, shadcn/ui Sheet, sonner (toast).

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `migrations/016_kanban_status.sql` | Add `status` column, migrate data, drop `done` |
| Modify | `models/task.go` | Replace `Done bool` → `Status string` |
| Modify | `repository/task.go` | Remove `ToggleDone`, update SQL to use `status` |
| Modify | `service/task.go` | Remove `ToggleDone`, default status to `not_urgent` |
| Modify | `handlers/task.go` | Remove `ToggleDone` handler |
| Modify | `router/router.go` | Remove `PATCH /tasks/:id/done` route |
| Create | `service/task_test.go` | Test status default behavior |
| Modify | `frontend/src/types/api.ts` | `done: boolean` → `status: string` |
| Modify | `frontend/src/lib/api/tasks.ts` | Update `TaskUpdateInput`, remove `toggleDone` |
| Modify | `frontend/src/lib/hooks/useTasks.ts` | Remove `useToggleTask` |
| Delete | `frontend/src/components/tasks/TasksWidget.tsx` | Replaced by KanbanWidget |
| Modify | `frontend/src/app/(dashboard)/dashboard/page.tsx` | Remove TasksWidget, later add KanbanWidget |
| Modify | `frontend/src/app/(dashboard)/calendar/page.tsx` | `done` → `status`, toggle via reschedule |
| Create | `frontend/src/components/tasks/KanbanWidget.tsx` | Full-width Kanban board |

---

### Task 1: Migration — add `status`, drop `done`

**Files:**
- Create: `migrations/016_kanban_status.sql`

- [ ] **Step 1: Create migration file**

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

- [ ] **Step 2: Apply migration**

```bash
goose -dir migrations postgres "$DB_URL" up
```

Expected output: `OK    016_kanban_status.sql`

- [ ] **Step 3: Verify schema**

```bash
psql "$DB_URL" -c "\d tasks"
```

Expected: `status text not null` column present, `done` column absent.

- [ ] **Step 4: Commit**

```bash
git add migrations/016_kanban_status.sql
git commit -m "feat: migrate tasks done→status"
```

---

### Task 2: Backend Go — model, repo, service, handler, router

**Files:**
- Modify: `models/task.go`
- Modify: `repository/task.go`
- Modify: `service/task.go`
- Modify: `handlers/task.go`
- Modify: `router/router.go`

- [ ] **Step 1: Replace `models/task.go`**

```go
package models

import "time"

type Task struct {
	ID              string    `json:"id"`
	TutorID         string    `json:"tutor_id"`
	Title           string    `json:"title"`
	Status          string    `json:"status"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	DurationMinutes int       `json:"duration_minutes"`
	CreatedAt       time.Time `json:"created_at"`
}

type CreateTaskRequest struct {
	Title           string    `json:"title"            validate:"required,max=200"`
	ScheduledAt     time.Time `json:"scheduled_at"     validate:"required"`
	DurationMinutes int       `json:"duration_minutes" validate:"required,gt=0"`
	Status          string    `json:"status"           validate:"omitempty,oneof=not_urgent urgent very_urgent done"`
}

type UpdateTaskRequest struct {
	Title           string    `json:"title"            validate:"required,max=200"`
	ScheduledAt     time.Time `json:"scheduled_at"     validate:"required"`
	DurationMinutes int       `json:"duration_minutes" validate:"required,gt=0"`
	Status          string    `json:"status"           validate:"required,oneof=not_urgent urgent very_urgent done"`
}
```

- [ ] **Step 2: Replace `repository/task.go`**

```go
package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskRepository interface {
	Create(ctx context.Context, tutorID string, req models.CreateTaskRequest) (models.Task, error)
	GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Task, error)
	Update(ctx context.Context, id, tutorID string, req models.UpdateTaskRequest) (models.Task, error)
	Delete(ctx context.Context, id, tutorID string) error
}

type taskRepository struct {
	conn *pgxpool.Pool
}

func NewTaskRepository(conn *pgxpool.Pool) TaskRepository {
	return &taskRepository{conn: conn}
}

func (r *taskRepository) Create(ctx context.Context, tutorID string, req models.CreateTaskRequest) (models.Task, error) {
	var t models.Task
	err := r.conn.QueryRow(ctx,
		`INSERT INTO tasks (tutor_id, title, scheduled_at, duration_minutes, status)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, tutor_id, title, status, scheduled_at, duration_minutes, created_at`,
		tutorID, req.Title, req.ScheduledAt, req.DurationMinutes, req.Status,
	).Scan(&t.ID, &t.TutorID, &t.Title, &t.Status, &t.ScheduledAt, &t.DurationMinutes, &t.CreatedAt)
	return t, err
}

func (r *taskRepository) GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Task, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT id, tutor_id, title, status, scheduled_at, duration_minutes, created_at
		 FROM tasks
		 WHERE tutor_id = $1 AND scheduled_at >= $2 AND scheduled_at < $3
		 ORDER BY scheduled_at`,
		tutorID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.TutorID, &t.Title, &t.Status, &t.ScheduledAt, &t.DurationMinutes, &t.CreatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (r *taskRepository) Update(ctx context.Context, id, tutorID string, req models.UpdateTaskRequest) (models.Task, error) {
	var t models.Task
	err := r.conn.QueryRow(ctx,
		`UPDATE tasks SET title=$1, scheduled_at=$2, duration_minutes=$3, status=$4
		 WHERE id=$5 AND tutor_id=$6
		 RETURNING id, tutor_id, title, status, scheduled_at, duration_minutes, created_at`,
		req.Title, req.ScheduledAt, req.DurationMinutes, req.Status, id, tutorID,
	).Scan(&t.ID, &t.TutorID, &t.Title, &t.Status, &t.ScheduledAt, &t.DurationMinutes, &t.CreatedAt)
	return t, err
}

func (r *taskRepository) Delete(ctx context.Context, id, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM tasks WHERE id=$1 AND tutor_id=$2`, id, tutorID,
	)
	return err
}
```

- [ ] **Step 3: Replace `service/task.go`**

```go
package service

import (
	"context"
	"tutorgo/models"
	"tutorgo/repository"
)

type TaskService interface {
	Create(ctx context.Context, tutorID string, req models.CreateTaskRequest) (models.Task, error)
	GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Task, error)
	Update(ctx context.Context, id, tutorID string, req models.UpdateTaskRequest) (models.Task, error)
	Delete(ctx context.Context, id, tutorID string) error
}

type taskService struct {
	repo repository.TaskRepository
}

func NewTaskService(repo repository.TaskRepository) TaskService {
	return &taskService{repo: repo}
}

func (s *taskService) Create(ctx context.Context, tutorID string, req models.CreateTaskRequest) (models.Task, error) {
	if req.Status == "" {
		req.Status = "not_urgent"
	}
	return s.repo.Create(ctx, tutorID, req)
}

func (s *taskService) GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Task, error) {
	return s.repo.GetByRange(ctx, tutorID, from, to)
}

func (s *taskService) Update(ctx context.Context, id, tutorID string, req models.UpdateTaskRequest) (models.Task, error) {
	return s.repo.Update(ctx, id, tutorID, req)
}

func (s *taskService) Delete(ctx context.Context, id, tutorID string) error {
	return s.repo.Delete(ctx, id, tutorID)
}
```

- [ ] **Step 4: Replace `handlers/task.go`**

```go
package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	service service.TaskService
	log     *slog.Logger
}

func NewTaskHandler(svc service.TaskService, log *slog.Logger) *TaskHandler {
	return &TaskHandler{service: svc, log: log}
}

func (h *TaskHandler) GetByRange(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	from := c.Query("from")
	to := c.Query("to")
	if from == "" || to == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to are required"})
		return
	}
	tasks, err := h.service.GetByRange(c.Request.Context(), tutorID, from, to)
	if err != nil {
		h.log.Error("Failed to get tasks", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve tasks"})
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	c.JSON(http.StatusOK, tasks)
}

func (h *TaskHandler) Create(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreateTaskRequest
	if !bindAndValidate(c, &req) {
		return
	}
	task, err := h.service.Create(c.Request.Context(), tutorID, req)
	if err != nil {
		h.log.Error("Failed to create task", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task"})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *TaskHandler) Update(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	var req models.UpdateTaskRequest
	if !bindAndValidate(c, &req) {
		return
	}
	task, err := h.service.Update(c.Request.Context(), id, tutorID, req)
	if err != nil {
		h.log.Error("Failed to update task", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task"})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), id, tutorID); err != nil {
		h.log.Error("Failed to delete task", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete task"})
		return
	}
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 5: Remove `PATCH /tasks/:id/done` from `router/router.go`**

Find and delete this line:
```go
auth.PATCH("/tasks/:id/done", taskHandler.ToggleDone)
```

- [ ] **Step 6: Build to verify no compile errors**

```bash
go build ./...
```

Expected: no output (success).

- [ ] **Step 7: Commit**

```bash
git add models/task.go repository/task.go service/task.go handlers/task.go router/router.go
git commit -m "feat: replace tasks done bool with status enum"
```

---

### Task 3: Service test

**Files:**
- Create: `service/task_test.go`

- [ ] **Step 1: Create test file**

```go
package service_test

import (
	"context"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockTaskRepo struct{ mock.Mock }

func (m *mockTaskRepo) Create(ctx context.Context, tutorID string, req models.CreateTaskRequest) (models.Task, error) {
	args := m.Called(ctx, tutorID, req)
	return args.Get(0).(models.Task), args.Error(1)
}
func (m *mockTaskRepo) GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Task, error) {
	args := m.Called(ctx, tutorID, from, to)
	return args.Get(0).([]models.Task), args.Error(1)
}
func (m *mockTaskRepo) Update(ctx context.Context, id, tutorID string, req models.UpdateTaskRequest) (models.Task, error) {
	args := m.Called(ctx, id, tutorID, req)
	return args.Get(0).(models.Task), args.Error(1)
}
func (m *mockTaskRepo) Delete(ctx context.Context, id, tutorID string) error {
	args := m.Called(ctx, id, tutorID)
	return args.Error(0)
}

func TestTaskCreate_DefaultsStatusToNotUrgent(t *testing.T) {
	repo := new(mockTaskRepo)
	svc := service.NewTaskService(repo)

	req := models.CreateTaskRequest{
		Title:           "Buy markers",
		ScheduledAt:     time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
		DurationMinutes: 30,
		// Status intentionally empty
	}
	expectedReq := req
	expectedReq.Status = "not_urgent"

	repo.On("Create", mock.Anything, "tutor-1", expectedReq).
		Return(models.Task{ID: "t1", Title: "Buy markers", Status: "not_urgent"}, nil)

	task, err := svc.Create(context.Background(), "tutor-1", req)

	assert.NoError(t, err)
	assert.Equal(t, "not_urgent", task.Status)
	repo.AssertExpectations(t)
}

func TestTaskCreate_PreservesExplicitStatus(t *testing.T) {
	repo := new(mockTaskRepo)
	svc := service.NewTaskService(repo)

	req := models.CreateTaskRequest{
		Title:           "Urgent call",
		ScheduledAt:     time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
		DurationMinutes: 15,
		Status:          "very_urgent",
	}

	repo.On("Create", mock.Anything, "tutor-1", req).
		Return(models.Task{ID: "t2", Title: "Urgent call", Status: "very_urgent"}, nil)

	task, err := svc.Create(context.Background(), "tutor-1", req)

	assert.NoError(t, err)
	assert.Equal(t, "very_urgent", task.Status)
	repo.AssertExpectations(t)
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./service/... -run TestTaskCreate -v
```

Expected:
```
--- PASS: TestTaskCreate_DefaultsStatusToNotUrgent (0.00s)
--- PASS: TestTaskCreate_PreservesExplicitStatus (0.00s)
PASS
```

- [ ] **Step 3: Run full test suite**

```bash
go test ./...
```

Expected: all PASS, no failures.

- [ ] **Step 4: Commit**

```bash
git add service/task_test.go
git commit -m "test: task service status default"
```

---

### Task 4: Frontend types, API, hooks — remove done/toggle, delete TasksWidget

**Files:**
- Modify: `frontend/src/types/api.ts`
- Modify: `frontend/src/lib/api/tasks.ts`
- Modify: `frontend/src/lib/hooks/useTasks.ts`
- Delete: `frontend/src/components/tasks/TasksWidget.tsx`
- Modify: `frontend/src/app/(dashboard)/dashboard/page.tsx`

- [ ] **Step 1: Update Task type in `frontend/src/types/api.ts`**

Find the `Task` interface and replace `done: boolean` with `status`:

```ts
export interface Task {
  id: string
  tutor_id: string
  title: string
  status: 'not_urgent' | 'urgent' | 'very_urgent' | 'done'
  scheduled_at: string
  duration_minutes: number
  created_at: string
}
```

- [ ] **Step 2: Replace `frontend/src/lib/api/tasks.ts`**

```ts
import { api } from './client'
import { Task } from '@/types/api'

export interface TaskInput {
  title: string
  scheduled_at: string
  duration_minutes: number
  status?: string
}

export interface TaskUpdateInput {
  title: string
  scheduled_at: string
  duration_minutes: number
  status: string
}

export const tasksApi = {
  list: (from: string, to: string) =>
    api.get<Task[]>('/tasks', { params: { from, to } }).then((r) => r.data ?? []),
  create: (data: TaskInput) =>
    api.post<Task>('/tasks', data).then((r) => r.data),
  update: (id: string, data: TaskUpdateInput) =>
    api.put<Task>(`/tasks/${id}`, data).then((r) => r.data),
  delete: (id: string) =>
    api.delete(`/tasks/${id}`).then(() => id),
}
```

- [ ] **Step 3: Replace `frontend/src/lib/hooks/useTasks.ts`**

```ts
import { useQuery, useMutation, useQueryClient, keepPreviousData } from '@tanstack/react-query'
import { tasksApi, TaskInput, TaskUpdateInput } from '@/lib/api/tasks'

export function useTasks(from: string, to: string) {
  return useQuery({
    queryKey:        ['tasks', from, to],
    queryFn:         () => tasksApi.list(from, to),
    enabled:         !!from && !!to,
    placeholderData: keepPreviousData,
  })
}

export function useCreateTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: TaskInput) => tasksApi.create(data),
    onSuccess:  () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}

export function useRescheduleTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: TaskUpdateInput }) => tasksApi.update(id, data),
    onSuccess:  () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}

export function useDeleteTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => tasksApi.delete(id),
    onSuccess:  () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}
```

- [ ] **Step 4: Delete `frontend/src/components/tasks/TasksWidget.tsx`**

```bash
rm frontend/src/components/tasks/TasksWidget.tsx
```

- [ ] **Step 5: Remove TasksWidget from `frontend/src/app/(dashboard)/dashboard/page.tsx`**

Remove the import line:
```ts
import { TasksWidget } from '@/components/tasks/TasksWidget'
```

Remove the JSX block:
```tsx
      {/* Задачи */}
      <div className="grid grid-cols-1 md:grid-cols-3" style={{ gap: 36, alignItems: 'start' }}>
        <TasksWidget />
      </div>
```

- [ ] **Step 6: TypeScript check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -30
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/types/api.ts frontend/src/lib/api/tasks.ts frontend/src/lib/hooks/useTasks.ts
git add frontend/src/app/\(dashboard\)/dashboard/page.tsx
git rm frontend/src/components/tasks/TasksWidget.tsx
git commit -m "feat: frontend tasks done→status, remove useToggleTask"
```

---

### Task 5: Calendar page — update done references

**Files:**
- Modify: `frontend/src/app/(dashboard)/calendar/page.tsx`

Context: The calendar loads tasks via `useTasks` and shows them as events. Currently uses `useToggleTask` (removed) and `t.done` (removed). We use `useRescheduleTask` already imported, and `t.status`.

- [ ] **Step 1: Remove `useToggleTask` from import**

Find line:
```ts
import { useTasks, useToggleTask, useRescheduleTask } from '@/lib/hooks/useTasks'
```

Replace with:
```ts
import { useTasks, useRescheduleTask } from '@/lib/hooks/useTasks'
```

- [ ] **Step 2: Remove `toggleTask` variable**

Find line:
```ts
  const toggleTask             = useToggleTask()
```

Delete it.

- [ ] **Step 3: Update `taskEvents` map — replace `t.done` with `t.status === 'done'`**

Find:
```ts
    const colors = t.done ? TASK_COLORS.done : TASK_COLORS.active
```
Replace with:
```ts
    const colors = t.status === 'done' ? TASK_COLORS.done : TASK_COLORS.active
```

Find:
```ts
        done:  t.done,
```
Replace with:
```ts
        status: t.status,
```

- [ ] **Step 4: Update drag-reschedule handler — replace `done` field with `status`**

Find:
```ts
            done:             arg.event.extendedProps.done as boolean,
```
Replace with:
```ts
            status:           arg.event.extendedProps.status as string,
```

- [ ] **Step 5: Update `eventContent` — replace `done` check and toggle call**

Find:
```ts
              const done = arg.event.extendedProps.done as boolean
```
Replace with:
```ts
              const done = arg.event.extendedProps.status === 'done'
```

Find:
```ts
                      toggleTask.mutate(arg.event.id)
```
Replace with (find the task from loaded tasks and call rescheduleTask):
```ts
                      const task = tasks.find((t) => t.id === arg.event.id)
                      if (!task) return
                      rescheduleTask.mutate({
                        id: task.id,
                        data: {
                          title:            task.title,
                          scheduled_at:     task.scheduled_at,
                          duration_minutes: task.duration_minutes,
                          status:           task.status === 'done' ? 'not_urgent' : 'done',
                        },
                      })
```

- [ ] **Step 6: TypeScript check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep "calendar" | head -20
```

Expected: no output (no errors in calendar page).

- [ ] **Step 7: Commit**

```bash
git add frontend/src/app/\(dashboard\)/calendar/page.tsx
git commit -m "feat: calendar tasks use status instead of done"
```

---

### Task 6: KanbanWidget — install @dnd-kit, create component, add to dashboard

**Files:**
- Create: `frontend/src/components/tasks/KanbanWidget.tsx`
- Modify: `frontend/src/app/(dashboard)/dashboard/page.tsx`

- [ ] **Step 1: Install @dnd-kit**

```bash
cd frontend && npm install @dnd-kit/core @dnd-kit/utilities
```

Expected: packages added to `node_modules`, `package.json` updated.

- [ ] **Step 2: Create `frontend/src/components/tasks/KanbanWidget.tsx`**

```tsx
'use client'

import { useState } from 'react'
import { toast } from 'sonner'
import { PlusIcon } from 'lucide-react'
import {
  DndContext, DragEndEvent,
  MouseSensor, TouchSensor,
  useSensor, useSensors,
  useDroppable, useDraggable,
} from '@dnd-kit/core'
import { useTasks, useCreateTask, useRescheduleTask, useDeleteTask } from '@/lib/hooks/useTasks'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetFooter } from '@/components/ui/sheet'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import type { Task } from '@/types/api'

const FROM = '2020-01-01T00:00:00Z'
const TO   = '2035-01-01T00:00:00Z'

const COLUMNS = [
  { id: 'not_urgent',  label: 'Несрочно',    color: 'var(--success)'          },
  { id: 'urgent',      label: 'Срочно',       color: 'var(--warning)'          },
  { id: 'very_urgent', label: 'Очень срочно', color: 'var(--destructive)'      },
  { id: 'done',        label: 'Выполнено',    color: 'var(--muted-foreground)' },
]

type SheetState = { open: boolean; task: Task | null }

function toDatetimeLocal(iso: string) {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function TaskCard({ task, color, onEdit }: { task: Task; color: string; onEdit: () => void }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({ id: task.id })
  return (
    <div
      ref={setNodeRef}
      {...listeners}
      {...attributes}
      onClick={onEdit}
      style={{
        opacity:        isDragging ? 0.4 : 1,
        transform:      transform ? `translate3d(${transform.x}px,${transform.y}px,0)` : undefined,
        cursor:         'grab',
        background:    'var(--card)',
        borderTop:     '1px solid var(--border)',
        borderRight:   '1px solid var(--border)',
        borderBottom:  '1px solid var(--border)',
        borderLeft:    `3px solid ${color}`,
        borderRadius:  6,
        padding:       '7px 10px',
        fontSize:      13,
        color:         task.status === 'done' ? 'var(--muted-foreground)' : 'var(--foreground)',
        textDecoration: task.status === 'done' ? 'line-through' : 'none',
        userSelect:    'none',
      }}
    >
      {task.title}
    </div>
  )
}

function Column({ id, label, color, tasks, onAddClick, onEdit }: {
  id: string; label: string; color: string
  tasks: Task[]; onAddClick: () => void; onEdit: (task: Task) => void
}) {
  const { setNodeRef, isOver } = useDroppable({ id })
  return (
    <div
      ref={setNodeRef}
      style={{
        display:       'flex',
        flexDirection: 'column',
        gap:           6,
        minHeight:     120,
        padding:       '10px 8px',
        borderRadius:  8,
        background:    isOver ? 'var(--muted)' : 'transparent',
        transition:    'background 0.15s',
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
        <span style={{
          fontSize: 11, fontWeight: 700, color,
          letterSpacing: '0.06em', textTransform: 'uppercase',
        }}>
          {label}
        </span>
        <button
          onClick={onAddClick}
          style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '0 2px', color: 'var(--muted-foreground)', display: 'flex' }}
          aria-label={`Добавить в ${label}`}
        >
          <PlusIcon size={13} />
        </button>
      </div>
      {tasks.map((task) => (
        <TaskCard key={task.id} task={task} color={color} onEdit={() => onEdit(task)} />
      ))}
    </div>
  )
}

export function KanbanWidget() {
  const { data: allTasks = [] } = useTasks(FROM, TO)
  const createTask = useCreateTask()
  const reschedule = useRescheduleTask()
  const deleteTask = useDeleteTask()

  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 5 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 250, tolerance: 5 } }),
  )

  const [sheet, setSheet]               = useState<SheetState>({ open: false, task: null })
  const [title, setTitle]               = useState('')
  const [status, setStatus]             = useState('not_urgent')
  const [scheduledAt, setScheduledAt]   = useState('')
  const [duration, setDuration]         = useState('60')

  function openCreate(defaultStatus = 'not_urgent') {
    setTitle('')
    setStatus(defaultStatus)
    setScheduledAt(toDatetimeLocal(new Date().toISOString()))
    setDuration('60')
    setSheet({ open: true, task: null })
  }

  function openEdit(task: Task) {
    setTitle(task.title)
    setStatus(task.status)
    setScheduledAt(toDatetimeLocal(task.scheduled_at))
    setDuration(String(task.duration_minutes))
    setSheet({ open: true, task })
  }

  function close() { setSheet({ open: false, task: null }) }

  function handleSave() {
    const data = {
      title:            title.trim(),
      scheduled_at:     new Date(scheduledAt).toISOString(),
      duration_minutes: Number(duration),
      status,
    }
    if (!data.title || !scheduledAt || !Number(duration)) return
    if (sheet.task) {
      reschedule.mutate({ id: sheet.task.id, data }, {
        onSuccess: () => { toast.success('Задача обновлена'); close() },
        onError:   () => toast.error('Не удалось сохранить'),
      })
    } else {
      createTask.mutate(data, {
        onSuccess: () => { toast.success('Задача создана'); close() },
        onError:   () => toast.error('Не удалось создать задачу'),
      })
    }
  }

  function handleDelete() {
    if (!sheet.task) return
    deleteTask.mutate(sheet.task.id, {
      onSuccess: () => { toast.success('Задача удалена'); close() },
      onError:   () => toast.error('Не удалось удалить'),
    })
  }

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event
    if (!over) return
    const task = allTasks.find((t) => t.id === active.id)
    if (!task || task.status === over.id) return
    reschedule.mutate({
      id:   task.id,
      data: {
        title:            task.title,
        scheduled_at:     task.scheduled_at,
        duration_minutes: task.duration_minutes,
        status:           over.id as string,
      },
    })
  }

  const isPending = createTask.isPending || reschedule.isPending || deleteTask.isPending

  return (
    <>
      <div style={{ borderBottom: '1px solid var(--border)', paddingBottom: 11, marginBottom: 12 }}>
        <h2 style={{ margin: 0, fontSize: 15, fontWeight: 600, color: 'var(--foreground)', letterSpacing: '-0.01em' }}>
          Задачи
        </h2>
      </div>

      <DndContext sensors={sensors} onDragEnd={handleDragEnd}>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 8 }}>
          {COLUMNS.map((col) => (
            <Column
              key={col.id}
              id={col.id}
              label={col.label}
              color={col.color}
              tasks={allTasks.filter((t) => t.status === col.id)}
              onAddClick={() => openCreate(col.id)}
              onEdit={openEdit}
            />
          ))}
        </div>
      </DndContext>

      <Sheet open={sheet.open} onOpenChange={(open) => !open && close()}>
        <SheetContent side="right" className="w-80">
          <SheetHeader>
            <SheetTitle>{sheet.task ? 'Редактировать задачу' : 'Новая задача'}</SheetTitle>
          </SheetHeader>
          <div className="flex flex-col gap-4 p-4">
            <div className="flex flex-col gap-1.5">
              <label style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Название</label>
              <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Название задачи" />
            </div>
            <div className="flex flex-col gap-1.5">
              <label style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Статус</label>
              <select
                value={status}
                onChange={(e) => setStatus(e.target.value)}
                style={{ padding: '6px 8px', borderRadius: 6, border: '1px solid var(--border)', background: 'var(--background)', color: 'var(--foreground)', fontSize: 13 }}
              >
                {COLUMNS.map((col) => (
                  <option key={col.id} value={col.id}>{col.label}</option>
                ))}
              </select>
            </div>
            <div className="flex flex-col gap-1.5">
              <label style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Дата и время</label>
              <Input type="datetime-local" value={scheduledAt} onChange={(e) => setScheduledAt(e.target.value)} />
            </div>
            <div className="flex flex-col gap-1.5">
              <label style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Длительность (мин)</label>
              <Input type="number" min={1} value={duration} onChange={(e) => setDuration(e.target.value)} />
            </div>
          </div>
          <SheetFooter className="flex-row justify-between px-4">
            {sheet.task && (
              <Button variant="destructive" size="sm" onClick={handleDelete} disabled={deleteTask.isPending}>
                Удалить
              </Button>
            )}
            <div className="flex gap-2 ml-auto">
              <Button variant="ghost" size="sm" onClick={close}>Отмена</Button>
              <Button size="sm" onClick={handleSave} disabled={isPending || !title.trim()}>
                Сохранить
              </Button>
            </div>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}
```

- [ ] **Step 3: Add KanbanWidget to `frontend/src/app/(dashboard)/dashboard/page.tsx`**

Add import after existing imports:
```tsx
import { KanbanWidget } from '@/components/tasks/KanbanWidget'
```

Add widget at the end of the main `<div>`, after the existing `grid grid-cols-1 md:grid-cols-3` div:
```tsx
      {/* Задачи — Канбан */}
      <div style={{ maxWidth: 900 }}>
        <KanbanWidget />
      </div>
```

- [ ] **Step 4: TypeScript check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -30
```

Expected: no output.

- [ ] **Step 5: Verify in browser**

```bash
cd frontend && npm run dev
```

Open `http://localhost:3000/dashboard` and verify:
- Full-width Kanban board appears below existing widgets with 4 columns
- Each column has a colored label (green/yellow/red/grey) and a `+` button
- Clicking `+` on a column opens the Sheet with that column pre-selected
- Creating a task adds it to the correct column
- Dragging a card to another column moves it (optimistic update via React Query)
- Clicking a card opens the Sheet in edit mode with prefilled fields
- Deleting a task removes it from the board
- Open `http://localhost:3000/calendar` and verify tasks still appear and the done toggle (checkbox icon) still works

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/tasks/KanbanWidget.tsx
git add frontend/src/app/\(dashboard\)/dashboard/page.tsx
git add frontend/package.json frontend/package-lock.json
git commit -m "feat: KanbanWidget with drag-and-drop"
```
