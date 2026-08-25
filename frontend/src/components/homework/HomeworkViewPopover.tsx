'use client'

import { useQuery } from '@tanstack/react-query'

import { studentApi } from '@/lib/api/student'
import { Markdown } from '@/components/common/Markdown'
import { Popover, PopoverContent, PopoverTitle } from '@/components/ui/popover'

// Просмотр ДЗ учеником в звонке. Гость не знает courseId урока, поэтому
// показываем все его ДЗ (обычно один-два курса).
export function HomeworkViewPopover({ anchor, onClose }: { anchor: Element | null; onClose: () => void }) {
  return (
    <Popover open={!!anchor} onOpenChange={(open) => !open && onClose()}>
      <PopoverContent
        anchor={anchor}
        side="top"
        align="center"
        className="w-[min(26rem,calc(100vw-2rem))]"
      >
        <PopoverTitle className="pr-8">Домашнее задание</PopoverTitle>
        {anchor && <HomeworkList />}
      </PopoverContent>
    </Popover>
  )
}

function HomeworkList() {
  const { data: homework } = useQuery({
    queryKey: ['student-homework'],
    queryFn: () => studentApi.homework(),
  })

  if (!homework || homework.length === 0) {
    return <p className="text-sm text-muted-foreground">Домашнего задания пока нет</p>
  }

  return (
    <div className="space-y-4">
      {homework.map((h) => (
        <div key={h.course_id}>
          <div className="text-sm font-semibold mb-1">{h.subject}</div>
          <Markdown>{h.homework}</Markdown>
        </div>
      ))}
    </div>
  )
}
