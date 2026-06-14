'use client'

import { useState } from 'react'
import { toast } from 'sonner'
import { Plus, X } from 'lucide-react'
import { useCreatePage, useDeletePage } from '@/lib/hooks/useWhiteboard'
import { useBoardContext } from './BoardContext'

export function BoardPageMenu() {
  const { boardId, courseId, pages, activePageId, onSelectPage, isGuest } = useBoardContext()
  const [newTitle, setNewTitle] = useState('')
  const [adding, setAdding] = useState(false)
  const createPage = useCreatePage(boardId, courseId)
  const deletePage = useDeletePage(boardId, courseId)

  const handleAdd = async () => {
    if (!newTitle.trim()) return
    try {
      const created = await createPage.mutateAsync(newTitle.trim())
      setNewTitle('')
      setAdding(false)
      onSelectPage(created.id)
    } catch {
      toast.error('Не удалось создать страницу')
    }
  }

  const handleDelete = async (pageId: string, e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      await deletePage.mutateAsync(pageId)
    } catch {
      toast.error('Не удалось удалить страницу')
    }
  }

  return (
    <div
      className="tlui-page-menu__wrapper"
      style={{
        display: 'flex',
        flexDirection: 'column',
        background: 'var(--color-panel)',
        border: '1px solid var(--color-divider)',
        borderRadius: 'var(--radius-4)',
        minWidth: 180,
        maxHeight: 320,
        overflow: 'hidden',
        boxShadow: 'var(--shadow-2)',
      }}
    >
      <div
        style={{
          overflowY: 'auto',
          flex: 1,
        }}
      >
        {pages.map((p) => (
          <div
            key={p.id}
            onClick={() => onSelectPage(p.id)}
            className="tlui-button tlui-button__normal"
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              width: '100%',
              borderRadius: 0,
              fontWeight: p.id === activePageId ? 600 : 400,
              background:
                p.id === activePageId ? 'var(--color-muted-1)' : undefined,
              cursor: 'pointer',
            }}
          >
            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {p.title}
            </span>
            {!isGuest && pages.length > 1 && (
              <button
                onClick={(e) => handleDelete(p.id, e)}
                className="tlui-button tlui-button__icon"
                style={{ marginLeft: 4, flexShrink: 0, opacity: 0.5 }}
                aria-label="Удалить страницу"
              >
                <X size={12} />
              </button>
            )}
          </div>
        ))}
      </div>

      {!isGuest && (
        <div style={{ borderTop: '1px solid var(--color-divider)', padding: 6 }}>
          {adding ? (
            <div style={{ display: 'flex', gap: 4 }}>
              <input
                autoFocus
                className="tlui-input"
                value={newTitle}
                onChange={(e) => setNewTitle(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleAdd()
                  if (e.key === 'Escape') setAdding(false)
                }}
                placeholder="Название..."
                style={{ flex: 1, fontSize: 12 }}
              />
              <button
                onClick={handleAdd}
                className="tlui-button tlui-button__icon"
                aria-label="Добавить"
              >
                <Plus size={14} />
              </button>
            </div>
          ) : (
            <button
              onClick={() => { setAdding(true); setNewTitle('') }}
              className="tlui-button tlui-button__normal"
              style={{ width: '100%', gap: 4, fontSize: 12 }}
            >
              <Plus size={14} /> Страница
            </button>
          )}
        </div>
      )}
    </div>
  )
}
