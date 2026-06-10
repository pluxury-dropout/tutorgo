'use client'

import { useState } from 'react'
import type { BoardPage } from '@/types/api'
import { useCreatePage, useDeletePage } from '@/lib/hooks/useWhiteboard'

interface Props {
  boardId: string
  pages: BoardPage[]
  activePageId: string
  onSelect: (pageId: string) => void
}

export function PageSidebar({ boardId, pages, activePageId, onSelect }: Props) {
  const [newTitle, setNewTitle] = useState('')
  const [adding, setAdding] = useState(false)
  const createPage = useCreatePage(boardId)
  const deletePage = useDeletePage(boardId)

  const handleAdd = async () => {
    if (!newTitle.trim()) return
    await createPage.mutateAsync(newTitle.trim())
    setNewTitle('')
    setAdding(false)
  }

  return (
    <div className="w-48 flex-shrink-0 border-r border-gray-200 bg-gray-50 flex flex-col h-full">
      <div className="p-3 font-medium text-sm text-gray-600 border-b">Страницы</div>
      <div className="flex-1 overflow-y-auto">
        {pages.map((p) => (
          <div
            key={p.id}
            onClick={() => onSelect(p.id)}
            className={`group flex items-center justify-between px-3 py-2 text-sm cursor-pointer hover:bg-gray-100 ${
              p.id === activePageId ? 'bg-blue-50 text-blue-700 font-medium' : ''
            }`}
          >
            <span className="truncate">{p.title}</span>
            {pages.length > 1 && (
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  deletePage.mutate(p.id)
                }}
                className="opacity-0 group-hover:opacity-100 text-gray-400 hover:text-red-500 ml-1"
              >
                ×
              </button>
            )}
          </div>
        ))}
      </div>
      <div className="p-2 border-t">
        {adding ? (
          <div className="flex gap-1">
            <input
              autoFocus
              className="flex-1 border rounded px-2 py-1 text-xs"
              value={newTitle}
              onChange={(e) => setNewTitle(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleAdd()
                if (e.key === 'Escape') setAdding(false)
              }}
              placeholder="Название..."
            />
            <button onClick={handleAdd} className="text-xs bg-blue-500 text-white px-2 rounded">
              +
            </button>
          </div>
        ) : (
          <button
            onClick={() => {
              setAdding(true)
              setNewTitle('')
            }}
            className="w-full text-xs text-gray-500 hover:text-gray-700 py-1"
          >
            + Страница
          </button>
        )}
      </div>
    </div>
  )
}
