'use client'

import { useState } from 'react'
import {
  DndContext,
  DragEndEvent,
  MouseSensor,
  TouchSensor,
  useSensor,
  useSensors,
  useDroppable,
  useDraggable,
} from '@dnd-kit/core'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useBoardTasks, useCreateTask, useRescheduleTask, useDeleteTask } from '@/lib/hooks/useTasks'
import { Task } from '@/types/api'

const COLUMNS = [
  { id: 'not_urgent',  label: 'Несрочно',      color: 'var(--success)' },
  { id: 'urgent',      label: 'Срочно',         color: 'var(--warning)' },
  { id: 'very_urgent', label: 'Очень срочно',   color: 'var(--destructive)' },
  { id: 'done',        label: 'Выполнено',      color: 'var(--muted-foreground)' },
] as const

function TaskCard({ task, color, onClick }: { task: Task; color: string; onClick: () => void }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({ id: task.id })
  return (
    <div
      ref={setNodeRef}
      {...listeners}
      {...attributes}
      onClick={onClick}
      style={{
        transform: transform ? `translate3d(${transform.x}px,${transform.y}px,0)` : undefined,
        opacity: isDragging ? 0.4 : 1,
        cursor: 'grab',
        borderRadius: 6,
        padding: '8px 10px',
        marginBottom: 6,
        background: 'var(--card)',
        borderTop: '1px solid var(--border)',
        borderRight: '1px solid var(--border)',
        borderBottom: '1px solid var(--border)',
        borderLeft: `3px solid ${color}`,
        fontSize: 13,
        userSelect: 'none',
      }}
    >
      {task.title}
    </div>
  )
}

function Column({
  col,
  tasks,
  onCardClick,
  onAdd,
}: {
  col: typeof COLUMNS[number]
  tasks: Task[]
  onCardClick: (task: Task) => void
  onAdd: () => void
}) {
  const { setNodeRef, isOver } = useDroppable({ id: col.id })
  return (
    <div style={{ flex: 1, minWidth: 0 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
        <span style={{ fontSize: 12, fontWeight: 600, color: col.color, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
          {col.label}
        </span>
        <button
          onClick={onAdd}
          style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--muted-foreground)', fontSize: 16, lineHeight: 1, padding: '0 2px' }}
        >
          +
        </button>
      </div>
      <div
        ref={setNodeRef}
        style={{
          minHeight: 80,
          borderRadius: 6,
          padding: 4,
          background: isOver ? 'var(--muted)' : 'transparent',
          transition: 'background 0.15s',
        }}
      >
        {tasks.map(task => (
          <TaskCard key={task.id} task={task} color={col.color} onClick={() => onCardClick(task)} />
        ))}
      </div>
    </div>
  )
}

type SheetMode = { mode: 'create'; status: string } | { mode: 'edit'; task: Task } | null

export default function KanbanWidget() {
  const [sheet, setSheet] = useState<SheetMode>(null)
  const [form, setForm] = useState({ title: '', status: 'not_urgent' })

  const { data: tasks = [] } = useBoardTasks()
  const createTask = useCreateTask()
  const reschedule = useRescheduleTask()
  const deleteTask = useDeleteTask()

  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 5 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 250, tolerance: 5 } }),
  )

  function openCreate(status: string) {
    setForm({ title: '', status })
    setSheet({ mode: 'create', status })
  }

  function openEdit(task: Task) {
    setForm({ title: task.title, status: task.status })
    setSheet({ mode: 'edit', task })
  }

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event
    if (!over) return
    const task = tasks.find(t => t.id === String(active.id))
    if (!task) return
    if (task.status === String(over.id)) return  // already in this column
    reschedule.mutate({
      id: String(active.id),
      data: { title: task.title, scheduled_at: task.scheduled_at, duration_minutes: task.duration_minutes, status: String(over.id) },
    })
  }

  function handleSave() {
    if (sheet?.mode === 'create') {
      // Канбан-задача: только название и статус, без времени.
      createTask.mutate(form, { onSuccess: () => setSheet(null) })
    } else if (sheet?.mode === 'edit') {
      // Прокидываем существующие scheduled_at/duration, чтобы не затереть календарные задачи.
      reschedule.mutate(
        {
          id: sheet.task.id,
          data: { ...form, scheduled_at: sheet.task.scheduled_at, duration_minutes: sheet.task.duration_minutes },
        },
        { onSuccess: () => setSheet(null) },
      )
    }
  }

  function handleDelete() {
    if (sheet?.mode === 'edit') {
      deleteTask.mutate(sheet.task.id, { onSuccess: () => setSheet(null) })
    }
  }

  const isPending = createTask.isPending || reschedule.isPending || deleteTask.isPending

  return (
    <>
      <DndContext sensors={sensors} onDragEnd={handleDragEnd}>
        <div style={{ display: 'flex', gap: 12 }}>
          {COLUMNS.map(col => (
            <Column
              key={col.id}
              col={col}
              tasks={tasks.filter(t => t.status === col.id)}
              onCardClick={openEdit}
              onAdd={() => openCreate(col.id)}
            />
          ))}
        </div>
      </DndContext>

      <Sheet open={sheet !== null} onOpenChange={open => !open && setSheet(null)}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{sheet?.mode === 'create' ? 'Новая задача' : 'Редактировать задачу'}</SheetTitle>
          </SheetHeader>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16, paddingTop: 16 }}>
            <div>
              <Label>Название</Label>
              <Input value={form.title} onChange={e => setForm(f => ({ ...f, title: e.target.value }))} />
            </div>
            <div>
              <Label>Статус</Label>
              <select
                value={form.status}
                onChange={e => setForm(f => ({ ...f, status: e.target.value }))}
                style={{ width: '100%', padding: '8px', borderRadius: 6, border: '1px solid var(--border)', background: 'var(--background)', color: 'var(--foreground)', fontSize: 14 }}
              >
                {COLUMNS.map(c => <option key={c.id} value={c.id}>{c.label}</option>)}
              </select>
            </div>
            <Button onClick={handleSave} disabled={isPending}>
              {isPending ? 'Сохранение...' : 'Сохранить'}
            </Button>
            {sheet?.mode === 'edit' && (
              <Button variant="destructive" onClick={handleDelete} disabled={isPending}>
                Удалить
              </Button>
            )}
          </div>
        </SheetContent>
      </Sheet>
    </>
  )
}
