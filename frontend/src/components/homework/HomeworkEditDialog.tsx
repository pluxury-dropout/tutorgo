'use client'

import { useEffect, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { coursesApi } from '@/lib/api/courses'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

// Редактор ДЗ курса (репетитор). Пишется в markdown, ученик видит рендер.
export function HomeworkEditDialog({
  open,
  onClose,
  courseId,
}: {
  open: boolean
  onClose: () => void
  courseId: string
}) {
  const queryClient = useQueryClient()
  const [text, setText] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['course-homework', courseId],
    queryFn: () => coursesApi.getHomework(courseId),
    enabled: open,
  })

  useEffect(() => {
    if (data != null) setText(data)
  }, [data])

  const save = useMutation({
    mutationFn: () => coursesApi.updateHomework(courseId, text),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['course-homework', courseId] })
      toast.success('Домашнее задание сохранено')
      onClose()
    },
    onError: () => toast.error('Не удалось сохранить'),
  })

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Домашнее задание</DialogTitle>
        </DialogHeader>
        <textarea
          className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm font-mono"
          rows={12}
          placeholder="Задание в формате Markdown..."
          value={text}
          onChange={(e) => setText(e.target.value)}
          disabled={isLoading}
        />
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={onClose}>
            Отмена
          </Button>
          <Button onClick={() => save.mutate()} disabled={save.isPending || isLoading}>
            Сохранить
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
