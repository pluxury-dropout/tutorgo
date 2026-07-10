'use client'

import { useState } from 'react'
import { useParams } from 'next/navigation'
import dynamic from 'next/dynamic'
import { useBoardByCourse } from '@/lib/hooks/useWhiteboard'

// Excalidraw трогает window при инициализации — только клиент, без SSR.
const ExcalidrawCanvas = dynamic(
  () =>
    import('@/components/whiteboard/ExcalidrawCanvas').then(
      (m) => m.ExcalidrawCanvas
    ),
  { ssr: false }
)

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
      <ExcalidrawCanvas
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
