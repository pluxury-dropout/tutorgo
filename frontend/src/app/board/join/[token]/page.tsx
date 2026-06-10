'use client'

import { useState } from 'react'
import { useJoinByInvite } from '@/lib/hooks/useWhiteboard'
import { TldrawCanvas } from '@/components/whiteboard/TldrawCanvas'
import { PageSidebar } from '@/components/whiteboard/PageSidebar'
import { BoardToolbar } from '@/components/whiteboard/BoardToolbar'

interface Props {
  params: { token: string }
}

export default function GuestBoardPage({ params }: Props) {
  const { data: board, isLoading, error } = useJoinByInvite(params.token)
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
    <div className="flex flex-col h-screen">
      <BoardToolbar boardId={board.id} isGuest />
      <div className="flex flex-1 overflow-hidden">
        <PageSidebar
          boardId={board.id}
          pages={board.pages}
          activePageId={currentPageId ?? ''}
          onSelect={setActivePageId}
        />
        <div className="flex-1">
          <TldrawCanvas page={currentPage} boardId={board.id} token={params.token} />
        </div>
      </div>
    </div>
  )
}
