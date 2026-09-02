'use client'

import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import type { RecurrenceScope } from '@/types/api'

interface Props {
  open:    boolean
  /** Правка и удаление спрашивают одно и то же, меняются только подписи. */
  action:  'edit' | 'delete'
  onPick:  (scope: RecurrenceScope) => void
  onClose: () => void
}

/** Вопрос об области при правке вхождения серии. Перенос мышью его не
 *  показывает: перетаскивание должно оставаться безопасным жестом. */
export function RecurrenceScopeDialog({ open, action, onPick, onClose }: Props) {
  const verb = action === 'delete' ? 'Удалить' : 'Изменить'

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-xs">
        <DialogHeader>
          <DialogTitle>{verb} повторяющееся занятие</DialogTitle>
        </DialogHeader>

        <div className="flex flex-col gap-2 pt-1">
          <Button variant="outline" onClick={() => onPick('one')}>Только это</Button>
          <Button variant="outline" onClick={() => onPick('following')}>Это и все следующие</Button>
          <Button variant="outline" onClick={() => onPick('all')}>Все</Button>
          <Button variant="ghost" size="sm" onClick={onClose}>Отмена</Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
