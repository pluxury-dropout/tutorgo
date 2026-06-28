'use client'

import { useState } from 'react'
import { parseRange } from '@/lib/pdfRange'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

interface Props {
  open: boolean
  numPages: number
  progress: { done: number; total: number } | null
  onConfirm: (from: number, to: number) => void
  onCancel: () => void
}

export function PdfRangeDialog({ open, numPages, progress, onConfirm, onCancel }: Props) {
  const [value, setValue] = useState('')

  const handleConfirm = () => {
    if (progress) return
    const [from, to] = parseRange(value, numPages)
    onConfirm(from, to)
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o && !progress) onCancel() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Вставить страницы PDF</DialogTitle>
        </DialogHeader>
        {progress ? (
          <p className="text-sm text-muted-foreground">
            Конвертация: {progress.done} / {progress.total}…
          </p>
        ) : (
          <div className="flex flex-col gap-2">
            <p className="text-sm text-muted-foreground">Всего страниц: {numPages}</p>
            <Input
              autoFocus
              placeholder={`Например: 5-8 или 5 (пусто — все ${numPages})`}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleConfirm() }}
            />
          </div>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={onCancel} disabled={!!progress}>
            Отмена
          </Button>
          <Button onClick={handleConfirm} disabled={!!progress}>
            Вставить
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
