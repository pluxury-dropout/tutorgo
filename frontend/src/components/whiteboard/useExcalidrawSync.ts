'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import {
  reconcileElements,
  CaptureUpdateAction,
  zoomToFitBounds,
  getVisibleSceneBounds,
} from '@excalidraw/excalidraw'
import type { SceneBounds } from '@excalidraw/excalidraw/element/bounds'
import type {
  ExcalidrawImperativeAPI,
  BinaryFileData,
  DataURL,
  Collaborator,
  SocketId,
} from '@excalidraw/excalidraw/types'
import type {
  ExcalidrawElement,
  FileId,
} from '@excalidraw/excalidraw/element/types'
import type { RemoteExcalidrawElement } from '@excalidraw/excalidraw/data/reconcile'
import { getWsUrl } from '@/lib/api/whiteboard'
import { getTokenAsync } from '@/lib/api/client'
import {
  diffChangedElements,
  parseSnapshot,
  blobToDataURL,
  mergeCollaborators,
  utf8ByteSize,
  SNAPSHOT_MAX_BYTES,
  type SnapshotFiles,
} from './excalidrawSync'
import type { BoardPage } from '@/types/api'
import type { BoardIdentity } from '@/lib/hooks/useBoardDisplayName'

type ConnStatus = 'connecting' | 'connected' | 'disconnected'

const UPDATE_THROTTLE_MS = 100
const SNAPSHOT_DEBOUNCE_MS = 1000

// Локальный ключ текущего пользователя в Map коллабораторов. Сервер свой peerId
// клиенту не сообщает, а для аватара «вы» реальный id не нужен: клик по своему
// аватару Excalidraw гасит по isCurrentUser, uuid'ы пиров с 'self' не столкнутся.
const SELF_ID = 'self' as SocketId

export interface ExcalidrawSyncResult {
  status: ConnStatus
  onApiReady: (api: ExcalidrawImperativeAPI) => void
  onChange: () => void
  sendCursor: (x: number, y: number) => void
  broadcastViewport: () => void
  registerFile: (fileId: string, url: string, mimeType: string) => void
}

