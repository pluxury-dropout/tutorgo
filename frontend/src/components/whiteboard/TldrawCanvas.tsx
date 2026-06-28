'use client'

import { useEffect, useMemo, useRef, useState } from 'react'
import { Tldraw, type Editor, type TldrawOptions, AssetRecordType } from '@tldraw/tldraw'
import '@tldraw/tldraw/tldraw.css'
import { toast } from 'sonner'
import { useWhiteboardSync } from './useWhiteboardSync'
import { BoardContextProvider } from './BoardContext'
import { PdfRangeDialog } from './PdfRangeDialog'
import type { BoardPage } from '@/types/api'
import type { PDFDocumentProxy } from 'pdfjs-dist'
import { loadPdf, renderPages } from '@/lib/pdf'
import { whiteboardApi, BASE_URL } from '@/lib/api/whiteboard'

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

  const pdfRef = useRef<PDFDocumentProxy | null>(null)
  const [pdfDialog, setPdfDialog] = useState<{ numPages: number; point: { x: number; y: number } } | null>(null)
  const [pdfProgress, setPdfProgress] = useState<{ done: number; total: number } | null>(null)

  // Stable ref so pdfOptions memo can call the latest version without re-creating the editor.
  const openPdfRef = useRef<(file: File, point: { x: number; y: number }) => void>(() => {})

  // Keep openPdfRef.current fresh after every render (effect, not render body — avoids react-hooks/refs).
  useEffect(() => {
    openPdfRef.current = async (file, point) => {
      try {
        const pdf = await loadPdf(file)
        pdfRef.current = pdf
        setPdfDialog({ numPages: pdf.numPages, point })
      } catch {
        toast.error('Не удалось открыть PDF')
      }
    }
  })

  // experimental__onDropOnCanvas вызывается ДО дефолтной обработки drop.
  // PDF → return true (блокируем tldraw) + открываем диалог; иначе return false
  // → tldraw вставляет картинки как обычно. Стабильно через useMemo([]).
  const pdfOptions = useMemo<Partial<TldrawOptions>>(
    () => ({
      experimental__onDropOnCanvas: ({ event, point }) => {
        const file = Array.from(event.dataTransfer?.files ?? []).find(
          (f) => f.type === 'application/pdf'
        )
        if (!file) return false
        openPdfRef.current(file, { x: point.x, y: point.y })
        return true
      },
    }),
    []
  )

  const handlePdfConfirm = async (from: number, to: number) => {
    const pdf = pdfRef.current
    const editor = editorRef.current
    const origin = pdfDialog?.point
    if (!pdf || !editor || !origin) return
    setPdfProgress({ done: 0, total: to - from + 1 })
    try {
      const pages = await renderPages(pdf, from, to, (done, total) =>
        setPdfProgress({ done, total })
      )
      let x = origin.x
      const y = origin.y
      for (let i = 0; i < pages.length; i++) {
        const p = pages[i]
        const pngFile = new File([p.blob], `page-${from + i}.png`, { type: 'image/png' })
        let result
        try {
          result = await whiteboardApi.uploadAsset(boardId, pngFile)
        } catch {
          toast.error(`Не удалось загрузить страницу ${from + i}`)
          break
        }
        const assetId = AssetRecordType.createId()
        editor.createAssets([
          {
            id: assetId,
            type: 'image',
            typeName: 'asset',
            props: {
              src: `${BASE_URL}${result.url}`,
              w: p.width,
              h: p.height,
              mimeType: 'image/png',
              name: `page-${from + i}`,
              isAnimated: false,
            },
            meta: {},
          },
        ])
        editor.createShape({ type: 'image', x, y, props: { assetId, w: p.width, h: p.height } })
        x += p.width // встык по горизонтали
      }
    } catch {
      toast.error('Не удалось обработать PDF')
    } finally {
      void pdf.cleanup() // освобождаем ресурсы документа pdfjs
      pdfRef.current = null
      setPdfProgress(null)
      setPdfDialog(null)
    }
  }

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
          options={pdfOptions}
          // tldraw hard-blocks the editor on production (https, non-localhost)
          // ~5s after mount unless a license key is provided. Read from
          // NEXT_PUBLIC_TLDRAW_LICENSE_KEY (set it in the deploy env).
          licenseKey={process.env.NEXT_PUBLIC_TLDRAW_LICENSE_KEY}
          onMount={(editor) => {
            editorRef.current = editor
          }}
          colorScheme="system"
        />
        {pdfDialog && (
          <PdfRangeDialog
            open
            numPages={pdfDialog.numPages}
            progress={pdfProgress}
            onConfirm={handlePdfConfirm}
            onCancel={() => {
              pdfRef.current = null
              setPdfDialog(null)
            }}
          />
        )}
      </div>
    </BoardContextProvider>
  )
}
