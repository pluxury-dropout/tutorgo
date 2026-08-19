'use client'

import { useRef, useState, useCallback, useEffect, useMemo } from 'react'
import {
  Excalidraw,
  convertToExcalidrawElements,
  viewportCoordsToSceneCoords,
  newElementWith,
  CaptureUpdateAction,
  FONT_FAMILY,
} from '@excalidraw/excalidraw'
import '@excalidraw/excalidraw/index.css'
import './excalidraw-theme.css'
import { AudioLines, Sigma } from 'lucide-react'
import { toast } from 'sonner'
import type {
  ExcalidrawImperativeAPI,
  BinaryFileData,
  DataURL,
  NormalizedZoomValue,
} from '@excalidraw/excalidraw/types'
import type {
  FileId,
  ExcalidrawElement,
} from '@excalidraw/excalidraw/element/types'
import { useExcalidrawSync } from './useExcalidrawSync'
import { useMathFiles } from './useMathFiles'
import { fileIdForLatex, formulaCustomData, looksLikeMath, readFormula } from './mathFormula'
import { MathEditor } from './MathEditor'
import { MathShapeAction } from './MathShapeAction'
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

/**
 * Ждёт, пока браузер раскодирует картинку.
 *
 * Excalidraw держит в imageCache Promise до onload и всё это время рисует
 * элемент заглушкой (drawImagePlaceholder). Свой new Image() на тот же
 * data-URL кладёт картинку в память браузера заранее, поэтому его собственный
 * loadHTMLImageElement получает onload ближайшей задачей — и подмена fileId
 * успевает в тот же кадр, без пустого мига.
 *
 * Ошибку глотаем: битую картинку разберёт сам Excalidraw (status: 'error').
 */
const preloadImage = (dataURL: string) =>
  new Promise<void>((resolve) => {
    const img = new Image()
    img.onload = img.onerror = () => resolve()
    img.src = dataURL
  })

// Камера (scroll+zoom) — своя у каждого и в БД не едет (эфемерка, как курсоры),
// поэтому запоминаем её на устройстве: вернулся на страницу — вернулся туда, где
// был, а не в начало координат. Ключ на страницу доски.
// ponytail: localStorage, не сервер. Серверное хранение — когда понадобится
// подхватывать позицию с другого устройства.
const camKey = (pageId: string) => `board-cam:${pageId}`
const CAM_SAVE_MS = 300

function readCamera(pageId: string) {
  try {
    const cam = JSON.parse(localStorage.getItem(camKey(pageId)) ?? 'null')
    if (!cam || ![cam.scrollX, cam.scrollY, cam.zoom].every(Number.isFinite)) return null
    return cam as { scrollX: number; scrollY: number; zoom: number }
  } catch {
    return null
  }
}

