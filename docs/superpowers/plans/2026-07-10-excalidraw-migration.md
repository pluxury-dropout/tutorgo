# Excalidraw Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Заменить tldraw на Excalidraw в доске, сохранив Go WS-хаб и протокол `snapshot`/`update`/`cursor` без изменений.

**Architecture:** Excalidraw не имеет sync-store — синхронизация строится на `onChange`-диффе по `id:version` (исходящие `update`) и `reconcileElements()` (входящие). Картинки живут в S3; в снапшоте и WS ходит только карта-указатель `fileId → {url, mimeType}`, base64 запрещён (у хаба `ReadLimit` 512 КБ). Бэкенд не трогается вообще.

**Tech Stack:** Next.js 16, React 19, `@excalidraw/excalidraw@^0.18.1`, node:test (Node 24, нативный TS-strip), существующий Go WS-хаб.

**Spec:** `docs/superpowers/specs/2026-07-10-excalidraw-migration-design.md`

## Global Constraints

- WS-протокол не меняется: `snapshot` (персистится хабом), `update` (ретранслируется), `cursor` (`x`/`y` в корне сообщения, хаб добавляет `peerId`), новый тип `file` уходит в `default`-ветку хаба и ретранслируется verbatim. Go-код не редактировать.
- Ни одно WS-сообщение не может превышать 512 КБ (`SetReadLimit` в `handlers/whiteboard_ws.go:164`). Никакого base64 в `snapshot`/`update`/`file`.
- Старые tldraw-снапшоты (`payload.document.store`) не конвертируются: фронт распознаёт формат и стартует с чистой доски.
- node:test-файлы НЕ должны импортировать `@excalidraw/excalidraw` (ESM+CSS не резолвится в node) — только локальные модули со структурными типами.
- Тесты запускаются из `frontend/`: `node --test src/components/whiteboard/<file>.test.ts`.
- Типы Excalidraw импортируются так (проверено по exports map 0.18.1): значения из `@excalidraw/excalidraw`, типы из `@excalidraw/excalidraw/types` и `@excalidraw/excalidraw/element/types`, CSS — `@excalidraw/excalidraw/index.css`.
- Комментарии в коде — на русском, в стиле существующих файлов.

## File Structure

| Файл | Судьба | Ответственность |
|---|---|---|
| `frontend/src/components/whiteboard/excalidrawSync.ts` | создать | чистые функции: дифф версий, парсинг снапшота, blob→dataURL |
| `frontend/src/components/whiteboard/excalidrawSync.test.ts` | создать | node:test на чистые функции |
| `frontend/src/components/whiteboard/useExcalidrawSync.ts` | создать | WS-хук: connect/reconnect, reconcile, файлы, курсоры |
| `frontend/src/components/whiteboard/ExcalidrawCanvas.tsx` | создать | канвас: Excalidraw + вставка картинок/PDF + меню страниц |
| `frontend/src/app/(dashboard)/boards/[courseId]/page.tsx` | изменить | dynamic-импорт ExcalidrawCanvas |
| `frontend/src/app/board/join/[token]/page.tsx` | изменить | dynamic-импорт ExcalidrawCanvas |
| `frontend/src/components/call/CallRoom.tsx` | изменить | dynamic-импорт ExcalidrawCanvas |
| `TldrawCanvas.tsx`, `useWhiteboardSync.ts`, `BoardUi.tsx`, `boardTools.ts`, `boardTools.test.ts` | удалить | — |
| `frontend/package.json` | изменить | −`@tldraw/tldraw`, +`@excalidraw/excalidraw` |

Не трогаются: `BoardContext.tsx`, `BoardPageMenu.tsx`, `InviteSharePanel.tsx`, `PdfRangeDialog.tsx`, `lib/pdf.ts`, `lib/pdfRange.ts`, `lib/api/whiteboard.ts`, весь Go-бэкенд.

---

### Task 1: Чистые sync-хелперы (`excalidrawSync.ts`)

**Files:**
- Create: `frontend/src/components/whiteboard/excalidrawSync.ts`
- Test: `frontend/src/components/whiteboard/excalidrawSync.test.ts`

