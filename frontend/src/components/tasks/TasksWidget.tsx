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
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
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

  const isPending = createTask.isPending || reschedule.isPending || deleteTask.isPending

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
                style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0, color: 'var(--muted-foreground)', display: 'flex', alignItems: 'center' }}
                className="opacity-0 group-hover:opacity-100"
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
