'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import {
  reconcileElements,
  CaptureUpdateAction,
  zoomToFitBounds,
  getVisibleSceneBounds,
  newElementWith,
} from '@excalidraw/excalidraw'
import type { SceneBounds } from '@excalidraw/excalidraw/element/bounds'
import type {
  ExcalidrawImperativeAPI,
  BinaryFileData,
  DataURL,
  Collaborator,
  SocketId,
  NormalizedZoomValue,
} from '@excalidraw/excalidraw/types'
import type {
  ExcalidrawElement,
  FileId,
} from '@excalidraw/excalidraw/element/types'
import type { RemoteExcalidrawElement } from '@excalidraw/excalidraw/data/reconcile'
import {
  getWsUrl,
  BASE_URL,
  saveSnapshot,
  beaconSnapshot,
} from '@/lib/api/whiteboard'
import { getTokenAsync } from '@/lib/api/client'
import {
  diffChangedElements,
  markRemoteVersions,
  parseSnapshot,
  blobToDataURL,
  mergeCollaborators,
  serializeSnapshot,
  serializeUpdate,
  type SnapshotFiles,
} from './excalidrawSync'
import { lerpCamera, camerasClose, type Camera } from './viewportInterp'
import type { MediaPayload } from './mediaSync'
import type { BoardPage } from '@/types/api'
import type { BoardIdentity } from '@/lib/hooks/useBoardDisplayName'

type ConnStatus = 'connecting' | 'connected' | 'disconnected'

const UPDATE_THROTTLE_MS = 100
// Регистрация файлов идёт пачкой (PDF на 50 страниц — 50 вызовов registerFile
// подряд), схлопываем их в один POST. Полная сцена сюда больше не уезжает, так
// что дебаунс не про WAL, а только про склейку пачки.
const FILES_DEBOUNCE_MS = 1000
const FOLLOW_LERP = 0.3 // доля пути к цели за кадр — компромисс плавность/лаг
const CURSOR_THROTTLE_MS = 50

// Буфер сокета выше порога — bulk-данные (снапшот/диффы) забили пайп;
// эфемерный кадр дропаем, следующий приедет свежим.
const EPHEMERAL_BACKPRESSURE_BYTES = 128 * 1024

// Локальный ключ текущего пользователя в Map коллабораторов. Сервер свой peerId
// клиенту не сообщает, а для аватара «вы» реальный id не нужен: клик по своему
// аватару Excalidraw гасит по isCurrentUser, uuid'ы пиров с 'self' не столкнутся.
const SELF_ID = 'self' as SocketId

export interface ExcalidrawSyncResult {
  status: ConnStatus
  // Последнее сохранение доски провалилось. Показывать обязательно: рисование
  // идёт по WS и выглядит рабочим, даже когда персист лежит — именно так
  // молчаливый отказ сохранения однажды уже съел содержимое уроков.
  saveFailed: boolean
  onApiReady: (api: ExcalidrawImperativeAPI) => void
  onChange: () => void
  sendCursor: (x: number, y: number) => void
  broadcastViewport: () => void
  syncFollowTarget: (target: string | null) => void
  registerFile: (fileId: string, url: string, mimeType: string) => void
  sendMedia: (p: MediaPayload) => void
}

