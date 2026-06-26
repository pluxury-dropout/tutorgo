'use client'

import { useEffect, useRef } from 'react'
import { Tldraw, type Editor, AssetRecordType } from '@tldraw/tldraw'
import '@tldraw/tldraw/tldraw.css'
import { useWhiteboardSync } from './useWhiteboardSync'
import { BoardContextProvider } from './BoardContext'
import type { BoardPage } from '@/types/api'

interface Props {
  page: BoardPage | null
  token?: string
  boardId: string
  pages: BoardPage[]
  activePageId: string
  onSelectPage: (id: string) => void
  courseId?: string
  isGuest?: boolean
}

export function TldrawCanvas({
  page,
  token,
  boardId,
  pages,
  activePageId,
  onSelectPage,
  courseId,
  isGuest = false,
}: Props) {
  const { store, status, sendCursor } = useWhiteboardSync(page, token)
  const editorRef = useRef<Editor | null>(null)

  useEffect(() => {
    const handler = (e: Event) => {
      const { url, width = 800, height = 600 } = (
        e as CustomEvent<{ url: string; width?: number; height?: number }>
      ).detail
      const editor = editorRef.current
      if (!editor) return

      const assetId = AssetRecordType.createId()
      editor.createAssets([
        {
          id: assetId,
          type: 'image',
          typeName: 'asset',
          props: {
            src: url,
            w: width,
            h: height,
            mimeType: 'image/png',
            name: 'image',
            isAnimated: false,
          },
          meta: {},
        },
      ])
      editor.createShape({ type: 'image', props: { assetId, w: width, h: height } })
    }
    window.addEventListener('wb:insert-image', handler)
    return () => window.removeEventListener('wb:insert-image', handler)
  }, [])

  return (
    <BoardContextProvider
      value={{ boardId, courseId, pages, activePageId, onSelectPage, isGuest }}
    >
      <div
        className="relative w-full h-full"
        onPointerMove={() => {
          const editor = editorRef.current
          if (!editor) return
          const pt = editor.inputs.currentPagePoint
          sendCursor(pt.x, pt.y)
        }}
      >
        {status === 'disconnected' && (
          <div className="absolute top-2 left-1/2 -translate-x-1/2 z-10 bg-yellow-100 border border-yellow-300 text-yellow-800 text-sm px-3 py-1 rounded-full">
            Переподключение...
          </div>
        )}
        <Tldraw
          key={page?.id ?? 'empty'}
          store={store}
          // Don't block the whole UI on font loading — tldraw hides .tlui-layout
          // while loadRequiredFontsForCurrentPage() is pending, and on prod that
          // request to tldraw's CDN can hang, leaving the board with no toolbar.
          options={{ maxFontsToLoadBeforeRender: 0 }}
          onMount={(editor) => {
            editorRef.current = editor
            // TEMP diag: log key state every 2s to catch what changes when the
            // tldraw UI layer vanishes a few seconds in. Remove once root-caused.
            const w = window as unknown as { __tlEditor?: Editor; __tlDiag?: number }
            w.__tlEditor = editor
            if (w.__tlDiag) clearInterval(w.__tlDiag)
            let n = 0
            w.__tlDiag = window.setInterval(() => {
              const c = document.querySelector('.tl-container')
              // eslint-disable-next-line no-console
              console.log('[tldiag]', n, {
                uiLayer: !!document.querySelector('.tlui-layout'),
                focus: editor.getInstanceState().isFocusMode,
                readonly: editor.getInstanceState().isReadonly,
                sameEditor: w.__tlEditor === editor,
                records: editor.store.allRecords().length,
                kids: c ? Array.from(c.children).map((x) => String(x.className).slice(0, 32)) : null,
              })
              if (++n > 7) clearInterval(w.__tlDiag)
            }, 2000)
          }}
          colorScheme="system"
        />
      </div>
    </BoardContextProvider>
  )
}
