# Tasks Widget Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a tasks widget to the dashboard with full CRUD — create, edit via Sheet, toggle done, delete.

**Architecture:** Single `TasksWidget.tsx` component with internal Sheet state for create/edit. Fetches all tasks via `useTasks` with a wide date range, filters `done=false` client-side. Dropped into `dashboard/page.tsx` as a fourth widget below the existing 3-column grid.

**Tech Stack:** Next.js 14, React, TanStack Query, shadcn/ui Sheet, existing `useTasks`/`useCreateTask`/`useRescheduleTask`/`useToggleTask`/`useDeleteTask` hooks.

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `frontend/src/components/tasks/TasksWidget.tsx` | Widget + Sheet (create/edit) |
| Modify | `frontend/src/app/(dashboard)/dashboard/page.tsx` | Import and render widget |

---

### Task 1: Create TasksWidget component

**Files:**
- Create: `frontend/src/components/tasks/TasksWidget.tsx`

- [ ] **Step 1: Create the component file**

```tsx
'use client'

import { useState } from 'react'
import { toast } from 'sonner'
import { PencilIcon, PlusIcon } from 'lucide-react'
import { useTasks, useCreateTask, useRescheduleTask, useToggleTask, useDeleteTask } from '@/lib/hooks/useTasks'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetFooter } from '@/components/ui/sheet'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import type { Task } from '@/types/api'

const FROM = '2020-01-01T00:00:00Z'
const TO   = '2035-01-01T00:00:00Z'

const WIDGET_HEAD: React.CSSProperties = {
  display: 'flex', alignItems: 'baseline', justifyContent: 'space-between',
  paddingBottom: 11, marginBottom: 4, borderBottom: '1px solid var(--border)',
}
const WIDGET_TITLE: React.CSSProperties = {
  margin: 0, fontSize: 15, fontWeight: 600,
  color: 'var(--foreground)', letterSpacing: '-0.01em',
}
const EMPTY: React.CSSProperties = {
  fontSize: 13, color: 'var(--muted-foreground)', textAlign: 'center', padding: '24px 0',
}

type SheetState = { open: boolean; task: Task | null }

function toDatetimeLocal(iso: string) {
  return new Date(iso).toISOString().slice(0, 16)
}

export function TasksWidget() {
  const { data: allTasks = [] } = useTasks(FROM, TO)
  const tasks = allTasks.filter((t) => !t.done)

  const createTask    = useCreateTask()
  const reschedule    = useRescheduleTask()
  const toggleDone    = useToggleTask()
  const deleteTask    = useDeleteTask()

  const [sheet, setSheet]           = useState<SheetState>({ open: false, task: null })
  const [title, setTitle]           = useState('')
  const [scheduledAt, setScheduledAt] = useState('')
  const [duration, setDuration]     = useState('60')

  function openCreate() {
    setTitle('')
    setScheduledAt(toDatetimeLocal(new Date().toISOString()))
    setDuration('60')
    setSheet({ open: true, task: null })
  }

  function openEdit(task: Task) {
    setTitle(task.title)
    setScheduledAt(toDatetimeLocal(task.scheduled_at))
    setDuration(String(task.duration_minutes))
    setSheet({ open: true, task })
  }

  function close() {
    setSheet({ open: false, task: null })
  }

  function handleSave() {
    const data = {
      title:            title.trim(),
      scheduled_at:     new Date(scheduledAt).toISOString(),
      duration_minutes: Number(duration),
    }
    if (!data.title || !scheduledAt || !Number(duration)) return

    if (sheet.task) {
      reschedule.mutate(
        { id: sheet.task.id, data: { ...data, done: sheet.task.done } },
        {
          onSuccess: () => { toast.success('Задача обновлена'); close() },
          onError:   () => toast.error('Не удалось сохранить'),
        },
      )
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

  const isPending = createTask.isPending || reschedule.isPending

  return (
    <>
      <section style={{ display: 'flex', flexDirection: 'column' }}>
        <header style={WIDGET_HEAD}>
          <h2 style={WIDGET_TITLE}>Задачи</h2>
          <button
            onClick={openCreate}
            style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '0 2px', color: 'var(--muted-foreground)', display: 'flex', alignItems: 'center' }}
            aria-label="Создать задачу"
          >
            <PlusIcon size={15} />
          </button>
        </header>

        {tasks.length === 0 ? (
          <p style={EMPTY}>Задач нет</p>
        ) : (
          tasks.map((task, i) => (
            <div
              key={task.id}
              style={{
                display: 'grid',
                gridTemplateColumns: '20px 1fr 20px',
                alignItems: 'center',
                gap: 10,
                padding: '8px 0',
                borderTop: i === 0 ? 'none' : '1px solid var(--border)',
              }}
              className="group"
            >
              <input
                type="checkbox"
                checked={task.done}
                onChange={() => toggleDone.mutate(task.id)}
                style={{ cursor: 'pointer', accentColor: 'var(--primary)', width: 14, height: 14 }}
              />
              <span style={{ fontSize: 14, color: 'var(--foreground)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                {task.title}
              </span>
              <button
                onClick={() => openEdit(task)}
                style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0, color: 'var(--muted-foreground)', display: 'flex', alignItems: 'center', opacity: 0 }}
                className="group-hover:opacity-100"
                aria-label="Редактировать задачу"
              >
                <PencilIcon size={13} />
              </button>
            </div>
          ))
        )}
      </section>

      <Sheet open={sheet.open} onOpenChange={(open) => !open && close()}>
        <SheetContent side="right" className="w-80">
          <SheetHeader>
            <SheetTitle>{sheet.task ? 'Редактировать задачу' : 'Новая задача'}</SheetTitle>
          </SheetHeader>

          <div className="flex flex-col gap-4 p-4">
            <div className="flex flex-col gap-1.5">
              <label style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Название</label>
              <Input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="Название задачи"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Дата и время</label>
              <Input
                type="datetime-local"
                value={scheduledAt}
                onChange={(e) => setScheduledAt(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Длительность (мин)</label>
              <Input
                type="number"
                min={1}
                value={duration}
                onChange={(e) => setDuration(e.target.value)}
              />
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

- [ ] **Step 2: Check TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep -i "tasks\|TasksWidget" | head -20
```

