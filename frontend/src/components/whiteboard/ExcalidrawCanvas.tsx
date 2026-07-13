'use client'

import { useRef, useState, useCallback } from 'react'
import {
  Excalidraw,
  convertToExcalidrawElements,
  viewportCoordsToSceneCoords,
  CaptureUpdateAction,
  FONT_FAMILY,
} from '@excalidraw/excalidraw'
import '@excalidraw/excalidraw/index.css'
import './excalidraw-fonts.css'
import { toast } from 'sonner'
import type {
  ExcalidrawImperativeAPI,
  BinaryFileData,
  DataURL,
} from '@excalidraw/excalidraw/types'
import type { FileId } from '@excalidraw/excalidraw/element/types'
import type { PDFDocumentProxy } from 'pdfjs-dist'
import { useExcalidrawSync } from './useExcalidrawSync'
import type { BoardIdentity } from '@/lib/hooks/useBoardDisplayName'
import { blobToDataURL } from './excalidrawSync'
import { BoardContextProvider } from './BoardContext'
import { PdfRangeDialog } from './PdfRangeDialog'
import { loadPdf, renderPage } from '@/lib/pdf'
import { whiteboardApi, BASE_URL } from '@/lib/api/whiteboard'
import type { BoardPage } from '@/types/api'

// Шрифты берём из public/fonts (см. scripts/excalidraw-fonts.mjs). Без этого
// Excalidraw грузит woff2 с unpkg.com. Читается лениво, в момент загрузки
// шрифта, поэтому достаточно выставить до первого рендера доски.
if (typeof window !== 'undefined') {
  ;(window as unknown as { EXCALIDRAW_ASSET_PATH: string }).EXCALIDRAW_ASSET_PATH = '/'
}

// Держать в синхроне с лимитом бэкенда (router.go: multipart 50 МБ).
const MAX_ASSET_BYTES = 50 * 1024 * 1024

const formatMb = (bytes: number) => (bytes / 1024 / 1024).toFixed(1)

interface Props {
  page: BoardPage | null
  token?: string
  boardId: string
  courseId?: string
  isGuest?: boolean
  identity?: BoardIdentity
  /** Отдаёт api наружу — звонку он нужен для follow из панели участников. */
  onApi?: (api: ExcalidrawImperativeAPI) => void
  /** Скрыть встроенный список участников: его роль берёт на себя панель звонка. */
  hideUserList?: boolean
}

