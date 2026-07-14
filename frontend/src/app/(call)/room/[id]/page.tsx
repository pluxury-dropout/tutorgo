'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import '@livekit/components-styles'
import { Loader2 } from 'lucide-react'

import { callsApi } from '@/lib/api/calls'
import { CallRoom } from '@/components/call/CallRoom'

type Stage = 'loading' | 'in-room' | 'error'

export default function QuickRoomPage() {
  const { id }  = useParams<{ id: string }>()
  const router  = useRouter()

  const [stage, setStage]         = useState<Stage>('loading')
  const [token, setToken]         = useState<string | null>(null)
  const [serverUrl, setServerUrl] = useState<string | null>(null)

  useEffect(() => {
    // LiveKit bug: placeholder→real track transition triggers a spurious console.error
    const orig = console.error.bind(console)
    console.error = (...args: unknown[]) => {
      if (typeof args[0] === 'string' && args[0].includes('Element not part of the array')) return
      orig(...args)
    }
    return () => { console.error = orig }
  }, [])

  useEffect(() => {
    const raw = sessionStorage.getItem(`quick-room-${id}`)
    if (!raw) { router.replace('/trial'); return }
    try {
      const { token: t, server_url: s } = JSON.parse(raw) as { token: string; server_url: string }
      setToken(t)
      setServerUrl(s)
      setStage('in-room')
    } catch {
      router.replace('/trial')
    }
  }, [id, router])

  async function handleDisconnected() {
    try { await callsApi.endQuickRoom(id) } catch {}
    sessionStorage.removeItem(`quick-room-${id}`)
    router.replace('/trial')
  }

  if (stage === 'loading') {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100dvh' }}>
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (stage === 'error' || !token || !serverUrl) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100dvh', gap: 16 }}>
        <p className="text-muted-foreground text-sm">Не удалось подключиться к комнате</p>
        <button
          onClick={() => router.replace('/trial')}
          style={{ padding: '8px 16px', borderRadius: 8, border: '1px solid var(--border)', cursor: 'pointer', fontSize: 13 }}
        >
          К пробным урокам
        </button>
      </div>
    )
  }

  const inviteUrl = `${window.location.origin}/join/room/${id}`

  return (
    <div style={{ height: '100dvh', position: 'relative' }}>
      <CallRoom
        trial
        serverUrl={serverUrl}
        token={token}
        role="tutor"
        inviteUrl={inviteUrl}
        onDisconnected={handleDisconnected}
      />
    </div>
  )
}