Expected: no output (no errors in these files).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/tasks/TasksWidget.tsx
git commit -m "feat: TasksWidget component with Sheet create/edit"
```

---

### Task 2: Add widget to dashboard

**Files:**
- Modify: `frontend/src/app/(dashboard)/dashboard/page.tsx`

- [ ] **Step 1: Add import at top of dashboard/page.tsx**

After the existing imports (around line 10), add:

```tsx
import { TasksWidget } from '@/components/tasks/TasksWidget'
```

- [ ] **Step 2: Add widget below the existing 3-column grid**

After the closing `</div>` of the `grid grid-cols-1 md:grid-cols-3` div (around line 286), add:

```tsx
      {/* Задачи */}
      <div className="grid grid-cols-1 md:grid-cols-3" style={{ gap: 36, alignItems: 'start' }}>
        <TasksWidget />
      </div>
```

- [ ] **Step 3: TypeScript check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```

Expected: no output.

- [ ] **Step 4: Run dev server and verify**

```bash
cd frontend && npm run dev
```

Open `http://localhost:3000/dashboard` and verify:
- Widget "Задачи" appears below the existing widgets
- [+] button opens Sheet on the right with empty form
- Filling in title + date + duration and clicking "Сохранить" creates a task and closes the Sheet
- Task appears in the list
- Hovering a task row reveals the ✎ pencil icon
- Clicking ✎ opens Sheet with prefilled fields
- Editing and saving updates the task
- "Удалить" removes the task
- Checking the checkbox removes it from the list (done=true filtered out)
- Toast notifications appear on success/error

- [ ] **Step 5: Commit**

```bash
git add frontend/src/app/\(dashboard\)/dashboard/page.tsx
git commit -m "feat: add TasksWidget to dashboard"
```