export function ExcalidrawCanvas({
  page,
  token,
  boardId,
  courseId,
  isGuest = false,
  identity,
  onApi,
  hideUserList = false,
}: Props) {
  const { status, onApiReady, onChange, sendCursor, broadcastViewport, registerFile } =
    useExcalidrawSync(page, token, identity)
  const apiRef = useRef<ExcalidrawImperativeAPI | null>(null)

  const pdfRef = useRef<PDFDocumentProxy | null>(null)
  const imageInputRef = useRef<HTMLInputElement | null>(null)
  const [pdfDialog, setPdfDialog] = useState<{
    numPages: number
    point: { x: number; y: number }
  } | null>(null)

  // Общий путь вставки картинки: S3 → локальный dataURL → files-карта →
  // image-элемент. Base64 в WS/снапшот не попадает (только URL).
  const insertImageBlob = useCallback(
    async (
      blob: Blob,
      mimeType: string,
      pos: { x: number; y: number },
      size: { w: number; h: number },
      fileName: string
    ) => {
      const api = apiRef.current
      if (!api) return
      const file = new File([blob], fileName, { type: mimeType })
      // Отсекаем до аплоада: иначе пользователь ждёт заливку 100 МБ ради 413.
      if (file.size > MAX_ASSET_BYTES) {
        throw new Error(
          `файл ${formatMb(file.size)} МБ, максимум ${formatMb(MAX_ASSET_BYTES)} МБ`
        )
      }
      const { url } = await whiteboardApi.uploadAsset(boardId, file)
      const fullUrl = `${BASE_URL}${url}`
      const fileId = crypto.randomUUID() as FileId
      const dataURL = (await blobToDataURL(blob)) as DataURL
      api.addFiles([
        {
          id: fileId,
          dataURL,
          mimeType: mimeType as BinaryFileData['mimeType'],
          created: Date.now(),
        },
      ])
      registerFile(fileId, fullUrl, mimeType)
      const [el] = convertToExcalidrawElements([
        {
          type: 'image',
          fileId,
          x: pos.x,
          y: pos.y,
          width: size.w,
          height: size.h,
        },
      ])
      api.updateScene({
        elements: [...api.getSceneElementsIncludingDeleted(), el],
        captureUpdate: CaptureUpdateAction.IMMEDIATELY,
      })
    },
    [boardId, registerFile]
  )

  // Центр вьюпорта в координатах сцены — точка вставки по кнопке.
  const viewportCenter = useCallback(() => {
    const api = apiRef.current
    if (!api) return { x: 0, y: 0 }
    const s = api.getAppState()
    return viewportCoordsToSceneCoords(
      {
        clientX: s.offsetLeft + s.width / 2,
        clientY: s.offsetTop + s.height / 2,
      },
      s
    )
  }, [])

  const insertImageFile = async (file: File) => {
    try {
      const objUrl = URL.createObjectURL(file)
      try {
        const dim = await new Promise<{ w: number; h: number }>(
          (resolve, reject) => {
            const img = new Image()
            img.onload = () =>
              resolve({ w: img.naturalWidth, h: img.naturalHeight })
            img.onerror = reject
            img.src = objUrl
          }
        )
        const c = viewportCenter()
        await insertImageBlob(
          file,
          file.type,
          { x: c.x - dim.w / 2, y: c.y - dim.h / 2 },
          dim,
          file.name
        )
      } finally {
        URL.revokeObjectURL(objUrl)
      }
    } catch (e) {
      // ApiError с бэкенда и локальный Error оба несут .message
      const msg = (e as { message?: string })?.message
      toast.error(
        msg ? `Не удалось вставить картинку: ${msg}` : 'Не удалось вставить картинку'
      )
    }
  }

  // Диалог закрывается сразу, обработка идёт в фоне с прогрессом в toast.
  const handlePdfConfirm = (from: number, to: number) => {
    const pdf = pdfRef.current
    const origin = pdfDialog?.point
    if (!pdf || !origin) return
    // Забираем владение документом: диалог закрыт, следующий drop перезапишет ref.
    pdfRef.current = null
    setPdfDialog(null)

    const toastId = `pdf-${Date.now()}`
    const total = to - from + 1
    // Страницы рендерятся и заливаются по одной, поэтому счётчик общий:
    // «готово N из total» = страница вставлена на доску.
    toast.loading(`PDF: 0 / ${total}…`, { id: toastId })

    void (async () => {
      try {
        let x = origin.x
        for (let i = from; i <= to; i++) {
          const p = await renderPage(pdf, i)
          await insertImageBlob(
            p.blob,
            'image/png',
            { x, y: origin.y },
            { w: p.width, h: p.height },
            `page-${i}.png`
          )
          x += p.width // встык по горизонтали
          toast.loading(`PDF: ${i - from + 1} / ${total}…`, { id: toastId })
        }
        toast.success(`PDF вставлен: ${total} стр.`, { id: toastId })
      } catch (e) {
        const msg = (e as { message?: string })?.message
        toast.error(`Не удалось обработать PDF${msg ? `: ${msg}` : ''}`, {
          id: toastId,
        })
      } finally {
        void pdf.cleanup()
      }
    })()
  }

  // Перехват drop PDF ДО Excalidraw (у него нет хука на drop; capture-фаза
  // обёртки срабатывает раньше). Не-PDF пропускаем — нативная вставка
  // картинок Excalidraw кладёт base64 в files; для брошенных мышкой мелких
  // картинок приемлемо, наша кнопка вставки идёт через S3.
  // ponytail: фоновая догрузка таких base64-файлов в S3 — когда заметим раздутые снапшоты.
  const onDropCapture = (e: React.DragEvent) => {
    if (isGuest) return // гостю вставка недоступна (кнопки скрыты) — не ловим drop
    const file = Array.from(e.dataTransfer?.files ?? []).find(
      (f) => f.type === 'application/pdf'
    )
    if (!file) return
    e.preventDefault()
    e.stopPropagation()
    const api = apiRef.current
    const point = api
      ? viewportCoordsToSceneCoords(
          { clientX: e.clientX, clientY: e.clientY },
          api.getAppState()
        )
      : { x: 0, y: 0 }
    void (async () => {
      try {
        const pdf = await loadPdf(file)
        // Повторный drop поверх незакрытого документа — чистим старый.
        void pdfRef.current?.cleanup()
        pdfRef.current = pdf
        setPdfDialog({ numPages: pdf.numPages, point })
      } catch {
        toast.error('Не удалось открыть PDF')
      }
    })()
  }

  return (
    <BoardContextProvider value={{ boardId, courseId, isGuest }}>
      <div
        className={`relative w-full h-full${hideUserList ? ' board-hide-userlist' : ''}`}
        onDropCapture={onDropCapture}
      >
        {status === 'disconnected' && (
          <div className="absolute top-2 left-1/2 -translate-x-1/2 z-10 bg-yellow-100 border border-yellow-300 text-yellow-800 text-sm px-3 py-1 rounded-full">
            Переподключение...
          </div>
        )}
        <Excalidraw
          key={page?.id ?? 'empty'}
          langCode="ru-RU"
          // Дефолт — Nunito вместо Excalifont. Только appState: элементы
          // приезжают по WS, initialData их не трогает.
          initialData={{ appState: { currentItemFontFamily: FONT_FAMILY.Nunito } }}
          excalidrawAPI={(api) => {
            apiRef.current = api
            onApiReady(api)
            onApi?.(api)
          }}
          onChange={onChange}
          onPointerUpdate={(p) => sendCursor(p.pointer.x, p.pointer.y)}
          onScrollChange={() => broadcastViewport()}
          renderTopRightUI={() => (
            <div
              data-board-ui
              style={{ position: 'relative', display: 'flex', gap: 4 }}
            >
              {/* Вставка картинки через S3 (нативный image-инструмент
                  Excalidraw отключён — он кладёт base64 в снапшот). */}
              {!isGuest && (
                <button
                  title="Вставить картинку"
                  onClick={() => imageInputRef.current?.click()}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    border: 'none',
                    background: 'transparent',
                    borderRadius: 8,
                    cursor: 'pointer',
                    padding: '6px 8px',
                  }}
                >
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.8"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <rect x="3" y="3" width="18" height="18" rx="3" />
                    <circle cx="8.5" cy="9" r="1.6" />
                    <path d="M21 16l-5-5L5 21" />
                  </svg>
                </button>
              )}
            </div>
          )}
          UIOptions={{
            canvasActions: {
              // Экспорт/сохранение файлов скрываем: персист у нас свой (WS).
              export: false,
              loadScene: false,
              saveToActiveFile: false,
            },
            // Нативный image-инструмент кладёт base64 в files → раздувает
            // снапшот и упирается в 512КБ WS-лимит. Вся вставка — через
            // нашу кнопку (S3). Проверено: UIOptions.tools.image есть в 0.18.
            tools: { image: false },
          }}
        />
        {/* Кнопка вставки картинки: скрытый file-input, гостю недоступна */}
        {!isGuest && (
          <input
            ref={imageInputRef}
            type="file"
            accept="image/*"
            style={{ display: 'none' }}
            onChange={(e) => {
              const f = e.target.files?.[0]
              if (f) void insertImageFile(f)
              e.target.value = ''
            }}
          />
        )}
        {pdfDialog && (
          <PdfRangeDialog
            open
            numPages={pdfDialog.numPages}
            onConfirm={handlePdfConfirm}
            onCancel={() => {
              void pdfRef.current?.cleanup()
              pdfRef.current = null
              setPdfDialog(null)
            }}
          />
        )}
      </div>
    </BoardContextProvider>
  )
}