**Interfaces:**
- Consumes: ничего (нулевые зависимости, намеренно).
- Produces (используется Task 2 и 3):
  - `interface VersionedElement { id: string; version: number }`
  - `type SnapshotFiles = Record<string, { url: string; mimeType: string }>`
  - `diffChangedElements<T extends VersionedElement>(prev: ReadonlyMap<string, number>, elements: readonly T[]): { changed: T[]; next: Map<string, number> }`
  - `parseSnapshot(payload: unknown): { elements: VersionedElement[]; files: SnapshotFiles } | null`
  - `blobToDataURL(blob: Blob): Promise<string>` (без теста — FileReader браузерный)

- [ ] **Step 1: Написать падающий тест**

`frontend/src/components/whiteboard/excalidrawSync.test.ts`:

```ts
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { diffChangedElements, parseSnapshot } from './excalidrawSync.ts'

test('diffChangedElements: новые и изменённые элементы попадают в changed', () => {
  const prev = new Map([['a', 1], ['b', 2]])
  const els = [
    { id: 'a', version: 1 }, // не изменился
    { id: 'b', version: 3 }, // изменился
    { id: 'c', version: 1 }, // новый
  ]
  const { changed, next } = diffChangedElements(prev, els)
  assert.deepEqual(changed.map((e) => e.id), ['b', 'c'])
  assert.equal(next.get('a'), 1)
  assert.equal(next.get('b'), 3)
  assert.equal(next.get('c'), 1)
})

test('diffChangedElements: пустой prev — все элементы changed', () => {
  const { changed } = diffChangedElements(new Map(), [{ id: 'a', version: 5 }])
  assert.equal(changed.length, 1)
})

test('diffChangedElements: без изменений — changed пуст', () => {
  const prev = new Map([['a', 1]])
  const { changed } = diffChangedElements(prev, [{ id: 'a', version: 1 }])
  assert.equal(changed.length, 0)
})

test('parseSnapshot: новый формат с elements и files', () => {
  const snap = parseSnapshot({
    elements: [{ id: 'a', version: 1 }],
    files: { f1: { url: '/assets/x', mimeType: 'image/png' } },
  })
  assert.ok(snap)
  assert.equal(snap.elements.length, 1)
  assert.equal(snap.files.f1.url, '/assets/x')
})

test('parseSnapshot: files отсутствует — пустая карта', () => {
  const snap = parseSnapshot({ elements: [] })
  assert.ok(snap)
  assert.deepEqual(snap.files, {})
})

test('parseSnapshot: старый tldraw-снапшот → null (чистая доска)', () => {
  assert.equal(parseSnapshot({ document: { store: { 'shape:x': {} } } }), null)
  assert.equal(parseSnapshot(null), null)
  assert.equal(parseSnapshot('garbage'), null)
  assert.equal(parseSnapshot({ elements: 'not-array' }), null)
})
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `cd frontend && node --test src/components/whiteboard/excalidrawSync.test.ts`
Expected: FAIL — `Cannot find module ... excalidrawSync.ts`

- [ ] **Step 3: Реализация**

`frontend/src/components/whiteboard/excalidrawSync.ts`:

```ts
// Чистые функции синхронизации Excalidraw — вынесены из хука ради node:test.
// Намеренно без импортов из @excalidraw/excalidraw: структурного { id, version }
// достаточно, а node:test не резолвит ESM+CSS этого пакета.

export interface VersionedElement {
  id: string
  version: number
}

// Карта-указатель на файлы картинок: данные живут в S3, здесь только URL.
// Base64 в снапшоты класть нельзя — у Go-хаба ReadLimit 512 КБ на сообщение.
export type SnapshotFiles = Record<string, { url: string; mimeType: string }>

// Возвращает элементы, чья версия изменилась или которых не было в prev,
// и новую карту версий. Excalidraw бампает version на каждую правку, включая
// удаление (isDeleted: true — tombstone едет как обычный update).
export function diffChangedElements<T extends VersionedElement>(
  prev: ReadonlyMap<string, number>,
  elements: readonly T[]
): { changed: T[]; next: Map<string, number> } {
  const next = new Map<string, number>()
  const changed: T[] = []
  for (const el of elements) {
    next.set(el.id, el.version)
    if (prev.get(el.id) !== el.version) changed.push(el)
  }
  return { changed, next }
}

