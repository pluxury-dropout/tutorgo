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
import { useBoardTasks, useCreateTask, useRescheduleTask, useDeleteTask } from '@/lib/hooks/useTasks'
import { Task } from '@/types/api'

const COLUMNS = [
  { id: 'not_urgent',  label: 'Несрочно',      color: 'var(--success)' },
  { id: 'urgent',      label: 'Срочно',         color: 'var(--warning)' },
  { id: 'very_urgent', label: 'Очень срочно',   color: 'var(--destructive)' },
  { id: 'done',        label: 'Выполнено',      color: 'var(--muted-foreground)' },
] as const

const cardBaseStyle: React.CSSProperties = {
  borderRadius: 6,
  padding: '8px 10px',
  marginBottom: 6,
  background: 'var(--card)',
  borderTop: '1px solid var(--border)',
  borderRight: '1px solid var(--border)',
  borderBottom: '1px solid var(--border)',
  fontSize: 13,
}

function TaskCard({
  task,
  color,
  onClick,
  onDelete,
}: {
  task: Task
  color: string
  onClick: () => void
  onDelete: () => void
}) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({ id: task.id })
  return (
    <div
      ref={setNodeRef}
      {...listeners}
      {...attributes}
      onClick={onClick}
      style={{
        ...cardBaseStyle,
        transform: transform ? `translate3d(${transform.x}px,${transform.y}px,0)` : undefined,
        opacity: isDragging ? 0.4 : 1,
        cursor: 'grab',
        borderLeft: `3px solid ${color}`,
        userSelect: 'none',
        display: 'flex',
        alignItems: 'center',
        gap: 6,
      }}
    >
      <span style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {task.title}
      </span>
      <button
        onClick={(e) => { e.stopPropagation(); onDelete() }}
        onPointerDown={(e) => e.stopPropagation()}
        style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--muted-foreground)', fontSize: 14, lineHeight: 1, padding: 0, opacity: 0.5 }}
        title="Удалить"
      >
        ×
      </button>
    </div>
  )
}

// Inline-поле для создания и редактирования задачи. Сохраняет по blur/Enter, отмена по Escape.
function InlineInput({
  defaultValue,
  color,
  onCommit,
  onCancel,
}: {
  defaultValue: string
  color: string
  onCommit: (value: string) => void
  onCancel: () => void
}) {
  const [value, setValue] = useState(defaultValue)
  return (
    <input
      autoFocus
      value={value}
      placeholder="Название задачи"
      onChange={(e) => setValue(e.target.value)}
      onBlur={() => onCommit(value)}
      onKeyDown={(e) => {
        if (e.key === 'Enter') (e.target as HTMLInputElement).blur()
        if (e.key === 'Escape') onCancel()
      }}
      style={{
        ...cardBaseStyle,
        borderLeft: `3px solid ${color}`,
        width: '100%',
        outline: 'none',
        color: 'var(--foreground)',
        fontFamily: 'inherit',
      }}
    />
  )
}

function Column({
  col,
  tasks,
  editingId,
  draftOpen,
  onCardClick,
  onCardDelete,
  onCommitEdit,
  onCancelEdit,
  onOpenDraft,
  onCommitDraft,
  onCancelDraft,
}: {
  col: typeof COLUMNS[number]
  tasks: Task[]
  editingId: string | null
  draftOpen: boolean
  onCardClick: (task: Task) => void
  onCardDelete: (task: Task) => void
  onCommitEdit: (task: Task, title: string) => void
  onCancelEdit: () => void
  onOpenDraft: () => void
  onCommitDraft: (title: string) => void
  onCancelDraft: () => void
}) {
  const { setNodeRef, isOver } = useDroppable({ id: col.id })
  return (
    <div style={{ flex: 1, minWidth: 0 }}>
      <div style={{ marginBottom: 8 }}>
        <span style={{ fontSize: 12, fontWeight: 600, color: col.color, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
          {col.label}
        </span>
      </div>
      <div
        ref={setNodeRef}
        onClick={(e) => { if (e.target === e.currentTarget && !draftOpen) onOpenDraft() }}
        style={{
          minHeight: 80,
          borderRadius: 6,
          padding: 4,
          background: isOver ? 'var(--muted)' : 'transparent',
          transition: 'background 0.15s',
          cursor: 'text',
        }}
      >
        {tasks.map(task =>
          editingId === task.id ? (
            <InlineInput
              key={task.id}
              defaultValue={task.title}
              color={col.color}
              onCommit={(title) => onCommitEdit(task, title)}
              onCancel={onCancelEdit}
            />
          ) : (
            <TaskCard
              key={task.id}
              task={task}
              color={col.color}
              onClick={() => onCardClick(task)}
              onDelete={() => onCardDelete(task)}
            />
          ),
        )}
        {draftOpen && (
          <InlineInput defaultValue="" color={col.color} onCommit={onCommitDraft} onCancel={onCancelDraft} />
        )}
      </div>
    </div>
  )
}

export default function KanbanWidget() {
  const [editingId, setEditingId]   = useState<string | null>(null)
  const [draftStatus, setDraftStatus] = useState<string | null>(null)

  const { data: tasks = [] } = useBoardTasks()
  const createTask = useCreateTask()
  const reschedule = useRescheduleTask()
  const deleteTask = useDeleteTask()

  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 5 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 250, tolerance: 5 } }),
  )

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event
    if (!over) return
    const task = tasks.find(t => t.id === String(active.id))
    if (!task || task.status === String(over.id)) return
    reschedule.mutate({
      id: String(active.id),
      data: { title: task.title, scheduled_at: task.scheduled_at, duration_minutes: task.duration_minutes, status: String(over.id) },
    })
  }

  function commitDraft(title: string) {
    const t = title.trim()
    if (t && draftStatus) createTask.mutate({ title: t, status: draftStatus })
    setDraftStatus(null)
  }

  function commitEdit(task: Task, title: string) {
    const t = title.trim()
    // Прокидываем существующие scheduled_at/duration, чтобы не затереть календарные задачи.
    if (t && t !== task.title) {
      reschedule.mutate({
        id: task.id,
        data: { title: t, status: task.status, scheduled_at: task.scheduled_at, duration_minutes: task.duration_minutes },
      })
    }
    setEditingId(null)
  }

  return (
    <DndContext sensors={sensors} onDragEnd={handleDragEnd}>
      <div style={{ display: 'flex', gap: 12 }}>
        {COLUMNS.map(col => (
          <Column
            key={col.id}
            col={col}
            tasks={tasks.filter(t => t.status === col.id)}
            editingId={editingId}
            draftOpen={draftStatus === col.id}
            onCardClick={(task) => { setDraftStatus(null); setEditingId(task.id) }}
            onCardDelete={(task) => deleteTask.mutate(task.id)}
            onCommitEdit={commitEdit}
            onCancelEdit={() => setEditingId(null)}
            onOpenDraft={() => { setEditingId(null); setDraftStatus(col.id) }}
            onCommitDraft={commitDraft}
            onCancelDraft={() => setDraftStatus(null)}
          />
        ))}
      </div>
    </DndContext>
  )
}
