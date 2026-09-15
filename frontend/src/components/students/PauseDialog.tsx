'use client'

import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { useCreatePause } from '@/lib/hooks/useStudents'
import type { ApiError } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

const today = () => new Date().toISOString().slice(0, 10)

interface PauseDialogProps {
  open:        boolean
  onClose:     () => void
  studentId:   string
  studentName: string
}

/** Заморозка «с даты по дату» (спека 2026-09-06, п. 6.9). Работает и задним
 *  числом: «он же болел всю прошлую неделю» — то же действие. */
export function PauseDialog({ open, onClose, studentId, studentName }: PauseDialogProps) {
  const createPause = useCreatePause(studentId)
  const [startsOn, setStartsOn] = useState('')
  const [endsOn, setEndsOn]     = useState('')
  const [reason, setReason]     = useState('')

  useEffect(() => {
    if (open) {
      const t = today()
      setStartsOn(t)
      setEndsOn(t)
      setReason('')
    }
  }, [open])

  // Даты ISO (YYYY-MM-DD) сравниваются строками.
  const reversed = !!startsOn && !!endsOn && endsOn < startsOn
  const invalid  = !startsOn || !endsOn || reversed

  async function submit() {
    if (invalid) return
    try {
      await createPause.mutateAsync({ starts_on: startsOn, ends_on: endsOn, reason })
      toast.success('Ученик заморожен')
      onClose()
    } catch (err) {
      toast.error((err as ApiError).message ?? 'Не удалось заморозить')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>Заморозить — {studentName}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 pt-2">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="pause-starts">С</Label>
              <Input id="pause-starts" type="date" value={startsOn} onChange={(e) => setStartsOn(e.target.value)} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="pause-ends">По</Label>
              <Input id="pause-ends" type="date" value={endsOn} onChange={(e) => setEndsOn(e.target.value)} />
            </div>
          </div>
          {reversed && <p className="text-xs text-destructive">Дата окончания раньше начала</p>}

          <div className="space-y-1.5">
            <Label htmlFor="pause-reason">Причина</Label>
            <Input
              id="pause-reason"
              placeholder="уехал, болеет, сессия"
              maxLength={500}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
            />
          </div>

          <p className="text-xs text-muted-foreground">
            Уроки этого периода не спишутся с оплаты — ни индивидуальные, ни в группах.
            Запланированные индивидуальные уроки отменятся, а их серия продлится на столько же.
            Расписание групп не меняется.
          </p>

          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={onClose}>Отмена</Button>
            <Button type="button" onClick={submit} disabled={invalid || createPause.isPending}>
              {createPause.isPending ? 'Сохранение...' : 'Заморозить'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
