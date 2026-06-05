'use client'

import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { Lesson } from '@/types/api'
import { SeriesUpdateInput } from '@/lib/api/lessons'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { TimePicker } from '@/components/ui/time-picker'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

interface SeriesDialogProps {
  lesson:   Lesson
  open:     boolean
  onClose:  () => void
  onDelete: (seriesId: string, fromDate?: string, toDate?: string) => Promise<void>
  onUpdate: (seriesId: string, data: SeriesUpdateInput) => Promise<void>
}


export function SeriesDialog({ lesson, open, onClose, onDelete, onUpdate }: SeriesDialogProps) {
  const [scope, setScope]         = useState<'all' | 'from'>('all')
  const [timeHour, setTimeHour]   = useState('')
  const [timeMin, setTimeMin]     = useState('0')
  const [duration, setDuration]   = useState('')
  const [notes, setNotes]         = useState('')
  const [saving, setSaving]       = useState(false)
  const [deleting, setDeleting]   = useState(false)
  const [toDate, setToDate]       = useState('')

  useEffect(() => {
    if (open) {
      setScope('all')
      setTimeHour('')
      setTimeMin('0')
      setDuration('')
      setNotes('')
      setToDate('')
    }
  }, [open])

  const fromDate = scope === 'from' ? lesson.scheduled_at : undefined
  const toDateISO = toDate ? `${toDate}T23:59:59Z` : undefined

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
    if (timeHour !== '') data.new_time = localTimeToUTC(`${String(Number(timeHour)).padStart(2,'0')}:${String(Number(timeMin)).padStart(2,'0')}`)
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
      await onDelete(lesson.series_id!, fromDate, toDateISO)
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
              {scope === 'from' && (
                <div className="pl-6 space-y-1">
                  <Label className="text-xs text-muted-foreground">По дату (необязательно)</Label>
                  <input
                    type="date"
                    value={toDate}
                    onChange={(e) => setToDate(e.target.value)}
                    className="block w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm"
                  />
                </div>
              )}
            </div>
          </div>

          <div className="border-t pt-3 space-y-3">
            <p className="text-xs text-muted-foreground">Оставьте поля пустыми, чтобы не менять их</p>

            <div className="space-y-1.5">
              <Label>Новое время</Label>
              <TimePicker
                hour={timeHour}
                minute={timeMin}
                onHourChange={setTimeHour}
                onMinuteChange={setTimeMin}
                hourPlaceholder="— ч —"
              />
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
