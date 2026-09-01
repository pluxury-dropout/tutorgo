'use client'

import { useState } from 'react'
import { toast } from 'sonner'
import { Trash2 } from 'lucide-react'

import { useUpdateEvent, useDeleteEvent } from '@/lib/hooks/useEvents'
import { EVENT_KINDS, KIND_LABELS, formatTimeRange } from '@/lib/eventKind'
import type { Event, EventKind } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTitle } from '@/components/ui/popover'
import { Input } from '@/components/ui/input'

interface Props {
  event:   Event | null
  /** Блок события в сетке — поповер встаёт рядом с ним. */
  anchor:  Element | null
  onClose: () => void
}

export function EventQuickPopover({ event, anchor, onClose }: Props) {
  return (
    <Popover open={!!event} onOpenChange={(open) => !open && onClose()}>
      <PopoverContent anchor={anchor}>
        {/* key: смена события пересоздаёт форму, поля берутся из пропа
            напрямую — без синхронизирующих эффектов. */}
        {event && <EventForm key={event.id} event={event} onClose={onClose} />}
      </PopoverContent>
    </Popover>
  )
}

function EventForm({ event, onClose }: { event: Event; onClose: () => void }) {
  const [title, setTitle]       = useState(event.title)
  const [kind, setKind]         = useState<EventKind>(event.kind)
  const [location, setLocation] = useState(event.location)
  const [notes, setNotes]       = useState(event.notes)

  const updateEvent = useUpdateEvent()
  const deleteEvent = useDeleteEvent()

  function handleSave() {
    if (!title.trim()) return
    updateEvent.mutate(
      {
        id:   event.id,
        data: {
          title:            title.trim(),
          kind,
          starts_at:        event.starts_at,
          duration_minutes: event.duration_minutes,
          color:            event.color,
          location:         location.trim(),
          notes:            notes.trim(),
        },
      },
      { onError: () => toast.error('Не удалось сохранить событие') },
    )
    onClose()
  }

  function handleDelete() {
    deleteEvent.mutate(event.id, { onError: () => toast.error('Не удалось удалить событие') })
    onClose()
  }

  return (
    <>
      <PopoverTitle className="pr-8">Событие</PopoverTitle>
      <p className="text-xs text-muted-foreground -mt-2">
        {new Date(event.starts_at).toLocaleDateString('ru-RU', { weekday: 'short', day: 'numeric', month: 'short' })}
        {` · ${formatTimeRange(event.starts_at, event.duration_minutes)}`}
      </p>

      <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Название" />

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

      <Input value={location} onChange={(e) => setLocation(e.target.value)} placeholder="Место (необязательно)" />
      <Input value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Заметка (необязательно)" />

      <div className="flex items-center justify-between pt-1">
        <Button variant="ghost" size="sm" onClick={handleDelete} className="text-destructive hover:text-destructive">
          <Trash2 className="size-4" /> Удалить
        </Button>
        <div className="flex gap-2">
          <Button variant="ghost" size="sm" onClick={onClose}>Отмена</Button>
          <Button size="sm" onClick={handleSave} disabled={!title.trim()}>Сохранить</Button>
        </div>
      </div>
    </>
  )
}
