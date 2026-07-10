'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { reconcileElements, CaptureUpdateAction } from '@excalidraw/excalidraw'
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
  type SnapshotFiles,
} from './excalidrawSync'
import type { BoardPage } from '@/types/api'

type ConnStatus = 'connecting' | 'connected' | 'disconnected'

const UPDATE_THROTTLE_MS = 100
const SNAPSHOT_DEBOUNCE_MS = 1000

export interface ExcalidrawSyncResult {
  status: ConnStatus
  onApiReady: (api: ExcalidrawImperativeAPI) => void
  onChange: () => void
  sendCursor: (x: number, y: number) => void
  registerFile: (fileId: string, url: string, mimeType: string) => void
}

export function useExcalidrawSync(
  page: BoardPage | null,
  token?: string
): ExcalidrawSyncResult {
  const [status, setStatus] = useState<ConnStatus>('connecting')
  const apiRef = useRef<ExcalidrawImperativeAPI | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const retryRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const updateTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const snapshotTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
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

  // Стабильный примитив — connect не пересоздаётся на рефетчах React Query,
  // возвращающих новый объект той же страницы.
  const pageId = page?.id ?? null

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

  const sendSnapshot = useCallback(() => {
    const api = apiRef.current
    if (!api || wsRef.current?.readyState !== WebSocket.OPEN) return
    // Персистим ВКЛЮЧАЯ tombstones (isDeleted): иначе пир, пропустивший
    // удаление офлайн, при реконнекте воскресит элемент через reconcile.
    // ponytail: tombstones копятся за жизнь доски; компакция — когда заметим.
    wsRef.current.send(
      JSON.stringify({
        type: 'snapshot',
        payload: {
          elements: api.getSceneElementsIncludingDeleted(),
          files: filesRef.current,
        },
      })
    )
  }, [])

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
          username: 'Гость',
        })
        apiRef.current?.updateScene({
          collaborators: new Map(collaboratorsRef.current),
        })
      }
    }

    ws.onclose = () => {
      // Игнорируем close от вытесненных сокетов — иначе поздний onclose
      // старого соединения снесёт здоровое и зациклит реконнекты.
      if (closedRef.current || wsRef.current !== ws) return
      setStatus('disconnected')
      // Intentional self-recursion for reconnect backoff; same pattern as
      // the pre-existing useWhiteboardSync.ts, safe since `connect` is
      // fully assigned by the time this closure runs (never invoked
      // synchronously during render).
      retryRef.current = setTimeout(() => {
        // eslint-disable-next-line react-hooks/immutability
        void connect()
      }, 2000)
    }

    ws.onerror = () => ws.close()
  }, [pageId, token, applySnapshot, applyRemote, hydrateFiles])

  useEffect(() => {
    closedRef.current = false
    // Resets connection status synchronously on page switch, mirroring
    // useWhiteboardSync.ts; deferring this would flash a stale 'connected'
    // status from the previous page while the new socket is opening.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setStatus('connecting')
    // Смена страницы = новая сцена: локальные карты обнуляем.
    versionsRef.current = new Map()
    filesRef.current = {}
    collaboratorsRef.current = new Map()
    pendingSeedRef.current = null
    void connect()
    return () => {
      closedRef.current = true
      if (retryRef.current) clearTimeout(retryRef.current)
      if (updateTimerRef.current) clearTimeout(updateTimerRef.current)
      if (snapshotTimerRef.current) clearTimeout(snapshotTimerRef.current)
      retryRef.current = updateTimerRef.current = snapshotTimerRef.current = null
      // Best-effort финальный снапшот перед закрытием.
      sendSnapshot()
      const ws = wsRef.current
      wsRef.current = null
      ws?.close()
    }
  }, [connect, sendSnapshot])

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

  // Локальная правка: троттлим update (100мс), дебаунсим снапшот (1с).
  const onChange = useCallback(() => {
    if (!updateTimerRef.current) {
      updateTimerRef.current = setTimeout(() => {
        updateTimerRef.current = null
        flushUpdate()
      }, UPDATE_THROTTLE_MS)
    }
    if (snapshotTimerRef.current) clearTimeout(snapshotTimerRef.current)
    snapshotTimerRef.current = setTimeout(sendSnapshot, SNAPSHOT_DEBOUNCE_MS)
  }, [flushUpdate, sendSnapshot])

  const sendCursor = useCallback((x: number, y: number) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'cursor', x, y }))
    }
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

  return { status, onApiReady, onChange, sendCursor, registerFile }
}
