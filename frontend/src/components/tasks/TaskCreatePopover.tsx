'use client'

import { useState } from 'react'
import { toast } from 'sonner'
import { useCreateTask } from '@/lib/hooks/useTasks'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTitle } from '@/components/ui/popover'
import { Input } from '@/components/ui/input'

interface Props {
  start:   Date | null
  end:     Date | null
  /** Прямоугольник выделенного слота: сама подсветка снимается сразу после
   *  выделения, привязываться не к чему — поэтому храним rect, а не узел. */
  anchor:  DOMRect | null
  onClose: () => void
}

function formatSlot(start: Date, end: Date): string {
  const day = start.toLocaleDateString('ru-RU', { weekday: 'short', day: 'numeric', month: 'short' })
  const t1  = start.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
  const t2  = end.toLocaleTimeString('ru-RU',   { hour: '2-digit', minute: '2-digit' })
  return `${day} · ${t1} – ${t2}`
}

export function TaskCreatePopover({ start, end, anchor, onClose }: Props) {
  return (
    <Popover open={!!start} onOpenChange={(open) => !open && onClose()}>
      <PopoverContent anchor={anchor}>
        <PopoverTitle className="pr-8">Новая задача</PopoverTitle>
        {/* key: новый слот пересоздаёт форму — поле названия чистое без эффекта.
            Фокус в поле ставит сам поповер: это первый tabbable-элемент внутри. */}
        {start && end && (
          <TaskForm key={start.toISOString()} start={start} end={end} onClose={onClose} />
        )}
      </PopoverContent>
    </Popover>
  )
}

function TaskForm({ start, end, onClose }: { start: Date; end: Date; onClose: () => void }) {
  const [title, setTitle] = useState('')
  const createTask        = useCreateTask()

  function handleSave() {
    if (!title.trim()) return
    const duration = Math.max(15, Math.round((end.getTime() - start.getTime()) / 60_000))
    createTask.mutate(
      { title: title.trim(), scheduled_at: start.toISOString(), duration_minutes: duration },
      { onError: () => toast.error('Не удалось создать задачу') },
    )
    onClose()
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter') handleSave()
  }

  return (
    <>
      <p className="text-xs text-muted-foreground -mt-2">{formatSlot(start, end)}</p>
      <Input
        placeholder="Название задачи"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        onKeyDown={handleKeyDown}
      />
      <div className="flex justify-end gap-2 pt-1">
        <Button variant="ghost" size="sm" onClick={onClose}>Отмена</Button>
        <Button size="sm" onClick={handleSave} disabled={!title.trim()}>
          Создать
        </Button>
      </div>
    </>
  )
}
