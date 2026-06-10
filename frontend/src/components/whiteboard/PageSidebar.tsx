'use client'

import { useState } from 'react'
import { Plus, X } from 'lucide-react'
import { toast } from 'sonner'
import type { BoardPage } from '@/types/api'
import { useCreatePage, useDeletePage } from '@/lib/hooks/useWhiteboard'

interface Props {
  boardId: string
  courseId?: string
  pages: BoardPage[]
  activePageId: string
  onSelect: (pageId: string) => void
  isGuest?: boolean
}

export function PageSidebar({
  boardId,
  courseId,
  pages,
  activePageId,
  onSelect,
  isGuest = false,
}: Props) {
  const [newTitle, setNewTitle] = useState('')
  const [adding, setAdding] = useState(false)
  const createPage = useCreatePage(boardId, courseId)
  const deletePage = useDeletePage(boardId, courseId)

  const handleAdd = async () => {
    if (!newTitle.trim()) return
    try {
      await createPage.mutateAsync(newTitle.trim())
      setNewTitle('')
      setAdding(false)
    } catch {
      toast.error('Не удалось создать страницу')
    }
  }

  const handleDelete = async (pageId: string) => {
    try {
      await deletePage.mutateAsync(pageId)
    } catch {
      toast.error('Не удалось удалить страницу')
    }
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
            {!isGuest && pages.length > 1 && (
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  handleDelete(p.id)
                }}
                className="opacity-0 group-hover:opacity-100 text-gray-400 hover:text-red-500 ml-1"
                aria-label="Удалить страницу"
              >
                <X size={14} />
              </button>
            )}
          </div>
        ))}
      </div>
      {!isGuest && (
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
              <button
                onClick={handleAdd}
                className="flex items-center justify-center bg-blue-500 text-white px-2 rounded"
                aria-label="Добавить страницу"
              >
                <Plus size={14} />
              </button>
            </div>
          ) : (
            <button
              onClick={() => {
                setAdding(true)
                setNewTitle('')
              }}
              className="flex items-center justify-center gap-1 w-full text-xs text-gray-500 hover:text-gray-700 py-1"
            >
              <Plus size={14} /> Страница
            </button>
          )}
        </div>
      )}
    </div>
  )
}
