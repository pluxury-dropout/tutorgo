'use client'

import { useState } from 'react'
import { useParams } from 'next/navigation'
import { useJoinByInvite } from '@/lib/hooks/useWhiteboard'
import { TldrawCanvas } from '@/components/whiteboard/TldrawCanvas'

export default function GuestBoardPage() {
  const { token } = useParams<{ token: string }>()
  const { data: board, isLoading, error } = useJoinByInvite(token)
  const [activePageId, setActivePageId] = useState<string | null>(null)

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
        token={token}
        isGuest={true}
      />
    </div>
  )
}