// Снапшот нового формата: { elements: [...], files?: {...} }.
// Старые tldraw-снапшоты ({ document: { store } }) и мусор → null:
// доска стартует с чистого листа (решение из спеки — конвертер не пишем).
export function parseSnapshot(
  payload: unknown
): { elements: VersionedElement[]; files: SnapshotFiles } | null {
  if (typeof payload !== 'object' || payload === null) return null
  const p = payload as { elements?: unknown; files?: unknown }
  if (!Array.isArray(p.elements)) return null
  return {
    elements: p.elements as VersionedElement[],
    files: (p.files as SnapshotFiles | undefined) ?? {},
  }
}

// FileReader — браузерный API, в node:test не гоняется (и не нужно).
export function blobToDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(blob)
  })
}
```

- [ ] **Step 4: Тест зелёный**

Run: `cd frontend && node --test src/components/whiteboard/excalidrawSync.test.ts`
Expected: PASS, 6 tests

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/whiteboard/excalidrawSync.ts frontend/src/components/whiteboard/excalidrawSync.test.ts
git commit -m "feat(board): pure sync helpers for excalidraw migration"
```

---

### Task 2: Зависимость + WS-хук `useExcalidrawSync`

**Files:**
- Modify: `frontend/package.json` (только добавить `@excalidraw/excalidraw`; `@tldraw/tldraw` удаляется в Task 5)
- Create: `frontend/src/components/whiteboard/useExcalidrawSync.ts`

**Interfaces:**
- Consumes: Task 1 (`diffChangedElements`, `parseSnapshot`, `blobToDataURL`, `SnapshotFiles`); существующие `getWsUrl(pageId, token?)` и `getTokenAsync(token?)` из `@/lib/api`.
- Produces (используется Task 3):

```ts
interface ExcalidrawSyncResult {
  status: 'connecting' | 'connected' | 'disconnected'
  // Отдать сюда api из пропа excalidrawAPI. До этого снапшоты буферизуются.
  onApiReady: (api: ExcalidrawImperativeAPI) => void
  // Подключить к пропу onChange Excalidraw.
  onChange: () => void
  // Подключить к пропу onPointerUpdate (шлёт cursor с троттлингом).
  sendCursor: (x: number, y: number) => void
  // Зарегистрировать загруженный в S3 файл: кладёт в карту files,
  // рассылает пирам WS-сообщение file.
  registerFile: (fileId: string, url: string, mimeType: string) => void
}
function useExcalidrawSync(page: BoardPage | null, token?: string): ExcalidrawSyncResult
```

- [ ] **Step 1: Установить зависимость**

```bash
cd frontend && npm install @excalidraw/excalidraw@^0.18.1
```

Expected: `package.json` содержит `"@excalidraw/excalidraw": "^0.18.1"`, install без peer-конфликтов (React 19 поддержан официально).

- [ ] **Step 2: Написать хук**

`frontend/src/components/whiteboard/useExcalidrawSync.ts`:

```ts
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
  OrderedExcalidrawElement,
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
      retryRef.current = setTimeout(() => {
        void connect()
      }, 2000)
    }

    ws.onerror = () => ws.close()
  }, [pageId, token, applySnapshot, applyRemote, hydrateFiles])

  useEffect(() => {
    closedRef.current = false
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
```

- [ ] **Step 3: Проверить сборку типов**

Run: `cd frontend && npx tsc --noEmit 2>&1 | grep -v node_modules | head -20`
Expected: ошибок в `useExcalidrawSync.ts` нет (могут быть только существующие ошибки других файлов, если такие были — сравнить с `git stash`-базой не требуется, файл новый).

Если импорт `RemoteExcalidrawElement` из `@excalidraw/excalidraw/data/reconcile` не резолвится (exports map отдаёт `./dist/types/excalidraw/*.d.ts` — путь `data/reconcile` валиден, проверено), запасной вариант — локальный каст: `remote as unknown as Parameters<typeof reconcileElements>[1]`.

