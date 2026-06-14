'use client'

import { useState } from 'react'
import { useParams } from 'next/navigation'
import { useBoardByCourse } from '@/lib/hooks/useWhiteboard'
import { TldrawCanvas } from '@/components/whiteboard/TldrawCanvas'

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
    <div className="h-screen">
      <TldrawCanvas
        page={currentPage}
        boardId={board.id}
        pages={board.pages}
        activePageId={currentPageId ?? ''}
        onSelectPage={setActivePageId}
        courseId={courseId}
        isGuest={false}
      />
    </div>
  )
}
