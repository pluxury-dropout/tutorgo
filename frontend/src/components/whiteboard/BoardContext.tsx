'use client'

import { createContext, useContext } from 'react'
import type { BoardPage } from '@/types/api'

interface BoardContextValue {
  boardId: string
  courseId?: string
  pages: BoardPage[]
  activePageId: string
  onSelectPage: (id: string) => void
  isGuest: boolean
}

const BoardContext = createContext<BoardContextValue | null>(null)

export function BoardContextProvider({
  children,
  value,
}: {
  children: React.ReactNode
  value: BoardContextValue
}) {
  return <BoardContext.Provider value={value}>{children}</BoardContext.Provider>
}

export function useBoardContext(): BoardContextValue {
  const ctx = useContext(BoardContext)
  if (!ctx) throw new Error('useBoardContext must be used inside BoardContextProvider')
  return ctx
}
