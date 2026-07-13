'use client'

import { useParams } from 'next/navigation'
import dynamic from 'next/dynamic'
import { useJoinByInvite } from '@/lib/hooks/useWhiteboard'
import { useBoardDisplayName } from '@/lib/hooks/useBoardDisplayName'

// Excalidraw трогает window при инициализации — только клиент, без SSR.
const ExcalidrawCanvas = dynamic(
  () =>
    import('@/components/whiteboard/ExcalidrawCanvas').then(
      (m) => m.ExcalidrawCanvas
    ),
  { ssr: false }
)

export default function GuestBoardPage() {
  const { token } = useParams<{ token: string }>()
  const { data: board, isLoading, error } = useJoinByInvite(token)
  const displayName = useBoardDisplayName('guest')

  if (isLoading)
    return (
      <div className="flex items-center justify-center h-screen text-gray-500">
        Загрузка доски...
      </div>
    )
  if (error || !board)
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-center">
          <p className="text-red-500 text-lg font-medium">Ссылка недействительна</p>
          <p className="text-gray-500 text-sm mt-1">
            Попросите репетитора прислать новую ссылку
          </p>
        </div>
      </div>
    )

  const currentPage = board.pages[0] ?? null

  return (
    <div className="h-screen">
      <ExcalidrawCanvas
        page={currentPage}
        boardId={board.id}
        token={token}
        isGuest={true}
        displayName={displayName}
      />
    </div>
  )
}
