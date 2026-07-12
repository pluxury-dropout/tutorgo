'use client'

import { useQuery } from '@tanstack/react-query'

import { studentApi } from '@/lib/api/student'
import { Markdown } from '@/components/common/Markdown'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

// Просмотр ДЗ учеником в звонке. Гость не знает courseId урока, поэтому
// показываем все его ДЗ (обычно один-два курса).
export function HomeworkViewDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { data: homework } = useQuery({
    queryKey: ['student-homework'],
    queryFn: () => studentApi.homework(),
    enabled: open,
  })

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Домашнее задание</DialogTitle>
        </DialogHeader>
        {!homework || homework.length === 0 ? (
          <p className="text-sm text-muted-foreground">Домашнего задания пока нет</p>
        ) : (
          <div className="space-y-4">
            {homework.map((h) => (
              <div key={h.course_id}>
                <div className="text-sm font-semibold mb-1">{h.subject}</div>
                <Markdown>{h.homework}</Markdown>
              </div>
            ))}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
