'use client'

import { useParams } from 'next/navigation'
import dynamic from 'next/dynamic'
import { useBoardByCourse } from '@/lib/hooks/useWhiteboard'
import { useBoardDisplayName } from '@/lib/hooks/useBoardDisplayName'

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
  const displayName = useBoardDisplayName('tutor')

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

  const currentPage = board.pages[0] ?? null

  return (
    <div className="h-screen">
      <ExcalidrawCanvas
        page={currentPage}
        boardId={board.id}
        courseId={courseId}
        isGuest={false}
        displayName={displayName}
      />
    </div>
  )
}
