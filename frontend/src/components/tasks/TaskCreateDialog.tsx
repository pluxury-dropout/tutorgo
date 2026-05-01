'use client'

import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { useCreateTask } from '@/lib/hooks/useTasks'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'

interface Props {
  start:   Date | null
  end:     Date | null
  onClose: () => void
}

function formatSlot(start: Date, end: Date): string {
  const day = start.toLocaleDateString('ru-RU', { weekday: 'short', day: 'numeric', month: 'short' })
  const t1  = start.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
  const t2  = end.toLocaleTimeString('ru-RU',   { hour: '2-digit', minute: '2-digit' })
  return `${day} · ${t1} – ${t2}`
}

export function TaskCreateDialog({ start, end, onClose }: Props) {
  const [title, setTitle] = useState('')
  const inputRef          = useRef<HTMLInputElement>(null)
  const createTask        = useCreateTask()

  useEffect(() => {
    if (start) {
      setTitle('')
      setTimeout(() => inputRef.current?.focus(), 50)
    }
  }, [start])

  function handleSave() {
    if (!start || !end || !title.trim()) return
    const duration = Math.max(15, Math.round((end.getTime() - start.getTime()) / 60_000))
    createTask.mutate(
      { title: title.trim(), scheduled_at: start.toISOString(), duration_minutes: duration },
      {
        onSuccess: () => { toast.success('Задача создана'); onClose() },
        onError:   () => toast.error('Не удалось создать задачу'),
      },
    )
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter') handleSave()
  }

  return (
    <Dialog open={!!start} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>Новая задача</DialogTitle>
        </DialogHeader>
        {start && end && (
          <p className="text-xs text-muted-foreground -mt-2">{formatSlot(start, end)}</p>
        )}
        <Input
          ref={inputRef}
          placeholder="Название задачи"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={handleKeyDown}
        />
        <div className="flex justify-end gap-2 pt-1">
          <Button variant="ghost" size="sm" onClick={onClose}>Отмена</Button>
          <Button size="sm" onClick={handleSave} disabled={!title.trim() || createTask.isPending}>
            Создать
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
