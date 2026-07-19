'use client'

import { useRef, useState, useCallback, useEffect } from 'react'
import {
  Excalidraw,
  convertToExcalidrawElements,
  viewportCoordsToSceneCoords,
  newElementWith,
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
import type {
  FileId,
  ExcalidrawElement,
} from '@excalidraw/excalidraw/element/types'
import { useExcalidrawSync } from './useExcalidrawSync'
import type { BoardIdentity } from '@/lib/hooks/useBoardDisplayName'
import { blobToDataURL, imageFromClipboard } from './excalidrawSync'
import { BoardContextProvider } from './BoardContext'
import { PdfRangeDialog } from './PdfRangeDialog'
import { layoutPages } from '@/lib/pdfRange'
import { MaterialsPanel } from './MaterialsPanel'
import { MediaPlayer } from './MediaPlayer'
import { useMediaPlayer } from './useMediaPlayer'
import { useYouTubeSync } from './useYouTubeSync'
import { YouTubeEmbed } from './YouTubeEmbed'
import { parseYouTubeId, type MediaPayload } from './mediaSync'
import { whiteboardApi, BASE_URL } from '@/lib/api/whiteboard'
import { withRetry } from '@/lib/retry'
import type { BoardPage, Material } from '@/types/api'

// Шрифты берём из public/fonts (см. scripts/excalidraw-fonts.mjs). Без этого
// Excalidraw грузит woff2 с unpkg.com. Читается лениво, в момент загрузки
// шрифта, поэтому достаточно выставить до первого рендера доски.
if (typeof window !== 'undefined') {
  ;(window as unknown as { EXCALIDRAW_ASSET_PATH: string }).EXCALIDRAW_ASSET_PATH = '/'
}

// Держать в синхроне с лимитом бэкенда (router.go: multipart 50 МБ).
const MAX_ASSET_BYTES = 50 * 1024 * 1024

const formatMb = (bytes: number) => (bytes / 1024 / 1024).toFixed(1)

// Стартовый размер ролика на доске — 16:9, дальше препод тянет за угол.
const YT_WIDTH = 560
const YT_HEIGHT = 315
const YT_RATIO = YT_HEIGHT / YT_WIDTH
// Меньше — считаем округлением, а не растяжкой: чинить каждый onChange незачем.
const ASPECT_EPS = 0.5

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
  // Плееру нужен sendMedia, а синхронизации — receive плеера: хуки нужны друг
  // другу. Цикл разрываем ref'ом — плеер шлёт через актуальный sendMedia.
  const sendMediaRef = useRef<(p: MediaPayload) => void>(() => {})
  const send = useCallback((p: MediaPayload) => sendMediaRef.current(p), [])
  const player = useMediaPlayer(send)
  const yt = useYouTubeSync(send)

  // Кадр с id — про ролик на доске, без id — про плеер материалов. Запрос
  // состояния (req) касается обоих: у одного может играть аудио, у другого видео.
  const onMedia = useCallback(
    (p: MediaPayload) => {
      if (p.action === 'req') {
        player.receive(p)
        yt.receive(p)
        return
      }
      if (p.id) yt.receive(p)
      else player.receive(p)
    },
    [player, yt]
  )

  // Ожидаемые страницы текущего импорта: file-события с этими id двигают тост.
  const pdfProgressRef = useRef<{ pending: Set<string>; total: number; toastId: string } | null>(null)

  const onPdfFile = useCallback((fileId: string) => {
    const pr = pdfProgressRef.current
    if (!pr || !pr.pending.delete(fileId)) return
    const done = pr.total - pr.pending.size
    if (pr.pending.size === 0) {
      toast.success(`PDF вставлен: ${pr.total} стр.`, { id: pr.toastId })
      pdfProgressRef.current = null
    } else {
      toast.loading(`PDF: ${done} / ${pr.total}…`, { id: pr.toastId })
    }
  }, [])

  const onPdfFailed = useCallback((fileIds: string[]) => {
    const pr = pdfProgressRef.current
    if (pr && fileIds.some((id) => pr.pending.has(id))) {
      toast.error('Не удалось обработать PDF', { id: pr.toastId })
      pdfProgressRef.current = null
    }
  }, [])

  const {
    status,
    onApiReady,
    onChange,
    sendCursor,
    broadcastViewport,
    syncFollowTarget,
    registerFile,
    sendMedia,
  } = useExcalidrawSync(page, token, identity, onMedia, onPdfFile, onPdfFailed)
  useEffect(() => {
    sendMediaRef.current = sendMedia
  }, [sendMedia])

  // Кто подключился позже (перезагрузил вкладку посреди трека) — спрашивает
  // состояние; у кого плеер открыт, тот ответит. Если ни у кого — ответа нет.
  useEffect(() => {
    if (status === 'connected') sendMedia({ action: 'req' })
  }, [status, sendMedia])

  const apiRef = useRef<ExcalidrawImperativeAPI | null>(null)

  // Паспорт preflight'а: PDF уже на сервере, ждём выбора диапазона.
  const pdfImportRef = useRef<{ importId: string; sizes: { w: number; h: number }[] } | null>(null)
  const imageInputRef = useRef<HTMLInputElement | null>(null)
  const [materialsOpen, setMaterialsOpen] = useState(false)
  const [pdfDialog, setPdfDialog] = useState<{
    numPages: number
    point: { x: number; y: number }
  } | null>(null)

  // Заливает картинку и отдаёт её Excalidraw под заранее известным fileId:
  // S3 → локальный dataURL → files-карта. Base64 в WS/снапшот не попадает
  // (только URL). fileId приходит снаружи, чтобы image-элемент можно было
  // положить на доску ДО заливки — addFiles потом сам дорисует его на месте.
  const uploadFile = useCallback(
    async (fileId: FileId, blob: Blob, mimeType: string, fileName: string) => {
      const api = apiRef.current
      if (!api) return
      const file = new File([blob], fileName, { type: mimeType })
      // Отсекаем до аплоада: иначе пользователь ждёт заливку 100 МБ ради 413.
      if (file.size > MAX_ASSET_BYTES) {
        throw new Error(
          `файл ${formatMb(file.size)} МБ, максимум ${formatMb(MAX_ASSET_BYTES)} МБ`
        )
      }
      // Гость (ученик) не имеет tutor JWT — льёт через публичный роут по invite-токену.
      // Повторяем транзиентные сбои: одна моргнувшая заливка из полусотни — это
      // дыра посреди документа, а не «ну не повезло».
      const { url } = await withRetry(() =>
        whiteboardApi.uploadAsset(boardId, file, isGuest ? token : undefined)
      )
      const fullUrl = `${BASE_URL}${url}`
      const dataURL = (await blobToDataURL(blob)) as DataURL
      // addFiles чистит image-кэш и передёргивает сцену — уже стоящий на доске
      // pending-элемент с этим fileId сам сменит плейсхолдер на картинку.
      api.addFiles([
        {
          id: fileId,
          dataURL,
          mimeType: mimeType as BinaryFileData['mimeType'],
          created: Date.now(),
        },
      ])
      registerFile(fileId, fullUrl, mimeType)
    },
    [boardId, registerFile, isGuest, token]
  )

  // Одиночная картинка (вставка из буфера, drop с диска): заливаем, потом кладём
  // на доску. Плейсхолдер тут не нужен — ждать всё равно нечего.
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
      const fileId = crypto.randomUUID() as FileId
      await uploadFile(fileId, blob, mimeType, fileName)
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
    [uploadFile]
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
  // Раскладка и плейсхолдеры встают сразу, страницы проявляются file-событиями
  // по мере того, как сервер их рендерит.
  const handlePdfConfirm = (from: number, to: number) => {
    const imp = pdfImportRef.current
    const origin = pdfDialog?.point
    if (!imp || !origin) return
    pdfImportRef.current = null
    setPdfDialog(null)

    const toastId = `pdf-${Date.now()}`
    toast.loading('PDF: готовим страницы…', { id: toastId })

    void (async () => {
      const api = apiRef.current
      if (!api) return
      try {
        const { pages } = await whiteboardApi.startPdfImport(imp.importId, from, to)
        // Сервер отдаёт пункты PDF — масштаб сцены наш.
        const LAYOUT_SCALE = 1.5
        const placed = layoutPages(
          pages.map((p) => ({ w: p.w * LAYOUT_SCALE, h: p.h * LAYOUT_SCALE })),
          origin
        )
        const els = convertToExcalidrawElements(
          placed.map((l, k) => ({
            type: 'image' as const,
            fileId: pages[k].file_id as FileId,
            x: l.x,
            y: l.y,
            width: l.w,
            height: l.h,
            status: 'pending' as const,
          }))
        )
        api.updateScene({
          elements: [...api.getSceneElementsIncludingDeleted(), ...els],
          captureUpdate: CaptureUpdateAction.IMMEDIATELY,
        })
        // Указатели fileId→URL сразу в files-карту и пирам: раскладка и ссылки
        // переживают закрытие вкладки — сервер дорендерит без нас.
        for (const p of pages) {
          registerFile(p.file_id, `${BASE_URL}${p.url}`, 'image/jpeg')
        }
        pdfProgressRef.current = {
          pending: new Set(pages.map((p) => p.file_id)),
          total: pages.length,
          toastId,
        }
        toast.loading(`PDF: 0 / ${pages.length}…`, { id: toastId })
      } catch (e) {
        const msg = (e as { message?: string })?.message
        toast.error(`Не удалось обработать PDF${msg ? `: ${msg}` : ''}`, { id: toastId })
      }
    })()
  }

  // Выбор файла в библиотеке. Медиа уходит в плеер (звук у обоих), картинка и
  // PDF — на доску теми же путями, что при вставке с диска, остальное качаем.
  // Библиотека хранит только аудио: синхронное прослушивание — единственное, чего
  // препод не сделает у себя на ноуте. PDF и картинки летят на доску прямым drop'ом
  // с диска, хранить их на сервере незачем.
  const handlePickMaterial = (m: Material, url: string) => {
    player.open({ url, mimeType: m.mime_type, name: m.name })
    setMaterialsOpen(false)
  }

  // Видео не храним: препод находит ролик по ходу урока и вставляет ссылкой.
  // Байты идут от Google к ученику мимо нас — ни хранилища, ни трафика.
  // Ролик кладётся на доску embeddable-элементом: возить его по WS и хранить в
  // снапшоте не нужно — это обычный элемент сцены, синк у него общий с фигурами.
  const handleOpenYouTube = () => {
    // ponytail: нативный prompt. Заменить на диалог, когда дойдут руки до дизайна.
    const link = window.prompt('Ссылка на YouTube')
    if (!link) return
    const id = parseYouTubeId(link)
    if (!id) {
      toast.error('Не похоже на ссылку YouTube')
      return
    }
    const api = apiRef.current
    if (!api) return
    const c = viewportCenter()
    // Скелет embeddable convertToExcalidrawElements не принимает (ждёт готовый
    // элемент со всеми полями), а фабрики наружу не выведено. Но embeddable —
    // это _ExcalidrawElementBase + type, ровно как rectangle: собираем из него.
    const [base] = convertToExcalidrawElements([
      {
        type: 'rectangle',
        link: `https://www.youtube.com/watch?v=${id}`,
        x: c.x - YT_WIDTH / 2,
        y: c.y - YT_HEIGHT / 2,
        width: YT_WIDTH,
        height: YT_HEIGHT,
      },
    ])
    const el = { ...base, type: 'embeddable' } as ExcalidrawElement
    api.updateScene({
      elements: [...api.getSceneElementsIncludingDeleted(), el],
      captureUpdate: CaptureUpdateAction.IMMEDIATELY,
    })
  }

  // Ролик тянут за угол — держим 16:9: растянутое видео теряет чёрные поля не
  // лучше, чем зритель — картинку. Excalidraw хранит пропорции только для image,
  // и включить это для embeddable из пропов нельзя — правим высоту сами, на каждом
  // кадре ресайза (Excalidraw пересчитывает размер от исходного, так что расхождение
  // не копится).
  const keepAspect = useCallback((elements: readonly ExcalidrawElement[]) => {
    const api = apiRef.current
    if (!api) return
    const stretched = new Set(
      elements
        .filter(
          (el) =>
            el.type === 'embeddable' &&
            !el.isDeleted &&
            Math.abs(el.height - el.width * YT_RATIO) > ASPECT_EPS
        )
        .map((el) => el.id)
    )
    if (stretched.size === 0) return
    api.updateScene({
      elements: elements.map((el) =>
        stretched.has(el.id)
          ? newElementWith(el, { height: el.width * YT_RATIO })
          : el
      ),
      // Ресайз в историю пишет сам Excalidraw; наша поправка — часть того же жеста.
      captureUpdate: CaptureUpdateAction.NEVER,
    })
  }, [])

  // Перехват drop PDF ДО Excalidraw (у него нет хука на drop; capture-фаза
  // обёртки срабатывает раньше). Не-PDF пропускаем — нативная вставка
  // картинок Excalidraw кладёт base64 в files; для брошенных мышкой мелких
  // картинок приемлемо, наша кнопка вставки идёт через S3.
  // ponytail: фоновая догрузка таких base64-файлов в S3 — когда заметим раздутые снапшоты.
  // Перехват Ctrl+V картинки ДО Excalidraw: нативный image-инструмент отключён
  // (tools.image:false), его paste-обработчик на document.body ответил бы
  // «Изображения отключены». Capture-фаза обёртки идёт раньше bubble на body —
  // ловим blob и уводим в S3-путь. Текст/элементы Excalidraw файлов не несут →
  // пропускаем к нативной вставке.
  const onPasteCapture = (e: React.ClipboardEvent) => {
    const file = imageFromClipboard(e.clipboardData)
    if (!file) return
    e.preventDefault()
    e.stopPropagation()
    void insertImageFile(file)
  }

  const onDropCapture = (e: React.DragEvent) => {
    if (isGuest) return // гостю вставка недоступна (кнопки скрыты) — не ловим drop
    if (!page) return // канвас без страницы доски не смонтирован — drop невозможен
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
        const preflight = await whiteboardApi.uploadPdf(boardId, page.id, file)
        pdfImportRef.current = { importId: preflight.import_id, sizes: preflight.page_sizes }
        setPdfDialog({ numPages: preflight.num_pages, point })
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
        onPasteCapture={onPasteCapture}
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
          onChange={(elements, appState) => {
            keepAspect(elements)
            // Не onUserFollow: тот молчит, когда follow включают программно из
            // панели участников звонка. appState ловит оба входа одинаково.
            syncFollowTarget(appState.userToFollow?.socketId ?? null)
            onChange()
          }}
          onPointerUpdate={(p) => sendCursor(p.pointer.x, p.pointer.y)}
          onScrollChange={() => broadcastViewport()}
          // Встраиваем только YouTube: остальные ссылки — обычные, не iframe.
          // Заодно это фильтр для ссылок, вставленных Ctrl+V.
          validateEmbeddable={(link) => parseYouTubeId(link) !== null}
          // Свой плеер вместо дефолтного iframe: нужен YT API для синхронизации
          // play/pause/seek между преподом и учеником.
          renderEmbeddable={(el) => {
            const videoId = el.link && parseYouTubeId(el.link)
            if (!videoId) return null
            return <YouTubeEmbed id={el.id} videoId={videoId} sync={yt} />
          }}
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
              {/* YouTube по ссылке: ничего не храним, ролик находят по ходу урока. */}
              {!isGuest && (
                <button
                  title="Видео с YouTube"
                  onClick={handleOpenYouTube}
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
                    <rect x="2" y="5" width="20" height="14" rx="4" />
                    <path d="M10 9.5l5 2.5-5 2.5z" />
                  </svg>
                </button>
              )}
              {/* Библиотека материалов препода: только аудио для аудирования. */}
              {!isGuest && (
                <button
                  title="Материалы"
                  onClick={() => setMaterialsOpen((v) => !v)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    border: 'none',
                    background: materialsOpen ? 'var(--color-primary-light)' : 'transparent',
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
                    <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
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
        {!isGuest && materialsOpen && (
          <MaterialsPanel
            onClose={() => setMaterialsOpen(false)}
            onPick={(m, url) => void handlePickMaterial(m, url)}
          />
        )}
        {/* Плеер виден обоим: управлять им может и ученик, закрыть — только препод. */}
        <MediaPlayer player={player} canClose={!isGuest} />
        {pdfDialog && (
          <PdfRangeDialog
            open
            numPages={pdfDialog.numPages}
            onConfirm={handlePdfConfirm}
            onCancel={() => {
              pdfImportRef.current = null
              setPdfDialog(null)
            }}
          />
        )}
      </div>
    </BoardContextProvider>
  )
}
