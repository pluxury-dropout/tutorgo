'use client'

import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { Lesson } from '@/types/api'
import { SeriesUpdateInput } from '@/lib/api/lessons'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

interface SeriesDialogProps {
  lesson:   Lesson
  open:     boolean
  onClose:  () => void
  onDelete: (seriesId: string, fromDate?: string) => Promise<void>
  onUpdate: (seriesId: string, data: SeriesUpdateInput) => Promise<void>
}

export function SeriesDialog({ lesson, open, onClose, onDelete, onUpdate }: SeriesDialogProps) {
  const [scope, setScope]         = useState<'all' | 'from'>('all')
  const [newTime, setNewTime]     = useState('')
  const [duration, setDuration]   = useState('')
  const [notes, setNotes]         = useState('')
  const [saving, setSaving]       = useState(false)
  const [deleting, setDeleting]   = useState(false)

  useEffect(() => {
    if (open) {
      setScope('all')
      setNewTime('')
      setDuration('')
      setNotes('')
    }
  }, [open])

  const fromDate = scope === 'from' ? lesson.scheduled_at : undefined

  const lessonDate = new Date(lesson.scheduled_at).toLocaleString('ru-RU', {
    day: '2-digit', month: 'long', hour: '2-digit', minute: '2-digit',
  })

  function localTimeToUTC(hhmm: string): string {
    const [h, m] = hhmm.split(':').map(Number)
    const d = new Date()
    d.setHours(h, m, 0, 0)
    return `${String(d.getUTCHours()).padStart(2, '0')}:${String(d.getUTCMinutes()).padStart(2, '0')}`
  }

  async function handleUpdate() {
    const data: SeriesUpdateInput = {}
    if (newTime)    data.new_time         = localTimeToUTC(newTime)
    if (duration)   data.duration_minutes = Number(duration)
    if (notes)      data.notes            = notes
    if (fromDate)   data.from_date        = fromDate

    if (!data.new_time && !data.duration_minutes && !data.notes) {
      toast.error('Укажите хотя бы одно поле для изменения')
      return
    }

    setSaving(true)
    try {
      await onUpdate(lesson.series_id!, data)
      toast.success('Серия обновлена')
      onClose()
    } catch {
      toast.error('Ошибка обновления серии')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete() {
    const label = scope === 'all' ? 'все уроки серии' : 'уроки серии с этого урока'
    if (!confirm(`Удалить ${label}?`)) return

    setDeleting(true)
    try {
      await onDelete(lesson.series_id!, fromDate)
      toast.success('Уроки удалены')
      onClose()
    } catch {
      toast.error('Ошибка удаления серии')
    } finally {
      setDeleting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>Управление серией</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 pt-1">
          <div className="space-y-1.5">
            <Label>Применить к</Label>
            <div className="space-y-1.5">
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input type="radio" name="scope" value="all"
                  checked={scope === 'all'} onChange={() => setScope('all')} />
                Все уроки серии
              </label>
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input type="radio" name="scope" value="from"
                  checked={scope === 'from'} onChange={() => setScope('from')} />
                С этого урока ({lessonDate})
              </label>
            </div>
          </div>

          <div className="border-t pt-3 space-y-3">
            <p className="text-xs text-muted-foreground">Оставьте поля пустыми, чтобы не менять их</p>

            <div className="space-y-1.5">
              <Label htmlFor="series-time">Новое время</Label>
              <Input id="series-time" type="time"
                value={newTime} onChange={(e) => setNewTime(e.target.value)} />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="series-duration">Длительность (мин)</Label>
              <Input id="series-duration" type="number" min={1} placeholder="без изменений"
                value={duration} onChange={(e) => setDuration(e.target.value)} />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="series-notes">Заметки</Label>
              <Input id="series-notes" placeholder="без изменений"
                value={notes} onChange={(e) => setNotes(e.target.value)} />
            </div>
          </div>

          <div className="flex justify-between gap-2 pt-1">
            <Button variant="destructive" size="sm" onClick={handleDelete} disabled={deleting || saving}>
              {deleting ? 'Удаление...' : 'Удалить серию'}
            </Button>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={onClose}>Отмена</Button>
              <Button size="sm" onClick={handleUpdate} disabled={saving || deleting}>
                {saving ? 'Сохранение...' : 'Сохранить'}
              </Button>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