- [ ] **Step 4: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/src/components/whiteboard/useExcalidrawSync.ts
git commit -m "feat(board): excalidraw dep + WS sync hook (reconcileElements, S3 file map)"
```

---

### Task 3: Компонент `ExcalidrawCanvas`

**Files:**
- Create: `frontend/src/components/whiteboard/ExcalidrawCanvas.tsx`

**Interfaces:**
- Consumes: Task 2 (`useExcalidrawSync`), Task 1 (`blobToDataURL`), существующие `PdfRangeDialog` (пропы `open`, `numPages`, `progress`, `onConfirm(from, to)`, `onCancel`), `BoardPageMenu` (без пропов, читает `useBoardContext`), `BoardContextProvider`, `loadPdf`/`renderPages` из `@/lib/pdf`, `whiteboardApi.uploadAsset(boardId, file)` → `{ id, url }`, `BASE_URL`.
- Produces: `export function ExcalidrawCanvas(props: Props)` — пропы **идентичны** нынешнему `TldrawCanvas` (`page`, `token?`, `boardId`, `pages`, `activePageId`, `onSelectPage`, `courseId?`, `isGuest?`), чтобы точки использования менялись минимально.

- [ ] **Step 1: Написать компонент**

`frontend/src/components/whiteboard/ExcalidrawCanvas.tsx`:

```tsx
'use client'

import { useRef, useState, useEffect, useCallback } from 'react'
import {
  Excalidraw,
  convertToExcalidrawElements,
  viewportCoordsToSceneCoords,
  CaptureUpdateAction,
} from '@excalidraw/excalidraw'
import '@excalidraw/excalidraw/index.css'
import { toast } from 'sonner'
import type {
  ExcalidrawImperativeAPI,
  BinaryFileData,
  DataURL,
} from '@excalidraw/excalidraw/types'
import type { FileId } from '@excalidraw/excalidraw/element/types'
import type { PDFDocumentProxy } from 'pdfjs-dist'
import { useExcalidrawSync } from './useExcalidrawSync'
import { blobToDataURL } from './excalidrawSync'
import { BoardContextProvider } from './BoardContext'
import { BoardPageMenu } from './BoardPageMenu'
import { PdfRangeDialog } from './PdfRangeDialog'
import { loadPdf, renderPages } from '@/lib/pdf'
import { whiteboardApi, BASE_URL } from '@/lib/api/whiteboard'
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

