'use client'

import { useState } from 'react'
import { useParams } from 'next/navigation'
import dynamic from 'next/dynamic'
import { useQuery } from '@tanstack/react-query'
import { useJoinByInvite } from '@/lib/hooks/useWhiteboard'
import { studentApi } from '@/lib/api/student'

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
  const [activePageId, setActivePageId] = useState<string | null>(null)
  // Ссылку может открыть залогиненный ученик или анонимный гость. Профиль есть
  // только у первого; у второго me() даёт 401 (retry: false) → имя останется
  // undefined и хук покажет «Гость».
  const { data: me } = useQuery({
    queryKey: ['student', 'me'],
    queryFn: () => studentApi.me(),
    retry: false,
  })
  const displayName =
    [me?.first_name, me?.last_name].filter(Boolean).join(' ') || undefined

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
      <ExcalidrawCanvas
        page={currentPage}
        boardId={board.id}
        pages={board.pages}
        activePageId={currentPageId ?? ''}
        onSelectPage={setActivePageId}
        token={token}
        isGuest={true}
        displayName={displayName}
      />
    </div>
  )
}
