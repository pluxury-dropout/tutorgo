# Whiteboard Reconnect Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix two bugs causing the "Переподключение..." banner to appear immediately on board load.

**Architecture:** Two targeted changes — `client.ts` gains a `refreshPromise` variable and exports `getTokenAsync()` so callers can await any in-flight token refresh; `useWhiteboardSync.ts` narrows the `connect` useCallback dependency from the full `page` object to the `page?.id` string primitive, and makes `connect` async to call `getTokenAsync` before opening the WebSocket.

**Tech Stack:** TypeScript, React hooks (useCallback, useMemo, useEffect, useRef), Axios interceptors, browser WebSocket API.

---

## Files

| File | Change |
|------|--------|
| `frontend/src/lib/api/client.ts` | Add `refreshPromise` module var; export `getTokenAsync()` |
| `frontend/src/components/whiteboard/useWhiteboardSync.ts` | `pageId` string dep; `connect` async + `getTokenAsync` call |

---

### Task 1: Export `getTokenAsync` from `client.ts`

**Files:**
- Modify: `frontend/src/lib/api/client.ts:20-37`

- [ ] **Step 1: Add `refreshPromise` variable and restructure `proactiveRefresh`**

Replace lines 20–37 in `client.ts` with:

```ts
let isRefreshing = false
let refreshPromise: Promise<void> | null = null

async function proactiveRefresh(): Promise<void> {
  if (isRefreshing) return
  isRefreshing = true
  const p = (async () => {
    try {
      const { data } = await axios.post<{ access_token: string }>(
        `${BASE_URL}/auth/refresh`,
        {},
        { withCredentials: true },
      )
      localStorage.setItem('tg_token', data.access_token)
    } catch {
      // silently ignore — reactive 401 handler will log out if needed
    }
  })()
  refreshPromise = p
  try {
    await p
  } finally {
    isRefreshing = false
    refreshPromise = null
  }
}

export async function getTokenAsync(fallback?: string): Promise<string | undefined> {
  if (refreshPromise) await refreshPromise
  return localStorage.getItem('tg_token') ?? fallback ?? undefined
}
```

The rest of the file (interceptors from line 39 onward) is **unchanged**.

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep client
```

Expected: no output (no errors in client.ts).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api/client.ts
git commit -m "feat: export getTokenAsync — waits for in-flight refresh before reading token"
```

---

### Task 2: Fix `connect` deps and make it async in `useWhiteboardSync.ts`

**Files:**
- Modify: `frontend/src/components/whiteboard/useWhiteboardSync.ts`

- [ ] **Step 1: Add `getTokenAsync` import**

Replace the import block at the top of `useWhiteboardSync.ts` (lines 1–12):

```ts
'use client'

import { useEffect, useRef, useState, useCallback, useMemo } from 'react'
import {
  createTLStore,
  defaultShapeUtils,
  getSnapshot,
  loadSnapshot,
  type TLRecord,
} from '@tldraw/tldraw'
import { getWsUrl } from '@/lib/api/whiteboard'
import { getTokenAsync } from '@/lib/api/client'
import type { BoardPage } from '@/types/api'
```

- [ ] **Step 2: Derive `pageId` string and key `store` on it**

Replace lines 37–53 (function signature through `store` useMemo):

```ts
export function useWhiteboardSync(page: BoardPage | null, token?: string): SyncResult {
  const [status, setStatus] = useState<ConnStatus>('connecting')
  const [cursors, setCursors] = useState<CursorInfo[]>([])
  const wsRef = useRef<WebSocket | null>(null)
  const retryRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const snapshotTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  // Guards against zombie reconnects after the effect has been cleaned up
  // (e.g. a late `onclose` firing once the component unmounted or the page switched).
  const closedRef = useRef(false)

  // Stable primitive — prevents connect from recreating on React Query refetches
  // that return a new object reference for the same page.
  const pageId = page?.id ?? null

  // Recreate the store per page so switching pages never merges one page's
  // content on top of another's. Keyed on pageId string.
  const store = useMemo(
    () => createTLStore({ shapeUtils: [...defaultShapeUtils] }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [pageId]
  )
```

- [ ] **Step 3: Replace `connect` with async version**