export function useExcalidrawSync(
  page: BoardPage | null,
  token?: string,
  identity?: BoardIdentity
): ExcalidrawSyncResult {
  const displayName = identity?.name
  const myUid = identity?.uid
  const [status, setStatus] = useState<ConnStatus>('connecting')
  const apiRef = useRef<ExcalidrawImperativeAPI | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const retryRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const updateTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const snapshotTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const viewportTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  // Гард против зомби-реконнектов после cleanup эффекта (поздний onclose
  // на размонтированном компоненте / переключённой странице).
  const closedRef = useRef(false)
  // id:version всего, что уже отправлено пирам или получено от них.
  const versionsRef = useRef<Map<string, number>>(new Map())
  // fileId → { url, mimeType }: указатели на S3, персистятся в снапшоте.
  const filesRef = useRef<SnapshotFiles>({})
  // Снапшот пришёл раньше, чем Excalidraw отдал api — буферизуем.
  const pendingSeedRef = useRef<unknown>(null)
  // Курсоры пиров для нативного рендера Excalidraw.
  const collaboratorsRef = useRef<Map<SocketId, Collaborator>>(new Map())
  // Кэш последней сцены для финального снапшота при teardown. НЕ читаем live
  // apiRef в cleanup: при переключении страницы новый (пустой) Excalidraw уже
  // перезаписал apiRef, а этот cleanup — пассивный и бежит позже, так что live
  // api = пустая сцена. Тег pageId страхует и от обратного порядка (initial
  // onChange нового Excalidraw в layout-фазе ДО cleanup): cleanup шлёт снапшот,
  // только если кэш принадлежит уходящей странице.
  const lastSceneRef = useRef<{
    pageId: string
    elements: readonly ExcalidrawElement[]
  } | null>(null)

  // Стабильный примитив — connect не пересоздаётся на рефетчах React Query,
  // возвращающих новый объект той же страницы.
  const pageId = page?.id ?? null

  // Пушит в Excalidraw коллабораторов = удалённые пиры + себя. Себя всегда
  // подмешиваем (isCurrentUser, без pointer — иначе на своём холсте появится
  // призрачный курсор-двойник), чтобы список и счётчик включали текущего юзера.
  const pushCollaborators = useCallback(() => {
    const api = apiRef.current
    if (!api) return
    // Один человек с двух устройств схлопнется силами самого Excalidraw:
    // UserList дедуплицирует по collaborator.id, падая на socketId только когда
    // id пуст (у анонима по ссылке).
    const merged = mergeCollaborators<Collaborator>(
      collaboratorsRef.current,
      SELF_ID,
      { username: displayName || 'Вы', isCurrentUser: true, id: myUid },
      myUid
    )
    api.updateScene({ collaborators: merged as Map<SocketId, Collaborator> })
  }, [displayName, myUid])

  // Догружаем недостающие файлы: S3 URL → blob → dataURL → addFiles.
  // Ошибка одного файла не валит остальные — элемент покажет плейсхолдер.
  const hydrateFiles = useCallback((files: SnapshotFiles) => {
    Object.assign(filesRef.current, files)
    const api = apiRef.current
    if (!api) return
    const have = api.getFiles()
    for (const [id, meta] of Object.entries(files)) {
      if (have[id]) continue
      void (async () => {
        try {
          const resp = await fetch(meta.url)
          if (!resp.ok) throw new Error(`${resp.status}`)
          const dataURL = await blobToDataURL(await resp.blob())
          apiRef.current?.addFiles([
            {
              id: id as FileId,
              dataURL: dataURL as DataURL,
              mimeType: meta.mimeType as BinaryFileData['mimeType'],
              created: Date.now(),
            },
          ])
        } catch {
          // недоступный файл — не критично, остальная доска работает
        }
      })()
    }
  }, [])

  // Сериализует снапшот и шлёт с проверкой размера. Единый путь и для
  // дебаунс-снапшота (live api), и для финального при teardown (кэш сцены).
  const sendSnapshotElements = useCallback(
    (elements: readonly ExcalidrawElement[]) => {
      if (wsRef.current?.readyState !== WebSocket.OPEN) return
      // Персистим ВКЛЮЧАЯ tombstones (isDeleted): иначе пир, пропустивший
      // удаление офлайн, при реконнекте воскресит элемент через reconcile.
      const json = JSON.stringify({
        type: 'snapshot',
        payload: { elements, files: filesRef.current },
      })
      const size = utf8ByteSize(json)
      if (size > SNAPSHOT_MAX_BYTES) {
        // ponytail: tombstones копятся за жизнь доски и растят снапшот
        // монотонно; потолок — ReadLimit 512 КБ Go-хаба, апгрейд — компакция
        // tombstones, когда упрёмся в лимит.
        console.warn(
          `Снапшот доски ${size} байт превышает лимит ${SNAPSHOT_MAX_BYTES} байт — отправка пропущена`
        )
        return
      }
      wsRef.current.send(json)
    },
    []
  )

  const sendSnapshot = useCallback(() => {
    const api = apiRef.current
    if (api) sendSnapshotElements(api.getSceneElementsIncludingDeleted())
  }, [sendSnapshotElements])

  // Шлёт пирам элементы, изменившиеся с последнего flush.
  const flushUpdate = useCallback(() => {
    const api = apiRef.current
    if (!api || wsRef.current?.readyState !== WebSocket.OPEN) return
    const elements = api.getSceneElementsIncludingDeleted()
    const { changed, next } = diffChangedElements(versionsRef.current, elements)
    versionsRef.current = next
    if (changed.length === 0) return
    wsRef.current.send(
      JSON.stringify({ type: 'update', payload: { elements: changed } })
    )
  }, [])

  // Вливает удалённые элементы через reconcileElements (слияние по
  // version/versionNonce — двое рисуют одновременно без затирания).
  const applyRemote = useCallback((remote: ExcalidrawElement[]) => {
    const api = apiRef.current
    if (!api) return
    const reconciled = reconcileElements(
      api.getSceneElementsIncludingDeleted(),
      remote as unknown as RemoteExcalidrawElement[],
      api.getAppState()
    )
    // Версии — ДО updateScene: эхо-onChange даст пустой дифф и не зациклит.
    versionsRef.current = new Map(reconciled.map((el) => [el.id, el.version]))
    // NEVER — чужие правки не попадают в локальный undo-стек.
    api.updateScene({
      elements: reconciled,
      captureUpdate: CaptureUpdateAction.NEVER,
    })
  }, [])

  // Сидинг при (ре)коннекте: тот же reconcile-путь, что и update, плюс
  // отправка пирам того, чего у сервера ещё нет (локальные офлайн-правки).
  const applySnapshot = useCallback(
    (payload: unknown) => {
      const snap = parseSnapshot(payload)
      if (!snap) return // старый tldraw-формат или мусор — чистая доска
      const remoteVersions = new Map(
        snap.elements.map((el) => [el.id, el.version])
      )
      applyRemote(snap.elements as ExcalidrawElement[])
      // applyRemote выставил versionsRef из reconciled; переставляем на то,
      // что знает сервер, и flush — локальные победители уйдут update'ом.
      versionsRef.current = remoteVersions
      flushUpdate()
      hydrateFiles(snap.files)
    },
    [applyRemote, flushUpdate, hydrateFiles]
  )

  const connect = useCallback(async () => {
    if (!pageId) return
    // Закрываем предыдущий сокет явно — переключения страниц и реконнекты
    // не должны плодить параллельные соединения.
    wsRef.current?.close()

    // Ждём возможный проактивный рефреш токена (HTTP-перехватчик axios
    // обновляет его асинхронно; WS этот путь минует).
    const wsToken = await getTokenAsync(token)
    if (closedRef.current) return
    const ws = new WebSocket(getWsUrl(pageId, wsToken))
    wsRef.current = ws

    ws.onopen = () => {
      if (wsRef.current === ws) setStatus('connected')
    }

    ws.onmessage = (e: MessageEvent) => {
      const msg = JSON.parse(e.data as string) as {
        type: string
        payload?: unknown
        peerId?: string
        name?: string
        uid?: string
        x?: number
        y?: number
      }

      if (msg.type === 'snapshot') {
        if (apiRef.current) applySnapshot(msg.payload)
        else pendingSeedRef.current = msg.payload
        return
      }

      if (msg.type === 'update') {
        const p = msg.payload as { elements?: ExcalidrawElement[] }
        if (Array.isArray(p?.elements)) applyRemote(p.elements)
        return
      }

      if (msg.type === 'file') {
        const p = msg.payload as {
          fileId?: string
          url?: string
          mimeType?: string
        }
        if (p?.fileId && p.url && p.mimeType)
          hydrateFiles({ [p.fileId]: { url: p.url, mimeType: p.mimeType } })
        return
      }

      if (msg.type === 'cursor' && msg.peerId) {
        collaboratorsRef.current.set(msg.peerId as SocketId, {
          pointer: { x: msg.x ?? 0, y: msg.y ?? 0, tool: 'pointer' },
          username: msg.name || 'Гость',
          id: msg.uid,
        })
        pushCollaborators()
        return
      }

      if (msg.type === 'leave' && msg.peerId) {
        collaboratorsRef.current.delete(msg.peerId as SocketId)
        pushCollaborators()
        return
      }

      // Follow-mode: ведомый двигает камеру за тем пиром, чей peerId совпал с
      // локальным userToFollow.socketId (его ставит клик по аватару в Excalidraw).
      // Пиры вещают границы всегда — фильтруем на приёме, без серверного трекинга.
      if (msg.type === 'viewport' && msg.peerId) {
        const api = apiRef.current
        if (!api) return
        const appState = api.getAppState()
        if (appState.userToFollow?.socketId !== msg.peerId) return
        const bounds = (msg.payload as { bounds?: SceneBounds } | undefined)
          ?.bounds
        if (!bounds) return
        api.updateScene({
          appState: zoomToFitBounds({ bounds, appState, fitToViewport: true })
            .appState,
        })
        return
      }
    }

    ws.onclose = () => {
      // Игнорируем close от вытесненных сокетов — иначе поздний onclose
      // старого соединения снесёт здоровое и зациклит реконнекты.
      if (closedRef.current || wsRef.current !== ws) return
      setStatus('disconnected')
      // Намеренная само-рекурсия для reconnect-backoff — тот же паттерн
      // реконнекта, что был в старом sync-хуке доски; безопасно, т.к.
      // `connect` полностью присвоен к моменту вызова этого замыкания
      // (никогда не вызывается синхронно во время рендера).
      retryRef.current = setTimeout(() => {
        // eslint-disable-next-line react-hooks/immutability
        void connect()
      }, 2000)
    }

    ws.onerror = () => ws.close()
  }, [pageId, token, applySnapshot, applyRemote, hydrateFiles, pushCollaborators])

  useEffect(() => {
    closedRef.current = false
    // Синхронно сбрасываем статус соединения при смене страницы — тот же
    // паттерн, что был в старом sync-хуке доски; отложенный сброс мигнул бы
    // устаревшим 'connected' с прошлой страницы, пока открывается новый сокет.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setStatus('connecting')
    // Смена страницы = новая сцена: локальные карты обнуляем.
    versionsRef.current = new Map()
    filesRef.current = {}
    collaboratorsRef.current = new Map()
    pendingSeedRef.current = null
    lastSceneRef.current = null
    void connect()
    return () => {
      closedRef.current = true
      if (retryRef.current) clearTimeout(retryRef.current)
      if (updateTimerRef.current) clearTimeout(updateTimerRef.current)
      if (snapshotTimerRef.current) clearTimeout(snapshotTimerRef.current)
      if (viewportTimerRef.current) clearTimeout(viewportTimerRef.current)
      retryRef.current = updateTimerRef.current = snapshotTimerRef.current = null
      viewportTimerRef.current = null
      // Best-effort финальный снапшот из кэша сцены (не из live apiRef, см.
      // lastSceneRef). Тег pageId защищает от гонки с initial onChange нового
      // Excalidraw: шлём, только если кэш — от уходящей страницы. filesRef ещё
      // держит файлы уходящей страницы (reset filesRef — в setup нового
      // эффекта, ПОСЛЕ этого cleanup). null → локальных правок не было, пропуск.
      const cached = lastSceneRef.current
      if (cached && cached.pageId === pageId) sendSnapshotElements(cached.elements)
      const ws = wsRef.current
      wsRef.current = null
      ws?.close()
    }
  }, [pageId, connect, sendSnapshotElements])

  const onApiReady = useCallback(
    (api: ExcalidrawImperativeAPI) => {
      apiRef.current = api
      if (pendingSeedRef.current !== null) {
        const seed = pendingSeedRef.current
        pendingSeedRef.current = null
        applySnapshot(seed)
      }
    },
    [applySnapshot]
  )

  // Показать себя в списке участников, не дожидаясь других. Именно из эффекта,
  // а НЕ из onApiReady: Excalidraw отдаёт api прямо в своём конструкторе, до
  // маунта, а updateScene({collaborators}) — это setState, который React на
  // несмонтированном компоненте молча выбрасывает. К моменту эффекта дерево уже
  // закоммичено. pageId в зависимостях: смена страницы = key= → новый
  // конструктор Excalidraw и пустой appState.
  useEffect(() => {
    pushCollaborators()
  }, [pageId, pushCollaborators])

  // Локальная правка: троттлим update (100мс), дебаунсим снапшот (1с).
  const onChange = useCallback(() => {
    // Кэшируем сцену уходящей страницы для teardown-снапшота (тег pageId —
    // чтобы initial onChange нового Excalidraw не подменил кэш пустой сценой).
    const els = apiRef.current?.getSceneElementsIncludingDeleted()
    if (els && pageId) lastSceneRef.current = { pageId, elements: els }
    if (!updateTimerRef.current) {
      updateTimerRef.current = setTimeout(() => {
        updateTimerRef.current = null
        flushUpdate()
      }, UPDATE_THROTTLE_MS)
    }
    if (snapshotTimerRef.current) clearTimeout(snapshotTimerRef.current)
    snapshotTimerRef.current = setTimeout(sendSnapshot, SNAPSHOT_DEBOUNCE_MS)
  }, [pageId, flushUpdate, sendSnapshot])

  const sendCursor = useCallback(
    (x: number, y: number) => {
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(
          JSON.stringify({ type: 'cursor', x, y, name: displayName, uid: myUid })
        )
      }
    },
    [displayName, myUid]
  )

  // Вещаем свои видимые границы сцены ведомым (follow-mode). Trailing-throttle:
  // при панорамировании летит поток onScrollChange — шлём не чаще UPDATE_THROTTLE_MS,
  // но последнюю позицию гарантированно дослыаем таймером (иначе камера ведомого
  // застынет чуть раньше конца жеста).
  const broadcastViewport = useCallback(() => {
    if (viewportTimerRef.current) return
    const send = () => {
      const api = apiRef.current
      if (!api || wsRef.current?.readyState !== WebSocket.OPEN) return
      const bounds = getVisibleSceneBounds(api.getAppState())
      wsRef.current.send(
        JSON.stringify({ type: 'viewport', payload: { bounds } })
      )
    }
    send()
    viewportTimerRef.current = setTimeout(() => {
      viewportTimerRef.current = null
      send() // trailing: финальная позиция после последнего скролла
    }, UPDATE_THROTTLE_MS)
  }, [])

  const registerFile = useCallback(
    (fileId: string, url: string, mimeType: string) => {
      filesRef.current[fileId] = { url, mimeType }
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(
          JSON.stringify({ type: 'file', payload: { fileId, url, mimeType } })
        )
      }
    },
    []
  )

  return {
    status,
    onApiReady,
    onChange,
    sendCursor,
    broadcastViewport,
    registerFile,
  }
}