export function ExcalidrawCanvas({
  page,
  token,
  boardId,
  pages,
  activePageId,
  onSelectPage,
  courseId,
  isGuest = false,
}: Props) {
  const { status, onApiReady, onChange, sendCursor, registerFile } =
    useExcalidrawSync(page, token)
  const apiRef = useRef<ExcalidrawImperativeAPI | null>(null)

  const pdfRef = useRef<PDFDocumentProxy | null>(null)
  const imageInputRef = useRef<HTMLInputElement | null>(null)
  const [pdfDialog, setPdfDialog] = useState<{
    numPages: number
    point: { x: number; y: number }
  } | null>(null)
  const [pdfProgress, setPdfProgress] = useState<{
    done: number
    total: number
  } | null>(null)
  const [pagesOpen, setPagesOpen] = useState(false)

  // Закрытие меню страниц по клику вне.
  useEffect(() => {
    if (!pagesOpen) return
    function onDown(e: PointerEvent) {
      if ((e.target as HTMLElement).closest('[data-board-ui]')) return
      setPagesOpen(false)
    }
    document.addEventListener('pointerdown', onDown, true)
    return () => document.removeEventListener('pointerdown', onDown, true)
  }, [pagesOpen])

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
    } catch {
      toast.error('Не удалось вставить картинку')
    }
  }

  const handlePdfConfirm = async (from: number, to: number) => {
    const pdf = pdfRef.current
    const origin = pdfDialog?.point
    if (!pdf || !origin) return
    setPdfProgress({ done: 0, total: to - from + 1 })
    try {
      const rendered = await renderPages(pdf, from, to, (done, total) =>
        setPdfProgress({ done, total })
      )
      let x = origin.x
      for (let i = 0; i < rendered.length; i++) {
        const p = rendered[i]
        try {
          await insertImageBlob(
            p.blob,
            'image/png',
            { x, y: origin.y },
            { w: p.width, h: p.height },
            `page-${from + i}.png`
          )
        } catch (e) {
          const msg = (e as { message?: string })?.message
          toast.error(
            `Не удалось загрузить страницу ${from + i}${msg ? `: ${msg}` : ''}`
          )
          break
        }
        x += p.width // встык по горизонтали
      }
    } catch {
      toast.error('Не удалось обработать PDF')
    } finally {
      void pdf.cleanup()
      pdfRef.current = null
      setPdfProgress(null)
      setPdfDialog(null)
    }
  }

  // Перехват drop PDF ДО Excalidraw (у него нет хука на drop; capture-фаза
  // обёртки срабатывает раньше). Не-PDF пропускаем — нативная вставка
  // картинок Excalidraw кладёт base64 в files; для брошенных мышкой мелких
  // картинок приемлемо, наша кнопка вставки идёт через S3.
  // ponytail: фоновая догрузка таких base64-файлов в S3 — когда заметим раздутые снапшоты.
  const onDropCapture = (e: React.DragEvent) => {
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
        pdfRef.current = pdf
        setPdfDialog({ numPages: pdf.numPages, point })
      } catch {
        toast.error('Не удалось открыть PDF')
      }
    })()
  }

  return (
    <BoardContextProvider
      value={{ boardId, courseId, pages, activePageId, onSelectPage, isGuest }}
    >
      <div className="relative w-full h-full" onDropCapture={onDropCapture}>
        {status === 'disconnected' && (
          <div className="absolute top-2 left-1/2 -translate-x-1/2 z-10 bg-yellow-100 border border-yellow-300 text-yellow-800 text-sm px-3 py-1 rounded-full">
            Переподключение...
          </div>
        )}
        <Excalidraw
          key={page?.id ?? 'empty'}
          langCode="ru-RU"
          excalidrawAPI={(api) => {
            apiRef.current = api
            onApiReady(api)
          }}
          onChange={onChange}
          onPointerUpdate={(p) => sendCursor(p.pointer.x, p.pointer.y)}
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
              <button
                onClick={() => setPagesOpen((v) => !v)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 6,
                  border: 'none',
                  background: 'transparent',
                  borderRadius: 8,
                  cursor: 'pointer',
                  fontSize: 13,
                  fontWeight: 600,
                  padding: '6px 10px',
                }}
              >
                Урок
                <svg
                  width="13"
                  height="13"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <path d="M6 9l6 6 6-6" />
                </svg>
              </button>
              {pagesOpen && (
                <div
                  style={{ position: 'absolute', top: 40, right: 0, zIndex: 20 }}
                  onPointerDown={(e) => e.stopPropagation()}
                >
                  <BoardPageMenu />
                </div>
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
```

Замечания для имплементера:
- `langCode="ru-RU"` — локаль подтверждена (чанк `ru-RU-*.js` есть в пакете). Если TS ругается на литерал — проверить `Language["code"]` в `node_modules/@excalidraw/excalidraw/dist/types/excalidraw/i18n.d.ts`.
- Проверить сигнатуру `PdfRangeDialog` по фактическому файлу перед использованием — пропы описаны в Interfaces, но файл первоисточник.

- [ ] **Step 2: Сборка типов**

Run: `cd frontend && npx tsc --noEmit 2>&1 | grep -v node_modules | head -20`
Expected: без новых ошибок.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/whiteboard/ExcalidrawCanvas.tsx
git commit -m "feat(board): ExcalidrawCanvas — canvas, S3 images, PDF glue, page menu"
```

---

### Task 4: Переключить точки использования

**Files:**
- Modify: `frontend/src/app/(dashboard)/boards/[courseId]/page.tsx:6`
- Modify: `frontend/src/app/board/join/[token]/page.tsx:6`
- Modify: `frontend/src/components/call/CallRoom.tsx:20`

**Interfaces:**
- Consumes: Task 3 (`ExcalidrawCanvas`, пропы идентичны `TldrawCanvas`).
- Produces: ничего нового — только замена импорта.

- [ ] **Step 1: Заменить импорты на dynamic**

Во всех трёх файлах заменить:

```ts
import { TldrawCanvas } from '@/components/whiteboard/TldrawCanvas'
```

на:

```ts
import dynamic from 'next/dynamic'

// Excalidraw трогает window при инициализации — только клиент, без SSR.
const ExcalidrawCanvas = dynamic(
  () =>
    import('@/components/whiteboard/ExcalidrawCanvas').then(
      (m) => m.ExcalidrawCanvas
    ),
  { ssr: false }
)
```

И в JSX: `<TldrawCanvas` → `<ExcalidrawCanvas` (пропы не меняются). В `CallRoom.tsx` найти JSX-использование на строке ~185.

- [ ] **Step 2: Сборка**

Run: `cd frontend && npm run build 2>&1 | tail -15`
Expected: build успешен (tldraw-файлы ещё на месте, но уже не импортируются ниоткуда, кроме самих себя).

- [ ] **Step 3: Commit**

```bash
git add "frontend/src/app/(dashboard)/boards/[courseId]/page.tsx" "frontend/src/app/board/join/[token]/page.tsx" frontend/src/components/call/CallRoom.tsx
git commit -m "feat(board): switch all canvas mounts to ExcalidrawCanvas (dynamic, no SSR)"
```

---

### Task 5: Удалить tldraw + финальная проверка

**Files:**
- Delete: `frontend/src/components/whiteboard/TldrawCanvas.tsx`
- Delete: `frontend/src/components/whiteboard/useWhiteboardSync.ts`
- Delete: `frontend/src/components/whiteboard/BoardUi.tsx`
- Delete: `frontend/src/components/whiteboard/boardTools.ts`
- Delete: `frontend/src/components/whiteboard/boardTools.test.ts`
- Modify: `frontend/package.json` (удалить `@tldraw/tldraw`)

**Interfaces:**
- Consumes: Task 4 (ни один файл больше не импортирует tldraw).
- Produces: репозиторий без tldraw.

- [ ] **Step 1: Убедиться, что tldraw больше нигде не используется**

Run: `cd frontend && grep -rn "tldraw\|TldrawCanvas\|useWhiteboardSync\|BoardUi\|boardTools" src --include="*.ts" --include="*.tsx" -l | grep -v "TldrawCanvas.tsx\|useWhiteboardSync.ts\|BoardUi.tsx\|boardTools"`
Expected: пусто (упоминания только внутри удаляемых файлов).

- [ ] **Step 2: Удалить файлы и зависимость**

```bash
cd frontend
git rm src/components/whiteboard/TldrawCanvas.tsx src/components/whiteboard/useWhiteboardSync.ts src/components/whiteboard/BoardUi.tsx src/components/whiteboard/boardTools.ts src/components/whiteboard/boardTools.test.ts
npm uninstall @tldraw/tldraw
```

- [ ] **Step 3: Найти остатки NEXT_PUBLIC_TLDRAW_LICENSE_KEY**

Run: `grep -rn "TLDRAW_LICENSE" .. --include="*.ts*" --include="*.env*" --include="*.md" 2>/dev/null | grep -v node_modules | grep -v docs/`
Expected: пусто в коде. Если ключ есть в `.env.local`/`.env` — удалить строку. **Деплой-env (Vercel/Railway) чистится вручную пользователем — напомнить в отчёте.**

- [ ] **Step 4: Полная проверка**

```bash
cd frontend
node --test src/components/whiteboard/excalidrawSync.test.ts
npm run lint 2>&1 | tail -5
npm run build 2>&1 | tail -10
```

Expected: тесты PASS, lint чистый, build успешен.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(board)!: remove tldraw — Excalidraw migration complete"
```

- [ ] **Step 6: Ручной smoke-чеклист (нужны бэкенд + БД)**

Не автоматизируется — прогнать вручную или отметить как carry-forward:
1. Два окна на одной доске — рисование видно в обе стороны, штрихи не затираются.
2. Reload — контент восстановился (снапшот из Postgres).
3. Вставка картинки кнопкой — видна во втором окне и после reload.
4. Drop PDF → диалог диапазона → страницы легли встык, видны во втором окне.
5. Invite-ссылка: гость рисует, кнопки вставки картинки у него нет.
6. Доска со старым tldraw-снапшотом открывается чистой, без ошибок в консоли.
7. Курсор второго участника виден (нативные collaborators).
