'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { coursesApi } from '@/lib/api/courses'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTitle } from '@/components/ui/popover'

interface Props {
  /** Кнопка, у которой всплывает поповер; null — закрыт. */
  anchor: Element | null
  courseId: string
  onClose: () => void
  side?: 'top' | 'bottom' | 'left' | 'right'
  align?: 'start' | 'center' | 'end'
}

// Редактор ДЗ курса (репетитор). Пишется в markdown, ученик видит рендер.
export function HomeworkEditPopover({ anchor, courseId, onClose, side = 'top', align = 'center' }: Props) {
  return (
    <Popover open={!!anchor} onOpenChange={(open) => !open && onClose()}>
      <PopoverContent
        anchor={anchor}
        side={side}
        align={align}
        className="w-[min(30rem,calc(100vw-2rem))]"
      >
        <PopoverTitle className="pr-8">Домашнее задание</PopoverTitle>
        {/* Форма живёт только пока поповер открыт: иначе черновик с прошлого
            раза переживает закрытие — эффект синхронизации с сервером не
            перезапустится, пока не изменится закешированный ответ. */}
        {anchor && <HomeworkForm courseId={courseId} onClose={onClose} />}
      </PopoverContent>
    </Popover>
  )
}

export function HomeworkForm({ courseId, onClose }: { courseId: string; onClose: () => void }) {
  const queryClient = useQueryClient()
  // null — «пользователь ещё не правил», показываем серверный текст. Так
  // ответ запроса подхватывается без эффекта-синхронизатора: он приходит
  // позже монтирования, но правка сразу перебивает его собой.
  const [draft, setDraft] = useState<string | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['course-homework', courseId],
    queryFn: () => coursesApi.getHomework(courseId),
  })

  const text = draft ?? data ?? ''

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
    <>
      <textarea
        className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm font-mono"
        rows={10}
        placeholder="Задание в формате Markdown..."
        value={text}
        onChange={(e) => setDraft(e.target.value)}
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
    </>
  )
}
