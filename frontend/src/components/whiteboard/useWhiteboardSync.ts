'use client'

import { useEffect, useRef, useState, useCallback } from 'react'
import {
  createTLStore,
  defaultShapeUtils,
  type TLRecord,
} from '@tldraw/tldraw'
import { getWsUrl } from '@/lib/api/whiteboard'
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

type WsPayload = {
  added?: Record<string, TLRecord>
  updated?: Record<string, [TLRecord, TLRecord]>
  removed?: Record<string, TLRecord>
}

export function useWhiteboardSync(page: BoardPage | null, token?: string): SyncResult {
  const [status, setStatus] = useState<ConnStatus>('connecting')
  const [cursors, setCursors] = useState<CursorInfo[]>([])
  const wsRef = useRef<WebSocket | null>(null)
  const retryRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const [store] = useState(() =>
    createTLStore({ shapeUtils: [...defaultShapeUtils] })
  )

  const connect = useCallback(() => {
    if (!page) return
    const ws = new WebSocket(getWsUrl(page.id, token))
    wsRef.current = ws

    ws.onopen = () => setStatus('connected')

    ws.onmessage = (e: MessageEvent) => {
      const msg = JSON.parse(e.data as string) as {
        type: string
        payload?: WsPayload
        peerId?: string
        x?: number
        y?: number
      }

      if ((msg.type === 'snapshot' || msg.type === 'update') && msg.payload) {
        store.mergeRemoteChanges(() => {
          const { added, updated, removed } = msg.payload!
          if (added) store.put(Object.values(added))
          if (updated)
            store.put(Object.values(updated).map(([, next]) => next))
          if (removed)
            store.remove(Object.keys(removed) as Array<TLRecord['id']>)
        })
      }

      if (msg.type === 'cursor' && msg.peerId) {
        setCursors((prev) => {
          const others = prev.filter((c) => c.peerId !== msg.peerId)
          return [...others, { peerId: msg.peerId!, x: msg.x ?? 0, y: msg.y ?? 0 }]
        })
      }
    }

    ws.onclose = () => {
      setStatus('disconnected')
      retryRef.current = setTimeout(connect, 2000)
    }

    ws.onerror = () => ws.close()
  }, [page, token, store])

  useEffect(() => {
    connect()
    return () => {
      if (retryRef.current) clearTimeout(retryRef.current)
      wsRef.current?.close()
    }
  }, [connect])

  // Send user changes over WebSocket
  useEffect(() => {
    const unsub = store.listen(
      ({ changes }) => {
        if (wsRef.current?.readyState !== WebSocket.OPEN) return
        wsRef.current.send(JSON.stringify({ type: 'update', payload: changes }))
      },
      { source: 'user', scope: 'document' }
    )
    return unsub
  }, [store])

  const sendCursor = useCallback((x: number, y: number) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'cursor', x, y }))
    }
  }, [])

  return { store, status, cursors, sendCursor }
}
