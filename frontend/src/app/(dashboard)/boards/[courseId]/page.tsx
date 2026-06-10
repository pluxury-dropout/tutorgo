'use client'

import { useState } from 'react'
import { useParams } from 'next/navigation'
import { useBoardByCourse } from '@/lib/hooks/useWhiteboard'
import { TldrawCanvas } from '@/components/whiteboard/TldrawCanvas'
import { PageSidebar } from '@/components/whiteboard/PageSidebar'
import { BoardToolbar } from '@/components/whiteboard/BoardToolbar'

export default function BoardPage() {
  const { courseId } = useParams<{ courseId: string }>()
  const { data: board, isLoading, error } = useBoardByCourse(courseId)
  const [activePageId, setActivePageId] = useState<string | null>(null)

  if (isLoading)
    return (
      <div className="flex items-center justify-center h-screen text-gray-500">
        Загрузка доски...
      </div>
    )
  if (error || !board)
    return (
      <div className="flex items-center justify-center h-screen text-red-500">
        Ошибка загрузки доски
      </div>
    )

  const currentPageId = activePageId ?? board.pages[0]?.id ?? null
  const currentPage = board.pages.find((p) => p.id === currentPageId) ?? null

  return (
    <div className="flex flex-col h-screen">
      <BoardToolbar boardId={board.id} />
      <div className="flex flex-1 overflow-hidden">
        <PageSidebar
          boardId={board.id}
          courseId={courseId}
          pages={board.pages}
          activePageId={currentPageId ?? ''}
          onSelect={setActivePageId}
        />
        <div className="flex-1">
          <TldrawCanvas page={currentPage} boardId={board.id} />
        </div>
      </div>
    </div>
  )
}
