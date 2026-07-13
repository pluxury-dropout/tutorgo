'use client'

import { useState, useMemo } from 'react'
import DOMPurify from 'dompurify'
import TaskEditor from './TaskEditor'
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
  borderRadius: 9,
  padding: '9px 11px',
  marginBottom: 7,
  background: 'var(--background)',
  border: '1px solid var(--border)',
  fontSize: 12.5,
}

function TaskCard({
  task,
  onClick,
  onDelete,
}: {
  task: Task
  onClick: () => void
  onDelete: () => void
}) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({ id: task.id })
  // Санитизация дёргает DOM-парсер; мемоизируем, чтобы не гонять на каждый drag-рендер.
  const safeHtml = useMemo(() => DOMPurify.sanitize(task.title), [task.title])
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
        userSelect: 'none',
        display: 'flex',
        alignItems: 'flex-start',
        gap: 6,
      }}
    >
      <div
        className="task-content"
        style={{ flex: 1, minWidth: 0, overflowWrap: 'anywhere' }}
        dangerouslySetInnerHTML={{ __html: safeHtml }}
      />
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
    <div className="kanban-col">
      <div style={{ display: 'flex', alignItems: 'center', gap: 7, marginBottom: 12 }}>
        <span style={{ width: 7, height: 7, borderRadius: '50%', background: col.color, flexShrink: 0 }} />
        <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--foreground)' }}>{col.label}</span>
        <span style={{
          fontSize: 10.5, color: 'var(--muted-foreground)', background: 'var(--muted)',
          borderRadius: 20, padding: '1px 7px', marginLeft: 'auto',
        }}>
          {tasks.length}
        </span>
      </div>
      <div
        ref={setNodeRef}
        className="kanban-drop"
        onClick={(e) => { if (e.target === e.currentTarget && !draftOpen) onOpenDraft() }}
        style={{
          borderRadius: 6,
          background: isOver ? 'var(--muted)' : 'transparent',
          transition: 'background 0.15s',
          cursor: 'text',
        }}
      >
        {tasks.map(task =>
          editingId === task.id ? (
            <TaskEditor
              key={task.id}
              defaultValue={task.title}
              onCommit={(html) => onCommitEdit(task, html)}
              onCancel={onCancelEdit}
            />
          ) : (
            <TaskCard
              key={task.id}
              task={task}
              onClick={() => onCardClick(task)}
              onDelete={() => onCardDelete(task)}
            />
          ),
        )}
        {draftOpen && (
          <TaskEditor defaultValue="" onCommit={onCommitDraft} onCancel={onCancelDraft} />
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
      <div className="kanban-grid">
        {COLUMNS.map((col) => (
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