// Ролик на доске держим 16:9, как его ни тянули за угол.
const YT_RATIO = 9 / 16
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
    saveFailed,
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
  // Обёртка канваса: нужна для перевода экранных координат клика в координаты
  // сцены (открытие редактора формулы) — переиспользуют задачи 5 и 6.
  const wrapRef = useRef<HTMLDivElement>(null)
  // Тот же узел в состоянии: MathShapeAction нужен контейнер порталу, а читать
  // wrapRef.current во время рендера eslint (react-hooks/refs) запрещает —
  // ref не переживает StrictMode-двойной рендер. Div не пересоздаётся за
  // жизнь компонента (в отличие от Excalidraw с key={page?.id}), поэтому
  // хватает одного присвоения на маунте.
  const [wrapEl, setWrapEl] = useState<HTMLDivElement | null>(null)
  useEffect(() => setWrapEl(wrapRef.current), [])
  const { renderMissing } = useMathFiles(apiRef)

  // onScrollChange летит покадрово во время пана — пишем не чаще CAM_SAVE_MS,
  // trailing'ом (нужна позиция ПОСЛЕ жеста, а не в его начале).
  const camTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pageId = page?.id

  // Снимок сохранённой камеры на момент монтирования Excalidraw (он пересоздаётся
  // по key={pageId}). Не состояние, а начальное значение — перечитывать на каждый
  // рендер нечего, писать её продолжает saveCamera.
  const camera = useMemo(() => (pageId ? readCamera(pageId) : null), [pageId])
  const saveCamera = useCallback(() => {
    if (camTimerRef.current || !pageId) return
    camTimerRef.current = setTimeout(() => {
      camTimerRef.current = null
      const api = apiRef.current
      if (!api) return
      const s = api.getAppState()
      try {
        localStorage.setItem(
          camKey(pageId),
          JSON.stringify({ scrollX: s.scrollX, scrollY: s.scrollY, zoom: s.zoom.value })
        )
      } catch {
        // приватный режим / квота — позиция камеры того не стоит
      }
    }, CAM_SAVE_MS)
  }, [pageId])

  // Первый визит на страницу (сохранённой камеры нет) — показываем содержимое
  // целиком. Элементы приезжают по WS уже ПОСЛЕ монтирования, поэтому ловим
  // момент их появления в onChange, а не при api-ready: там сцена ещё пуста.
  const fitDoneRef = useRef<string | null>(null)
  const fitOnFirstVisit = useCallback(
    (elements: readonly ExcalidrawElement[]) => {
      if (!pageId || fitDoneRef.current === pageId) return
      if (readCamera(pageId)) {
        fitDoneRef.current = pageId
        return
      }
      const visible = elements.filter((el) => !el.isDeleted)
      if (!visible.length) return // сид ещё не доехал — ждём следующего onChange
      fitDoneRef.current = pageId
      // fitToContent, а не fitToViewport: одинокий штрих не должен раздуться
      // на весь экран, зум выше 100% тут не нужен.
      apiRef.current?.scrollToContent(visible, { fitToContent: true, animate: false })
    },
    [pageId]
  )

  // Отложенная запись держит pageId в замыкании: при смене страницы её нужно
  // отменить, иначе камера НОВОЙ страницы уедет в ключ старой.
  useEffect(
    () => () => {
      if (camTimerRef.current) clearTimeout(camTimerRef.current)
      camTimerRef.current = null
    },
    [pageId]
  )

  // Паспорт preflight'а: PDF уже на сервере, ждём выбора диапазона.
  const pdfImportRef = useRef<{ importId: string; sizes: { w: number; h: number }[] } | null>(null)
  const [materialsOpen, setMaterialsOpen] = useState(false)
  const [pdfDialog, setPdfDialog] = useState<{
    numPages: number
    point: { x: number; y: number }
  } | null>(null)
  // Счётчик сеанса редактора формулы. applyFormula бежит асинхронно (await
  // динамического импорта ~490 КБ + await рендера — сотни мс на первой
  // формуле в сессии) и может доехать до api.updateScene ПОСЛЕ того, как
  // редактор уже закрылся (Enter/Esc) или открылась другая формула —
  // применять такой результат нельзя (дублирующая формула поверх коммита,
  // осиротевший черновик после Esc). Бампится на каждое открытие и закрытие
  // редактора; applyFormula ловит несовпадение сразу после await'ов.
  const editorSessionRef = useRef(0)
  const [mathEditor, setMathEditor] = useState<{
    /** id уже стоящей на доске формулы; null — формула ещё не создана */
    elementId: string | null
    latex: string
    /**
     * true — формула создаётся в этом сеансе редактора (openNewFormula,
     * convertTextToFormula): натуральные размеры рендера, Esc удаляет черновик.
     * false — правится формула, уже стоявшая на доске (даблклик, кнопка панели
     * свойств): ширина, растянутая пользователем руками, сохраняется.
     */
    isNew: boolean
    /**
     * id текстового элемента, из которого formula конвертируется
     * (convertTextToFormula); null во всех остальных путях открытия.
     * Тумбстоуним его только в момент коммита — applyFormula.
     */
    sourceTextId: string | null
    /** экранные координаты поля ввода */
    anchor: { left: number; top: number }
    /** куда и с какими свойствами ставить новую формулу */
    place: {
      x: number
      y: number
      angle: number
      groupIds: string[]
      frameId: string | null
    }
  } | null>(null)

  // Что выделено на доске одиночным элементом — повод показать кнопку
  // конвертации в панели свойств (текст → формула / формула → текст).
  const [selection, setSelection] = useState<{ mode: 'text' | 'formula' | null; id: string | null }>({
    mode: null,
    id: null,
  })
  // Синхронная копия selection для сравнения внутри onChange. Сам onChange —
  // инлайновая стрелка, свежая на каждый рендер, но React может не успеть
  // закоммитить обновлённый selection (state) между двумя соседними вызовами
  // onChange в пределах одного жеста — тогда стейт-версия сравнения словила бы
  // повторный «пере-детект» той же смены выделения. Ref не ждёт коммита.
  const selectionRef = useRef(selection)
  // Растёт на каждый onChange доски — сигнал MathShapeAction переспросить
  // .panelColumn: контейнер пересоздаётся React'ом при снятии/повторном
  // выделении, MutationObserver внутри компонента — лишь подстраховка.
  const [panelRevision, setPanelRevision] = useState(0)

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

  // Кадр набора: рисуем формулу и мутируем элемент. Промежуточные кадры не
  // попадают в историю — Ctrl+Z должен откатывать формулу целиком. Возвращает
  // true при успехе — onCommit закрывает редактор только на успехе, чтобы
  // битый LaTeX не съедал набранный текст молча.
  const applyFormula = useCallback(
    async (elementId: string | null, latex: string, commit: boolean): Promise<boolean> => {
      const api = apiRef.current
      if (!api || !latex.trim()) return false

      // Сеанс на момент старта — сверяем после await'ов ниже (см. комментарий
      // у editorSessionRef).
      const session = editorSessionRef.current
      const fontSize = api.getAppState().currentItemFontSize
      // Правка уже стоящей формулы красится в её сохранённый цвет, а не в
      // текущий цвет обводки инструмента — иначе и обычная правка, и Esc-откат
      // молча перекрашивали бы формулу. Тот же приём, что и colorOf в
      // useMathFiles.ts: customData — Record<string, unknown> чужого
      // элемента, но color в него кладём мы сами строкой (см. ниже), каст безопасен.
      const existingBefore = elementId
        ? api.getSceneElementsIncludingDeleted().find((e) => e.id === elementId)
        : null
      const color =
        (existingBefore?.customData?.color as string | undefined) ??
        api.getAppState().currentItemStrokeColor

      let svg: string, width: number, height: number, error: string | undefined
      let svgToDataUrl: (svg: string) => string
      try {
        const mod = await import('@/lib/latexToSvg')
        svgToDataUrl = mod.svgToDataUrl
        ;({ svg, width, height, error } = await mod.latexToSvg(latex, { fontSize, color }))
      } catch (err) {
        // Срыв догрузки чанка (шрифт, сеть) — иначе необработанный rejection.
        console.warn('Не удалось отрендерить формулу', err)
        if (commit) toast.error('Не удалось отрендерить формулу')
        return false
      }
      // Сеанс сменился (Esc/Enter уже закрыли этот редактор или открылась
      // другая формула), пока ждали импорт и рендер — применять результат
      // некуда: элемент/место, под которые он считался, уже не актуальны.
      if (editorSessionRef.current !== session) return false
      if (error || !svg) {
        if (commit) toast.error(`Формула не распознана: ${error ?? 'пустой LaTeX'}`)
        return false
      }

      const fileId = fileIdForLatex(latex, color, fontSize) as FileId
      const file: BinaryFileData = {
        id: fileId,
        dataURL: svgToDataUrl(svg) as DataURL,
        mimeType: 'image/svg+xml',
        created: Date.now(),
      }
      // Порядок обязателен: addFiles греет imageCache только для fileId, на
      // которые УЖЕ ссылается элемент сцены (addNewImagesToImageCache читает
      // scene.getNonDeletedElements()). Файл, добавленный до подмены fileId,
      // мимо кэша проходит незамеченным, и элемент висит заглушкой, пока не
      // сработает throttle(500 мс) на scheduleImageRefresh — вот это и был
      // мигающий провал на каждом кадре набора. Поэтому ниже сначала
      // updateScene, потом addFiles — как в useMathFiles.ts.
      await preloadImage(file.dataURL)
      if (editorSessionRef.current !== session) return false

      const elements = api.getSceneElementsIncludingDeleted()
      const existing = elementId ? elements.find((e) => e.id === elementId) : null
      // Исходный текст конвертации text→formula (convertTextToFormula) убираем
      // тумбстоуном только в момент коммита — до этого он остаётся на доске:
      // нажатие кнопки не должно молча стирать текст раньше, чем есть формула,
      // на которую его заменили (Esc/гейт looksLikeMath оставляли бы дыру).
      const sourceTextId = commit ? mathEditor?.sourceTextId : null

      if (existing) {
        // isNew === false — правится формула, уже стоявшая на доске: пользователь
        // мог растянуть её руками, держим его ширину, высоту пересчитываем по
        // новому соотношению сторон. isNew === true — элемент уже создан
        // предыдущим черновым кадром ЭТОГО сеанса набора, но ширина ещё не
        // финальная (первый кадр мог уйти на одном символе) — берём натуральные
        // размеры свежего рендера на каждый кадр, а не застывшие с первого.
        const preserveWidth = !mathEditor?.isNew
        const nextWidth = preserveWidth ? existing.width : width
        const nextHeight = preserveWidth ? +(nextWidth * (height / width)).toFixed(2) : height
        api.updateScene({
          elements: elements.map((e) => {
            // Формула всегда image-элемент; type-guard нужен, чтобы TS увидел
            // fileId — он есть только у ExcalidrawImageElement, не у union.
            if (e.id === elementId && e.type === 'image') {
              return newElementWith(e, {
                fileId,
                width: nextWidth,
                height: nextHeight,
                customData: { ...e.customData, ...formulaCustomData(latex), color },
              })
            }
            if (sourceTextId && e.id === sourceTextId) return newElementWith(e, { isDeleted: true })
            return e
          }),
          captureUpdate: commit ? CaptureUpdateAction.IMMEDIATELY : CaptureUpdateAction.NEVER,
        })
        api.addFiles([file])
        return true
      }

      const place = mathEditor?.place
      if (!place) return false
      const [el] = convertToExcalidrawElements([
        {
          type: 'image',
          fileId,
          x: place.x,
          y: place.y,
          width,
          height,
          angle: place.angle,
          groupIds: place.groupIds,
          frameId: place.frameId,
          customData: { ...formulaCustomData(latex), color },
        },
      ])
      api.updateScene({
        elements: [
          ...elements.map((e) =>
            sourceTextId && e.id === sourceTextId ? newElementWith(e, { isDeleted: true }) : e
          ),
          el,
        ],
        // Черновой кадр не должен попасть в историю — иначе Esc после него
        // не сможет откатить создание одним newElementWith(isDeleted:true) без
        // лишнего шага отмены, и Ctrl+Z на готовую формулу бил бы дважды.
        captureUpdate: commit ? CaptureUpdateAction.IMMEDIATELY : CaptureUpdateAction.NEVER,
      })
      api.addFiles([file])
      setMathEditor((s) => (s ? { ...s, elementId: el.id } : s))
      return true
    },
    [mathEditor]
  )

  // Открытие пустого редактора: формула ставится в центр видимой области,
  // поле ввода — под ней.
  const openNewFormula = useCallback(() => {
    const api = apiRef.current
    const rect = wrapRef.current?.getBoundingClientRect()
    if (!api || !rect) return
    const scene = viewportCoordsToSceneCoords(
      { clientX: rect.left + rect.width / 2, clientY: rect.top + rect.height / 2 },
      api.getAppState()
    )
    editorSessionRef.current++
    setMathEditor({
      elementId: null,
      latex: '',
      isNew: true,
      sourceTextId: null,
      anchor: { left: rect.width / 2 - 180, top: rect.height - 180 },
      place: { x: scene.x, y: scene.y, angle: 0, groupIds: [], frameId: null },
    })
  }, [])

  // Excalidraw не даёт сменить type, поэтому переключение — это удаление
  // старого элемента и вставка нового на его месте.
  const convertTextToFormula = useCallback(async () => {
    const api = apiRef.current
    if (!api || !selection.id) return
    const src = api.getSceneElementsIncludingDeleted().find((e) => e.id === selection.id)
    if (!src || src.type !== 'text') return

    // Конвертер отличен на математике, но съедает пробелы: «Задача 5» стала бы
    // произведением курсивных переменных. Не прошло гейт — открываем пустое поле.
    const { convertAsciiMathToLatex } = await import('mathlive')
    const latex = looksLikeMath(src.text) ? convertAsciiMathToLatex(src.text) : ''

    const rect = wrapRef.current?.getBoundingClientRect()
    editorSessionRef.current++
    setMathEditor({
      elementId: null,
      latex,
      isNew: true,
      // Исходный текст тумбстоунится в applyFormula в момент коммита — до
      // этого он остаётся на доске (иначе кнопка молча стирает текст раньше,
      // чем появилась формула, которой его заменяют: гейт looksLikeMath или
      // Esc без единого набранного символа оставили бы дыру).
      sourceTextId: src.id,
      anchor: { left: (rect?.width ?? 0) / 2 - 180, top: (rect?.height ?? 0) - 180 },
      // формула встаёт ровно на место текста и остаётся в его группе и фрейме
      place: {
        x: src.x,
        y: src.y,
        angle: src.angle,
        groupIds: [...src.groupIds],
        frameId: src.frameId,
      },
    })
  }, [selection])

  // Обратное превращение: формула становится обычным текстом с её LaTeX.
  const convertFormulaToText = useCallback(() => {
    const api = apiRef.current
    if (!api || !selection.id) return
    const elements = api.getSceneElementsIncludingDeleted()
    const src = elements.find((e) => e.id === selection.id)
    const formula = readFormula(src)
    if (!src || !formula) return

    const [text] = convertToExcalidrawElements([
      {
        type: 'text',
        x: src.x,
        y: src.y,
        text: formula.latex,
        angle: src.angle,
        groupIds: [...src.groupIds],
        frameId: src.frameId,
        // customData — Record<string, unknown> чужого элемента, но color в
        // него кладём мы сами строкой при вставке формулы (applyFormula),
        // поэтому каст безопасен.
        strokeColor: (src.customData?.color as string) ?? api.getAppState().currentItemStrokeColor,
      },
    ])

    api.updateScene({
      elements: elements
        .map((e) => (e.id === src.id ? newElementWith(e, { isDeleted: true }) : e))
        .concat(text),
      captureUpdate: CaptureUpdateAction.IMMEDIATELY,
    })
  }, [selection])

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

  const onDoubleClickCapture = (e: React.MouseEvent) => {
    const api = apiRef.current
    if (!api) return
    const selected = api.getAppState().selectedElementIds
    const el = api.getSceneElements().find((x) => selected[x.id])
    const formula = readFormula(el)
    // Без проверки на формулу мы отобрали бы у обычных картинок штатную
    // обрезку: даблклик по image в 0.18.1 включает crop-режим.
    if (!el || !formula) return
    e.preventDefault()
    e.stopPropagation()
    const rect = wrapRef.current?.getBoundingClientRect()
    editorSessionRef.current++
    setMathEditor({
      elementId: el.id,
      latex: formula.latex,
      isNew: false,
      sourceTextId: null,
      anchor: { left: e.clientX - (rect?.left ?? 0) - 160, top: e.clientY - (rect?.top ?? 0) + 24 },
      // формула уже стоит на доске; place не используется, но держим тип целым
      place: { x: el.x, y: el.y, angle: el.angle, groupIds: [...el.groupIds], frameId: el.frameId },
    })
  }

  return (
    <BoardContextProvider value={{ boardId, courseId, isGuest }}>
      <div
        ref={wrapRef}
        className={`relative w-full h-full${hideUserList ? ' board-hide-userlist' : ''}`}
        onDropCapture={onDropCapture}
        onPasteCapture={onPasteCapture}
        onDoubleClickCapture={onDoubleClickCapture}
      >
        {status === 'disconnected' && (
          <div className="absolute top-2 left-1/2 -translate-x-1/2 z-10 bg-yellow-100 border border-yellow-300 text-yellow-800 text-sm px-3 py-1 rounded-full">
            Переподключение...
          </div>
        )}
        {/* Провал персиста рисование не останавливает, поэтому без баннера он
            незаметен — а именно так доска однажды и переставала сохраняться. */}
        {saveFailed && status !== 'disconnected' && (
          <div className="absolute top-2 left-1/2 -translate-x-1/2 z-10 bg-red-100 border border-red-300 text-red-800 text-sm px-3 py-1 rounded-full">
            Доска не сохраняется — не закрывайте страницу
          </div>
        )}
        <Excalidraw
          key={page?.id ?? 'empty'}
          langCode="ru-RU"
          // Дефолт — Nunito вместо Excalifont. Только appState: элементы
          // приезжают по WS, initialData их не трогает.
          initialData={{
            appState: {
              currentItemFontFamily: FONT_FAMILY.Nunito,
              // Камеру отдаём декларативно, а не updateScene'ом из
              // excalidrawAPI: тот колбэк Excalidraw зовёт из КОНСТРУКТОРА App,
              // и в dev StrictMode конструктор отрабатывает дважды, а монтируют
              // один инстанс — updateScene от осиротевшего бил setState'ом в
              // компонент, который никогда не смонтируется («not yet mounted»).
              // Отложить это таймером нельзя: ждать там нечего. initialData
              // Excalidraw применяет сам в componentDidMount через restore(),
              // причём zoom нормализует по своим MIN/MAX_ZOOM — клампить не
              // нужно, мусор из storage отсекает readCamera.
              ...(camera && {
                scrollX: camera.scrollX,
                scrollY: camera.scrollY,
                zoom: { value: camera.zoom as NormalizedZoomValue },
              }),
            },
          }}
          excalidrawAPI={(api) => {
            apiRef.current = api
            onApiReady(api)
            onApi?.(api)
          }}
          onChange={(elements, appState) => {
            renderMissing()
            // Кнопка конвертации в панели свойств: одиночное выделение текста
            // или формулы включает её, иначе (0 или несколько элементов) — нет.
            const api = apiRef.current
            if (api) {
              const ids = appState.selectedElementIds
              const picked = elements.filter((el) => ids[el.id] && !el.isDeleted)
              const one = picked.length === 1 ? picked[0] : null
              const next = one
                ? readFormula(one)
                  ? ({ mode: 'formula', id: one.id } as const)
                  : one.type === 'text'
                    ? ({ mode: 'text', id: one.id } as const)
                    : ({ mode: null, id: null } as const)
                : ({ mode: null, id: null } as const)
              // Ревизию поднимаем только на реальную смену выделения — onChange
              // летит на каждый кадр рисования и панорамирования, и безусловный
              // инкремент гонял бы MutationObserver в MathShapeAction на каждом
              // таком кадре без всякой связи с панелью свойств. Сверяемся с
              // ref, а не с state selection: state могло ещё не закоммититься
              // между соседними вызовами onChange одного жеста.
              if (selectionRef.current.mode !== next.mode || selectionRef.current.id !== next.id) {
                selectionRef.current = next
                setSelection(next)
                setPanelRevision((n) => n + 1)
              }
            }
            keepAspect(elements)
            fitOnFirstVisit(elements)
            // Не onUserFollow: тот молчит, когда follow включают программно из
            // панели участников звонка. appState ловит оба входа одинаково.
            syncFollowTarget(appState.userToFollow?.socketId ?? null)
            onChange()
          }}
          onPointerUpdate={(p) => sendCursor(p.pointer.x, p.pointer.y)}
          onScrollChange={() => {
            broadcastViewport()
            saveCamera()
          }}
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
          // Картинки и ролики вставляются через Ctrl+V (см. onPasteCapture и
          // validateEmbeddable) — кнопок под них не держим. Здесь остаётся
          // единственное, чего буфером не сделать: библиотека аудио препода
          // и вставка формул (доступна и ученику — как обычный инструмент рисования).
          renderTopRightUI={() => (
            <>
              <button
                data-board-ui
                title="Формула"
                onClick={openNewFormula}
                style={{
                  height: 40,
                  padding: '0 14px 0 10px',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 7,
                  border: '1px solid var(--border)',
                  borderRadius: 10,
                  background: 'var(--card)',
                  color: 'var(--foreground)',
                  fontSize: 13,
                  fontWeight: 500,
                  whiteSpace: 'nowrap',
                  cursor: 'pointer',
                }}
              >
                <Sigma size={18} strokeWidth={1.75} />
                Формула
              </button>
              {!isGuest && (
                <button
                  data-board-ui
                  title="Материалы урока"
                  onClick={() => setMaterialsOpen((v) => !v)}
                  style={{
                    height: 40,
                    padding: '0 14px 0 10px',
                    display: 'flex',
                    alignItems: 'center',
                    gap: 7,
                    border: '1px solid var(--border)',
                    borderRadius: 10,
                    background: materialsOpen ? 'var(--primary-light)' : 'var(--card)',
                    color: 'var(--foreground)',
                    fontSize: 13,
                    fontWeight: 500,
                    whiteSpace: 'nowrap',
                    cursor: 'pointer',
                  }}
                >
                  <AudioLines size={18} strokeWidth={1.75} />
                  Материалы
                </button>
              )}
            </>
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
        <MathShapeAction
          revision={panelRevision}
          container={wrapEl}
          mode={selection.mode}
          onConvert={() => void convertTextToFormula()}
          onEdit={() => {
            const api = apiRef.current
            const el = api?.getSceneElements().find((x) => x.id === selection.id)
            const formula = readFormula(el)
            if (!el || !formula) return
            const rect = wrapRef.current?.getBoundingClientRect()
            editorSessionRef.current++
            setMathEditor({
              elementId: el.id,
              latex: formula.latex,
              isNew: false,
              sourceTextId: null,
              anchor: { left: (rect?.width ?? 0) / 2 - 180, top: (rect?.height ?? 0) - 180 },
              place: { x: el.x, y: el.y, angle: el.angle, groupIds: [...el.groupIds], frameId: el.frameId },
            })
          }}
          onToText={() => void convertFormulaToText()}
        />
        {!isGuest && materialsOpen && (
          <MaterialsPanel
            onClose={() => setMaterialsOpen(false)}
            onPick={(m, url) => void handlePickMaterial(m, url)}
          />
        )}
        {/* Плеер виден обоим: управлять им может и ученик, закрыть — только препод. */}
        <MediaPlayer player={player} canClose={!isGuest} />
        {mathEditor && (
          <MathEditor
            initialLatex={mathEditor.latex}
            anchor={mathEditor.anchor}
            onDraft={(latex) => void applyFormula(mathEditor.elementId, latex, false)}
            onCommit={(latex) => {
              // Бампаем сеанс СРАЗУ: любой ещё летящий черновой applyFormula
              // (debounce) должен опознать себя устаревшим, а не наложиться
              // на этот коммит второй формулой.
              editorSessionRef.current++
              void (async () => {
                const ok = await applyFormula(mathEditor.elementId, latex, true)
                // Провал (битый LaTeX, срыв рендера) не закрывает редактор —
                // иначе набранное молча терялось бы без единого сообщения.
                // toast с текстом ошибки уже показан внутри applyFormula.
                if (ok) setMathEditor(null)
              })()
            }}
            onCancel={() => {
              // См. onCommit — тот же принцип: закрытие сеанса инвалидирует
              // ещё не доехавший черновик до того, как мы решаем, что удалять.
              editorSessionRef.current++
              if (mathEditor.isNew) {
                // Формула этого сеанса набора: черновой кадр (debounce) уже мог
                // создать элемент на доске до Esc. Убираем тумбстоуном мимо
                // истории — до commit'а формулы не было, Ctrl+Z не должен её видеть.
                const api = apiRef.current
                if (api && mathEditor.elementId) {
                  const elementId = mathEditor.elementId
                  api.updateScene({
                    elements: api
                      .getSceneElementsIncludingDeleted()
                      .map((e) => (e.id === elementId ? newElementWith(e, { isDeleted: true }) : e)),
                    captureUpdate: CaptureUpdateAction.NEVER,
                  })
                }
              } else if (mathEditor.elementId) {
                // Правка уже стоявшей формулы: черновые кадры реально меняли
                // элемент (captureUpdate: NEVER) и уже уехали пирам обычным
                // диффом. Откатываем тем же путём, что и обычный кадр —
                // applyFormula с исходным latex, commit=false, чтобы возврат
                // тоже не попал в историю. Ширина не пострадала: preserveWidth
                // держит её нетронутой все черновые кадры правки существующей
                // формулы, только высота гуляла под соотношение сторон.
                void applyFormula(mathEditor.elementId, mathEditor.latex, false)
              }
              setMathEditor(null)
            }}
          />
        )}
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
