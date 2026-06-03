'use client'

import { Component, useEffect, useState, type ReactNode } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { LiveKitRoom, VideoConference } from '@livekit/components-react'
import '@livekit/components-styles'

import { callsApi } from '@/lib/api/calls'
import { Check, Copy, Loader2 } from 'lucide-react'

class VideoConferenceBoundary extends Component<
  { children: ReactNode },
  { hasError: boolean }
> {
  state = { hasError: false }
  static getDerivedStateFromError() { return { hasError: true } }
  componentDidCatch() { setTimeout(() => this.setState({ hasError: false }), 0) }
  render() { return this.state.hasError ? null : this.props.children }
}

type Stage = 'loading' | 'in-room' | 'error'

export default function QuickRoomPage() {
  const { id }  = useParams<{ id: string }>()
  const router  = useRouter()

  const [stage, setStage]         = useState<Stage>('loading')
  const [token, setToken]         = useState<string | null>(null)
  const [serverUrl, setServerUrl] = useState<string | null>(null)
  const [copied, setCopied]       = useState(false)

  const joinUrl = typeof window !== 'undefined'
    ? `${window.location.origin}/join/room/${id}`
    : `/join/room/${id}`

  function handleCopy() {
    navigator.clipboard.writeText(joinUrl)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  useEffect(() => {
    const raw = sessionStorage.getItem(`quick-room-${id}`)
    if (!raw) { router.replace('/dashboard'); return }
    try {
      const { token: t, server_url: s } = JSON.parse(raw) as { token: string; server_url: string }
      setToken(t)
      setServerUrl(s)
      setStage('in-room')
    } catch {
      router.replace('/dashboard')
    }
  }, [id, router])

  async function handleDisconnected() {
    try { await callsApi.endQuickRoom(id) } catch {}
    sessionStorage.removeItem(`quick-room-${id}`)
    router.replace('/dashboard')
  }

  if (stage === 'loading') {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '80vh' }}>
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (stage === 'error' || !token || !serverUrl) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '80vh', gap: 16 }}>
        <p className="text-muted-foreground text-sm">Не удалось подключиться к комнате</p>
        <button
          onClick={() => router.replace('/dashboard')}
          style={{ padding: '8px 16px', borderRadius: 8, border: '1px solid var(--border)', cursor: 'pointer', fontSize: 13 }}
        >
          На главную
        </button>
      </div>
    )
  }

  return (
    <div style={{ height: 'calc(100vh - 64px)', position: 'relative' }}>
      <LiveKitRoom
        key={token}
        serverUrl={serverUrl}
        token={token}
        onDisconnected={handleDisconnected}
        data-lk-theme="default"
        style={{ height: '100%' }}
      >
        <VideoConferenceBoundary>
          <VideoConference />
        </VideoConferenceBoundary>
      </LiveKitRoom>

      <button
        onClick={handleCopy}
        title="Скопировать ссылку для ученика"
        style={{
          position: 'absolute', top: 12, right: 12, zIndex: 50,
          display: 'flex', alignItems: 'center', gap: 6,
          padding: '6px 12px', borderRadius: 8,
          background: 'rgba(0,0,0,0.55)', backdropFilter: 'blur(8px)',
          border: '1px solid rgba(255,255,255,0.12)',
          color: '#fff', fontSize: 12, fontWeight: 600, cursor: 'pointer',
        }}
      >
        {copied ? <Check size={13} /> : <Copy size={13} />}
        {copied ? 'Скопировано' : 'Ссылка для ученика'}
      </button>
    </div>
  )
}
