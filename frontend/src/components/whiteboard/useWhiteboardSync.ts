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

type ConnStatus = 'connecting' | 'connected' | 'disconnected'

interface CursorInfo {
  peerId: string
  x: number
  y: number
}

interface SyncResult {
  store: ReturnType<typeof createTLStore>
  status: ConnStatus
  cursors: CursorInfo[]
  sendCursor: (x: number, y: number) => void
}

type WsDiff = {
  added?: Record<string, TLRecord>
  updated?: Record<string, [TLRecord, TLRecord]>
  removed?: Record<string, TLRecord>
}

const SNAPSHOT_DEBOUNCE_MS = 1000

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

  const sendSnapshot = useCallback(() => {
    if (wsRef.current?.readyState !== WebSocket.OPEN) return
    wsRef.current.send(
      JSON.stringify({ type: 'snapshot', payload: getSnapshot(store) })
    )
  }, [store])

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

  useEffect(() => {
    closedRef.current = false
    setStatus('connecting')
    setCursors([])
    connect()
    return () => {
      // Mark closed so a late `onclose` won't schedule a reconnect on a dead
      // socket / unmounted component.
      closedRef.current = true
      if (retryRef.current) {
        clearTimeout(retryRef.current)
        retryRef.current = null
      }
      if (snapshotTimerRef.current) {
        clearTimeout(snapshotTimerRef.current)
        snapshotTimerRef.current = null
      }
      // Best-effort final snapshot before tearing the socket down.
      sendSnapshot()
      const ws = wsRef.current
      wsRef.current = null
      ws?.close()
    }
  }, [connect, sendSnapshot])

  // Send local user changes over the WebSocket.
  useEffect(() => {
    const unsub = store.listen(
      ({ changes }) => {
        if (wsRef.current?.readyState === WebSocket.OPEN) {
          // Relay the diff immediately (not persisted by the hub).
          wsRef.current.send(JSON.stringify({ type: 'update', payload: changes }))
        }
        // Debounce a full-document snapshot send (persisted by the hub).
        if (snapshotTimerRef.current) clearTimeout(snapshotTimerRef.current)
        snapshotTimerRef.current = setTimeout(sendSnapshot, SNAPSHOT_DEBOUNCE_MS)
      },
      { source: 'user', scope: 'document' }
    )
    return unsub
  }, [store, sendSnapshot])

  const sendCursor = useCallback((x: number, y: number) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'cursor', x, y }))
    }
  }, [])

  return { store, status, cursors, sendCursor }
}