export function useExcalidrawSync(
  page: BoardPage | null,
  token?: string,
  identity?: BoardIdentity,
  onMedia?: (p: MediaPayload) => void,
  onFile?: (fileId: string) => void,
  onImportFailed?: (fileIds: string[]) => void,
  /** uid человека → сколько пиров за ним сейчас следит (только ненулевые). */
  onFollowers?: (byUid: Record<string, number>) => void
): ExcalidrawSyncResult {
  const displayName = identity?.name
  const myUid = identity?.uid
  // Колбэк живёт в ref: иначе новый инлайн-обработчик на каждом рендере попал бы
  // в deps connect и передёргивал бы WS-соединение.
  const onMediaRef = useRef(onMedia)
  useEffect(() => {
    onMediaRef.current = onMedia
  }, [onMedia])
  const onFileRef = useRef(onFile)
  const onImportFailedRef = useRef(onImportFailed)
  const onFollowersRef = useRef(onFollowers)
  useEffect(() => {
    onFileRef.current = onFile
    onImportFailedRef.current = onImportFailed
    onFollowersRef.current = onFollowers
  }, [onFile, onImportFailed, onFollowers])
  const [status, setStatus] = useState<ConnStatus>('connecting')
  const [saveFailed, setSaveFailed] = useState(false)
  const apiRef = useRef<ExcalidrawImperativeAPI | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  // Токен для персиста — тот же, что ушёл в WS. Держим в ref: sendBeacon на
  // выгрузке страницы синхронный, ждать getTokenAsync там уже негде.
  const httpTokenRef = useRef<string | undefined>(token)
  // Сохранения не должны идти внахлёст: ответы могут прийти не в том порядке,
  // и старая сцена затрёт новую. Пока запрос в полёте, копим последнее тело.
  const savingRef = useRef(false)
  const pendingBodyRef = useRef<string | null>(null)
  const retryRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const updateTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const filesTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
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
  // Право сохранять: открывается только сидом от сервера. Excalidraw монтируется
  // пустым, а onChange стартует и без рисования (updateScene коллабораторов), так
  // что до прихода сида дебаунс успевал запостить пустую сцену поверх целой доски
  // — на HTTP-персисте это стёрло боевую доску. У WS такой гонки не было: до
  // открытия сокета отправить было физически нечего куда.
  const seededRef = useRef(false)
  // Курсоры пиров для нативного рендера Excalidraw.
  const collaboratorsRef = useRef<Map<SocketId, Collaborator>>(new Map())
  // Follow: за кем следим (peerId) и куда ведём камеру.
  const followTargetRef = useRef<string | null>(null)
  const targetCamRef = useRef<Camera | null>(null)
  // Гард: пока сами двигаем камеру в follow, onScrollChange не вещаем (эхо).
  const applyingRemoteRef = useRef(false)
  // Курсоры пиров меняются — применим пачкой в rAF, не на каждое сообщение.
  const collaboratorsDirtyRef = useRef(false)
  const rafRef = useRef<number | null>(null)
  const cursorSentRef = useRef(0)
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

  // Счётчики слежки: сервер считает их по peerId, а панель участников знает
  // людей по uid — перевод возможен только здесь, по карте коллабораторов.
  // Свой счётчик приходит отдельным полем: собственный peerId клиенту не
  // сообщается (SELF_ID — локальная выдумка, см. выше).
  const followerCountsRef = useRef<Record<string, number>>({})
  const myFollowersRef = useRef(0)
  const emitFollowers = useCallback(() => {
    const byUid: Record<string, number> = {}
    for (const [peerId, n] of Object.entries(followerCountsRef.current)) {
      const uid = collaboratorsRef.current.get(peerId as SocketId)?.id
      if (uid) byUid[uid] = n
    }
    if (myUid && myFollowersRef.current) byUid[myUid] = myFollowersRef.current
    onFollowersRef.current?.(byUid)
  }, [myUid])

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
          const src = meta.url.startsWith('/') ? `${BASE_URL}${meta.url}` : meta.url
          const resp = await fetch(src)
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

  // Отправка снапшота на сервер. Persist идёт по HTTP, а не по WS: у хаба
  // ReadLimit 512 КБ, и снапшот, переросший его, раньше молча не сохранялся —
  // при живом WS и работающем рисовании доска просто переставала переживать
  // перезагрузку. У HTTP лимит наш, и провал виден по коду ответа.
  const postSnapshot = useCallback(async (pid: string, body: string) => {
    if (savingRef.current) {
      // Запрос уже в полёте — он же дошлёт это тело, когда освободится.
      pendingBodyRef.current = body
      return
    }
    savingRef.current = true
    try {
      let next: string | null = body
      while (next) {
        try {
          await saveSnapshot(pid, next, httpTokenRef.current)
          if (!closedRef.current) setSaveFailed(false)
        } catch (err) {
          console.warn('Снапшот доски не сохранён', err)
          if (!closedRef.current) setSaveFailed(true)
        }
        // Дренаж накопленного за время запроса: без этого последняя правка
        // осталась бы только в памяти вкладки.
        next = pendingBodyRef.current
        pendingBodyRef.current = null
      }
    } finally {
      savingRef.current = false
    }
  }, [])

  // Единый путь и для дебаунс-снапшота (live api), и для финального при
  // teardown (кэш сцены). beacon — для выгрузки страницы, см. вызывающих.
  const sendSnapshotElements = useCallback(
    (elements: readonly ExcalidrawElement[], beacon = false) => {
      if (!pageId || !seededRef.current) return
      const body = serializeSnapshot(elements, filesRef.current)
      if (beacon && beaconSnapshot(pageId, body, httpTokenRef.current)) return
      void postSnapshot(pageId, body)
    },
    [pageId, postSnapshot]
  )

  // Карта files — единственное, чего нет в поэлементном персисте: WS-сообщение
  // 'file' сервер ретранслирует пирам, но в БД не пишет (MergeFiles зовут только
  // отсюда). Шлём её тем же роутом с ПУСТЫМ elements: MergeElements на пустом
  // списке — no-op, MergeFiles сливает карту, стереть ничего нельзя.
  //
  // Раньше этим путём каждые 3 с уезжала вся сцена. На боевой странице в 2337
  // элементов JSON.stringify занимал 69 мс главного потока и слал 2.8 МБ — ровно
  // те провалы кадров при панорамировании, из-за которых это и переписано.
  // Элементы туда больше не нужны: с миграции 029 они доезжают диффами по WS.
  //
  // Гард seededRef тут не нужен (в отличие от sendSnapshotElements): затирать
  // нечего, а картинку, вставленную до прихода сида, терять нельзя. HTTP не
  // зависит от WS, так что путь живёт и на оборванном сокете.
  const persistFiles = useCallback(() => {
    if (!pageId) return
    void postSnapshot(pageId, serializeSnapshot([], filesRef.current))
  }, [pageId, postSnapshot])

  // Шлёт пирам элементы, изменившиеся с последнего flush.
  const flushUpdate = useCallback(() => {
    const api = apiRef.current
    if (!api || wsRef.current?.readyState !== WebSocket.OPEN) return
    const elements = api.getSceneElementsIncludingDeleted()
    const { changed, next } = diffChangedElements(versionsRef.current, elements)
    versionsRef.current = next
    if (changed.length === 0) return
    // roundFloats режет координаты до COORD_PRECISION — тот же путь, что у
    // снапшота. Голый JSON.stringify гнал полные float64 (40 байт на точку
    // пера вместо 14): штрих на 400-500 точек уходил ~300 КБ вместо ~105 КБ.
    wsRef.current.send(serializeUpdate(changed))
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
    // Отмечаем только приехавшее: локальные элементы, ещё ждущие flushUpdate
    // (вставка PDF — это минуты таких), обязаны остаться в диффе.
    markRemoteVersions(versionsRef.current, remote)
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
    // Тот же токен обслуживает и HTTP-персист — роут снапшота авторизуется
    // ровно как WS (tutor JWT или invite гостя).
    httpTokenRef.current = wsToken
    const ws = new WebSocket(getWsUrl(pageId, wsToken))
    wsRef.current = ws

    ws.onopen = () => {
      if (wsRef.current === ws) setStatus('connected')
      // Реконнект во время активной слежки: у нового сокета новый peerId, и
      // серверная following-карта забыла нас при уходе старого коннекта —
      // пере-подписываемся, иначе камера ведомого молча замрёт до ручного
      // пере-клика. Цель не переподключалась, её peerId ещё валиден.
      if (followTargetRef.current) {
        ws.send(
          JSON.stringify({
            type: 'follow',
            payload: { target: followTargetRef.current, action: 'FOLLOW' },
          })
        )
      }
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
        // Флаг ставим по факту получения, а не применения: пока api нет,
        // отправлять всё равно нечего (teardown шлёт кэш из onChange), а
        // onApiReady применит буфер до первого onChange.
        seededRef.current = true
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
        if (p?.fileId && p.url && p.mimeType) {
          hydrateFiles({ [p.fileId]: { url: p.url, mimeType: p.mimeType } })
          onFileRef.current?.(p.fileId)
        }
        return
      }

      // Сервер не смог отрендерить PDF: убираем плейсхолдеры навсегда —
      // файла для них не будет. Тумбстоуны уедут пирам обычным диффом.
      if (msg.type === 'import_failed') {
        const p = msg.payload as { fileIds?: string[] }
        const dead = new Set(p?.fileIds ?? [])
        const api = apiRef.current
        if (!api || dead.size === 0) return
        api.updateScene({
          elements: api
            .getSceneElementsIncludingDeleted()
            .map((el) =>
              el.type === 'image' && el.fileId && dead.has(el.fileId)
                ? newElementWith(el, { isDeleted: true })
                : el
            ),
          captureUpdate: CaptureUpdateAction.IMMEDIATELY,
        })
        onImportFailedRef.current?.(p?.fileIds ?? [])
        return
      }

      // Синхронный плеер: кадр применяет useMediaPlayer у получателя. Сервер
      // ретранслирует тип 'media' вербатим (ветка default: в хабе доски).
      if (msg.type === 'media') {
        onMediaRef.current?.(msg.payload as MediaPayload)
        return
      }

      // Кто за кем следит. Хаб шлёт это на каждое follow/unfollow/уход пира.
      if (msg.type === 'followers') {
        const p = msg.payload as { counts?: Record<string, number>; me?: number }
        followerCountsRef.current = p?.counts ?? {}
        myFollowersRef.current = p?.me ?? 0
        emitFollowers()
        return
      }

      if (msg.type === 'cursor' && msg.peerId) {
        // Пир, подключившийся при уже активной слежке, приезжает в счётчиках
        // раньше, чем в карте коллабораторов (та наполняется курсорами). Узнав
        // его uid, пересчитываем: следующего follow-события можно ждать до
        // конца урока, а глаз должен появиться сейчас.
        const wasUnknown = !collaboratorsRef.current.get(msg.peerId as SocketId)?.id
        collaboratorsRef.current.set(msg.peerId as SocketId, {
          pointer: { x: msg.x ?? 0, y: msg.y ?? 0, tool: 'pointer' },
          username: msg.name || 'Гость',
          id: msg.uid,
        })
        collaboratorsDirtyRef.current = true
        if (wasUnknown && msg.uid && followerCountsRef.current[msg.peerId]) {
          emitFollowers()
        }
        return
      }

      if (msg.type === 'leave' && msg.peerId) {
        collaboratorsRef.current.delete(msg.peerId as SocketId)
        collaboratorsDirtyRef.current = true
        // Ушёл тот, за кем следили — снимаем слежку, иначе камера застынет.
        if (followTargetRef.current === msg.peerId) {
          followTargetRef.current = null
          targetCamRef.current = null
        }
        return
      }

      // Follow: вьюпорт приходит уже только для нашей цели (сервер
      // маршрутизирует). Считаем целевую камеру, доводит к ней rAF-цикл.
      if (msg.type === 'viewport' && msg.peerId) {
        if (msg.peerId !== followTargetRef.current) return
        const api = apiRef.current
        if (!api) return
        const bounds = (msg.payload as { bounds?: SceneBounds } | undefined)
          ?.bounds
        if (!bounds) return
        const fit = zoomToFitBounds({
          bounds,
          appState: api.getAppState(),
          fitToViewport: true,
        }).appState
        targetCamRef.current = {
          scrollX: fit.scrollX,
          scrollY: fit.scrollY,
          zoom: fit.zoom.value,
        }
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
  }, [
    pageId,
    token,
    emitFollowers,
    applySnapshot,
    applyRemote,
    hydrateFiles,
    pushCollaborators,
  ])

  // Единый кадровый цикл: раз в кадр применяем накопленные курсоры и,
  // если следим за кем-то, подводим камеру к цели интерполяцией.
  const tick = useCallback(() => {
    const api = apiRef.current
    if (api) {
      if (collaboratorsDirtyRef.current) {
        collaboratorsDirtyRef.current = false
        pushCollaborators()
      }
      const target = targetCamRef.current
      if (followTargetRef.current && target) {
        const s = api.getAppState()
        const cur: Camera = { scrollX: s.scrollX, scrollY: s.scrollY, zoom: s.zoom.value }
        if (!camerasClose(cur, target)) {
          const next = lerpCamera(cur, target, FOLLOW_LERP)
          applyingRemoteRef.current = true
          api.updateScene({
            appState: {
              scrollX: next.scrollX,
              scrollY: next.scrollY,
              zoom: { value: next.zoom as NormalizedZoomValue },
            },
          })
          applyingRemoteRef.current = false
        }
      }
    }
    rafRef.current = requestAnimationFrame(tick)
  }, [pushCollaborators])

  useEffect(() => {
    closedRef.current = false
    // Синхронно сбрасываем статус соединения при смене страницы — тот же
    // паттерн, что был в старом sync-хуке доски; отложенный сброс мигнул бы
    // устаревшим 'connected' с прошлой страницы, пока открывается новый сокет.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setStatus('connecting')
    setSaveFailed(false)
    // Смена страницы = новая сцена: локальные карты обнуляем.
    versionsRef.current = new Map()
    filesRef.current = {}
    collaboratorsRef.current = new Map()
    pendingSeedRef.current = null
    seededRef.current = false
    lastSceneRef.current = null
    void connect()
    rafRef.current = requestAnimationFrame(tick)

    // Закрытие вкладки и F5 не дают React отработать cleanup, а последняя
    // секунда рисования живёт только в дебаунсе. pagehide (в отличие от
    // beforeunload) срабатывает и на мобильных, а sendBeacon переживает
    // выгрузку — обычный fetch браузер бы отменил.
    const onPageHide = () => {
      const cached = lastSceneRef.current
      if (cached && cached.pageId === pageId) {
        sendSnapshotElements(cached.elements, true)
      }
    }
    window.addEventListener('pagehide', onPageHide)

    return () => {
      window.removeEventListener('pagehide', onPageHide)
      closedRef.current = true
      if (rafRef.current) cancelAnimationFrame(rafRef.current)
      followTargetRef.current = null
      targetCamRef.current = null
      if (retryRef.current) clearTimeout(retryRef.current)
      if (updateTimerRef.current) clearTimeout(updateTimerRef.current)
      if (filesTimerRef.current) clearTimeout(filesTimerRef.current)
      if (viewportTimerRef.current) clearTimeout(viewportTimerRef.current)
      retryRef.current = updateTimerRef.current = filesTimerRef.current = null
      viewportTimerRef.current = null
      // Best-effort финальный снапшот из кэша сцены (не из live apiRef, см.
      // lastSceneRef). Тег pageId защищает от гонки с initial onChange нового
      // Excalidraw: шлём, только если кэш — от уходящей страницы. filesRef ещё
      // держит файлы уходящей страницы (reset filesRef — в setup нового
      // эффекта, ПОСЛЕ этого cleanup). null → локальных правок не было, пропуск.
      // Он же дошлёт карту files, если отменённый выше дебаунс не успел, и
      // закрывает окно в 100 мс между последним flushUpdate и закрытием сокета —
      // это единственное место, где полная сцена ещё уезжает по HTTP.
      const cached = lastSceneRef.current
      if (cached && cached.pageId === pageId) sendSnapshotElements(cached.elements)
      const ws = wsRef.current
      wsRef.current = null
      ws?.close()
    }
  }, [pageId, connect, sendSnapshotElements, tick])

  const onApiReady = useCallback(
    (api: ExcalidrawImperativeAPI) => {
      apiRef.current = api
      if (pendingSeedRef.current !== null) {
        const seed = pendingSeedRef.current
        pendingSeedRef.current = null
        applySnapshot(seed)
      }
      // Показать себя в списке участников сразу, не дожидаясь пиров. Через
      // setTimeout, а не напрямую: Excalidraw отдаёт api из своего конструктора,
      // где updateScene({collaborators}) — это setState на несмонтированном
      // компоненте. Ноль-таймаут откладывает пуш до конца коммита.
      setTimeout(() => pushCollaborators(), 0)
    },
    [applySnapshot, pushCollaborators]
  )

  // Перепушиваем себя, когда доехала личность: имя и uid приходят асинхронно
  // (auth-store гидратируется, /student/me — запрос), нередко уже ПОСЛЕ того,
  // как onApiReady показал «Вы» без uid. Без этого пуша дедупликация своих
  // соединений не сработала бы. Пуш идемпотентен; до появления api — no-op.
  // pageId в зависимостях: смена страницы = key= → новый Excalidraw, пустой
  // appState.
  useEffect(() => {
    pushCollaborators()
  }, [pageId, pushCollaborators])

  // Локальная правка: троттлим update (100мс). Excalidraw зовёт onChange из
  // componentDidUpdate, то есть на КАЖДЫЙ кадр панорамирования и на каждый пуш
  // коллабораторов — всё, что висит здесь, автоматически покадровое. Держим путь
  // пустым: сериализация сцены отсюда убрана (см. persistFiles).
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
  }, [pageId, flushUpdate])

  const sendCursor = useCallback(
    (x: number, y: number) => {
      const now = performance.now()
      if (now - cursorSentRef.current < CURSOR_THROTTLE_MS) return
      cursorSentRef.current = now
      if (wsRef.current?.readyState !== WebSocket.OPEN) return
      if (wsRef.current.bufferedAmount > EPHEMERAL_BACKPRESSURE_BYTES) return
      wsRef.current.send(
        JSON.stringify({ type: 'cursor', x, y, name: displayName, uid: myUid })
      )
    },
    [displayName, myUid]
  )

  // Вещаем свои видимые границы сцены ведомым (follow-mode). Trailing-throttle:
  // при панорамировании летит поток onScrollChange — шлём не чаще UPDATE_THROTTLE_MS,
  // но последнюю позицию гарантированно дослыаем таймером (иначе камера ведомого
  // застынет чуть раньше конца жеста).
  const broadcastViewport = useCallback(() => {
    // Пока следим за кем-то, все движения камеры — наша интерполяция, не наш
    // жест: не вещаем. Надёжнее синхронного applyingRemoteRef — под React 18
    // updateScene батчится, и onScrollChange прилетает уже ПОСЛЕ сброса флага.
    // Excalidraw сам гасит userToFollow на ручном пане → followTargetRef
    // очистится через syncFollowTarget(null), и вещание возобновится.
    if (followTargetRef.current || applyingRemoteRef.current) return
    if (viewportTimerRef.current) return
    const send = () => {
      const api = apiRef.current
      const ws = wsRef.current
      if (!api || ws?.readyState !== WebSocket.OPEN) return
      if (ws.bufferedAmount > EPHEMERAL_BACKPRESSURE_BYTES) return
      const bounds = getVisibleSceneBounds(api.getAppState())
      ws.send(JSON.stringify({ type: 'viewport', payload: { bounds } }))
    }
    send()
    viewportTimerRef.current = setTimeout(() => {
      viewportTimerRef.current = null
      send() // trailing: финальная позиция после последнего скролла
    }, UPDATE_THROTTLE_MS)
  }, [])

  const sendFollow = useCallback(
    (target: string | null, action: 'FOLLOW' | 'UNFOLLOW') => {
      followTargetRef.current = action === 'FOLLOW' ? target : null
      if (action === 'UNFOLLOW') targetCamRef.current = null
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(
          JSON.stringify({ type: 'follow', payload: { target, action } })
        )
      }
    },
    []
  )

  // Единственный вход в follow: диффим appState.userToFollow на каждом onChange.
  // Клик по аватарке в UserList Excalidraw и клик в панели участников звонка
  // (CallParticipants пишет userToFollow через updateScene) дают одно и то же
  // изменение appState — а вот onUserFollow срабатывает ТОЛЬКО на первом, из-за
  // чего follow из звонка не доезжал до сервера и ведомый стоял на месте.
  const syncFollowTarget = useCallback(
    (target: string | null) => {
      if (target === followTargetRef.current) return
      sendFollow(target, target ? 'FOLLOW' : 'UNFOLLOW')
    },
    [sendFollow]
  )

  // Единственная точка, где карта files пополняется локально, — значит и
  // единственный повод её персистить. Раньше это делал дебаунс onChange, то есть
  // и на кадрах камеры, где files не менялись.
  const registerFile = useCallback(
    (fileId: string, url: string, mimeType: string) => {
      filesRef.current[fileId] = { url, mimeType }
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(
          JSON.stringify({ type: 'file', payload: { fileId, url, mimeType } })
        )
      }
      if (filesTimerRef.current) clearTimeout(filesTimerRef.current)
      filesTimerRef.current = setTimeout(() => {
        filesTimerRef.current = null
        persistFiles()
      }, FILES_DEBOUNCE_MS)
    },
    [persistFiles]
  )

  const sendMedia = useCallback((p: MediaPayload) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'media', payload: p }))
    }
  }, [])

  return {
    status,
    saveFailed,
    onApiReady,
    onChange,
    sendCursor,
    broadcastViewport,
    syncFollowTarget,
    registerFile,
    sendMedia,
  }
}