Replace lines 62–123 (`connect` useCallback through its closing bracket and deps):

```ts
  const connect = useCallback(async () => {
    if (!pageId) return
    // Explicitly close any previous socket before opening a new one so page
    // switches / reconnects can't leak parallel sockets.
    wsRef.current?.close()

    // Await any in-flight proactive token refresh so we never open a WS with
    // a stale token (the HTTP Axios interceptor refreshes async; WS skips it).
    const wsToken = await getTokenAsync(token)
    const ws = new WebSocket(getWsUrl(pageId, wsToken))
    wsRef.current = ws

    ws.onopen = () => {
      if (wsRef.current === ws) setStatus('connected')
    }

    ws.onmessage = (e: MessageEvent) => {
      const msg = JSON.parse(e.data as string) as {
        type: string
        payload?: WsDiff | unknown
        peerId?: string
        x?: number
        y?: number
      }

      if (msg.type === 'snapshot' && msg.payload) {
        // Full document load (seeded from DB or latest client snapshot).
        store.mergeRemoteChanges(() => {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          loadSnapshot(store, msg.payload as any)
        })
        return
      }

      if (msg.type === 'update' && msg.payload) {
        store.mergeRemoteChanges(() => {
          const { added, updated, removed } = msg.payload as WsDiff
          if (added) store.put(Object.values(added))
          if (updated)
            store.put(Object.values(updated).map(([, next]) => next))
          if (removed)
            store.remove(Object.keys(removed) as Array<TLRecord['id']>)
        })
        return
      }

      if (msg.type === 'cursor' && msg.peerId) {
        setCursors((prev) => {
          const others = prev.filter((c) => c.peerId !== msg.peerId)
          return [...others, { peerId: msg.peerId!, x: msg.x ?? 0, y: msg.y ?? 0 }]
        })
      }
    }

    ws.onclose = () => {
      // Ignore close events from sockets that were superseded by a newer
      // connection — otherwise a replaced socket's late onclose would tear
      // down the healthy one and loop reconnects forever.
      if (closedRef.current || wsRef.current !== ws) return
      setStatus('disconnected')
      retryRef.current = setTimeout(() => { connect() }, 2000)
    }

    ws.onerror = () => ws.close()
  }, [pageId, token, store])
```

Key changes vs original:
- `async` keyword on the callback
- `pageId` (string) instead of `page` (object) in dep array — prevents refetch-driven reconnects
- `const wsToken = await getTokenAsync(token)` before `new WebSocket(...)`
- `getWsUrl(pageId, wsToken)` uses the fresh token
- `retryRef.current = setTimeout(() => { connect() }, 2000)` — wraps async call in arrow function (prevents unhandled-rejection lint warnings)

The rest of the file (lines 125–174: `useEffect`, store listener, `sendCursor`, `return`) is **unchanged**.

- [ ] **Step 4: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep useWhiteboardSync
```

Expected: no output.

- [ ] **Step 5: Full TypeScript check**

```bash
cd frontend && npx tsc --noEmit
```

Expected: exit 0, no errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/whiteboard/useWhiteboardSync.ts
git commit -m "fix: stable pageId dep in connect + await token refresh before WS open

- page?.id (string) replaces page (object) in connect useCallback deps so
  React Query background refetches no longer trigger WS reconnects
- connect is now async and awaits getTokenAsync() before opening the socket,
  eliminating the race where a proactive refresh was in-flight while the WS
  read a stale token from localStorage"
```

---

### Task 3: Manual verification

- [ ] **Step 1: Start the app**

```bash
# Terminal 1 — backend
cd /home/dragonbrn/tutorgo && air

# Terminal 2 — frontend
cd /home/dragonbrn/tutorgo/frontend && npm run dev
```

- [ ] **Step 2: Open the board and confirm no banner**

Navigate to any course board (`/boards/<courseId>`). Confirm the "Переподключение..." yellow banner does NOT appear on initial load.

- [ ] **Step 3: Confirm page switching still works**

In the board, switch to a different page via the page menu. Confirm the board reconnects to the new page (brief "connecting" is acceptable) and loads the new page content.

- [ ] **Step 4: Confirm guest board works**

Open a guest invite link (`/board/join/<token>`). Confirm the board loads without the banner.
