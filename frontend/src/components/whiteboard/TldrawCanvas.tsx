'use client'

import { useEffect, useRef } from 'react'
import { Tldraw, type Editor, AssetRecordType } from '@tldraw/tldraw'
import '@tldraw/tldraw/tldraw.css'
import { useWhiteboardSync } from './useWhiteboardSync'
import type { BoardPage } from '@/types/api'

interface Props {
  page: BoardPage | null
  token?: string
  boardId: string
}

export function TldrawCanvas({ page, token, boardId: _boardId }: Props) {
  const { store, status, sendCursor } = useWhiteboardSync(page, token)
  const editorRef = useRef<Editor | null>(null)

  // Handle wb:insert-image custom events dispatched by BoardToolbar
  useEffect(() => {
    const handler = (e: Event) => {
      const { url, width = 800, height = 600 } = (e as CustomEvent<{ url: string; width?: number; height?: number }>).detail
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
        onMount={(editor) => {
          editorRef.current = editor
        }}
        colorScheme="system"
      />
    </div>
  )
}
