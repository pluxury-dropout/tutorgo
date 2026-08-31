'use client'

import { useState } from 'react'
import { toast } from 'sonner'
import { AlertTriangle } from 'lucide-react'

import { useCreateTask } from '@/lib/hooks/useTasks'
import { useCreateEvent, useConflicts } from '@/lib/hooks/useEvents'
import { EVENT_KINDS, KIND_LABELS, formatTimeRange } from '@/lib/eventKind'
import type { EventKind } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTitle } from '@/components/ui/popover'
import { Input } from '@/components/ui/input'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'

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

function slotMinutes(start: Date, end: Date): number {
  return Math.max(15, Math.round((end.getTime() - start.getTime()) / 60_000))
}

export function SlotCreatePopover({ start, end, anchor, onClose }: Props) {
  return (
    <Popover open={!!start} onOpenChange={(open) => !open && onClose()}>
      <PopoverContent anchor={anchor}>
        <PopoverTitle className="pr-8">Новая запись</PopoverTitle>
        {/* key: новый слот пересоздаёт форму — поля чистые без эффекта.
            Фокус в поле ставит сам поповер: это первый tabbable-элемент. */}
        {start && end && (
          <SlotForm key={start.toISOString()} start={start} end={end} onClose={onClose} />
        )}
      </PopoverContent>
    </Popover>
  )
}

function SlotForm({ start, end, onClose }: { start: Date; end: Date; onClose: () => void }) {
  // Тип не запоминается между открытиями: иначе репетитор, один раз создавший
  // задачу, потом молча создаёт задачи вместо всего остального.
  const [tab, setTab] = useState('event')

  // Занятость запрашиваем сразу при открытии, до заполнения полей.
  const { data: conflicts = [] } = useConflicts({
    starts_at:        start.toISOString(),
    duration_minutes: slotMinutes(start, end),
  })

  return (
    <>
      <p className="text-xs text-muted-foreground -mt-2">{formatSlot(start, end)}</p>

      {conflicts.length > 0 && (
        <p className="flex items-start gap-1.5 rounded-md bg-[var(--cal-missed-bg)] px-2 py-1.5 text-xs text-[var(--cal-missed-text)]">
          <AlertTriangle className="mt-px size-3.5 shrink-0" />
          <span>
            Занято: {conflicts.map((c) => `${c.title} ${formatTimeRange(c.starts_at, c.duration_minutes)}`).join(', ')}
          </span>
        </p>
      )}

      <Tabs value={tab} onValueChange={(v) => setTab(v as string)}>
        <TabsList className="w-full">
          <TabsTrigger value="event">Событие</TabsTrigger>
          <TabsTrigger value="task">Задача</TabsTrigger>
        </TabsList>

        <TabsContent value="event" className="pt-3">
          <EventFields start={start} end={end} onClose={onClose} />
        </TabsContent>
        <TabsContent value="task" className="pt-3">
          <TaskFields start={start} end={end} onClose={onClose} />
        </TabsContent>
      </Tabs>
    </>
  )
}

function EventFields({ start, end, onClose }: { start: Date; end: Date; onClose: () => void }) {
  const [title, setTitle] = useState('')
  const [kind, setKind]   = useState<EventKind>('personal')
  const createEvent = useCreateEvent()

  function handleSave() {
    if (!title.trim()) return
    createEvent.mutate(
      {
        title:            title.trim(),
        kind,
        starts_at:        start.toISOString(),
        duration_minutes: slotMinutes(start, end),
      },
      { onError: () => toast.error('Не удалось создать событие') },
    )
    onClose()
  }

  return (
    <div className="space-y-3">
      <Input
        placeholder="Название события"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && handleSave()}
      />
      <div className="flex gap-1">
        {EVENT_KINDS.map((k) => (
          <button
            key={k}
            type="button"
            onClick={() => setKind(k)}
            className={`h-7 flex-1 rounded text-xs font-medium transition-colors ${
              kind === k ? 'bg-primary text-primary-foreground' : 'border border-input hover:bg-muted'
            }`}
          >
            {KIND_LABELS[k]}
          </button>
        ))}
      </div>
      <div className="flex justify-end gap-2">
        <Button variant="ghost" size="sm" onClick={onClose}>Отмена</Button>
        <Button size="sm" onClick={handleSave} disabled={!title.trim()}>Создать</Button>
      </div>
    </div>
  )
}

function TaskFields({ start, end, onClose }: { start: Date; end: Date; onClose: () => void }) {
  const [title, setTitle] = useState('')
  const createTask        = useCreateTask()

  function handleSave() {
    if (!title.trim()) return
    createTask.mutate(
      { title: title.trim(), scheduled_at: start.toISOString(), duration_minutes: slotMinutes(start, end) },
      { onError: () => toast.error('Не удалось создать задачу') },
    )
    onClose()
  }

  return (
    <div className="space-y-3">
      <Input
        placeholder="Название задачи"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && handleSave()}
      />
      <div className="flex justify-end gap-2">
        <Button variant="ghost" size="sm" onClick={onClose}>Отмена</Button>
        <Button size="sm" onClick={handleSave} disabled={!title.trim()}>Создать</Button>
      </div>
    </div>
  )
}
