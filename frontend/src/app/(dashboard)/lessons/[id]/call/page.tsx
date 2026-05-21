'use client'

import { Component, useState, type ReactNode } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { LiveKitRoom, VideoConference } from '@livekit/components-react'
import '@livekit/components-styles'

import { callsApi, type RoomTokenResponse } from '@/lib/api/calls'
import { Button } from '@/components/ui/button'
import { Link, Check } from 'lucide-react'

class VideoConferenceBoundary extends Component<
  { children: ReactNode },
  { hasError: boolean }
> {
  state = { hasError: false }

  static getDerivedStateFromError() {
    return { hasError: true }
  }

  componentDidCatch() {
    setTimeout(() => this.setState({ hasError: false }), 0)
  }

  render() {
    return this.state.hasError ? null : this.props.children
  }
}

type Stage = 'idle' | 'starting' | 'connecting' | 'in-room'

export default function CallPage() {
  const { id } = useParams<{ id: string }>()
  const router  = useRouter()

  const [stage, setStage]   = useState<Stage>('idle')
  const [room, setRoom]     = useState<RoomTokenResponse | null>(null)
  const [error, setError]   = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  function handleCopyLink() {
    const url = `${window.location.origin}/join/${id}`
    navigator.clipboard.writeText(url)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  async function handleStart() {
    setStage('starting')
    setError(null)
    try {
      await callsApi.startRoom(id)
      setStage('connecting')
      const data = await callsApi.getRoomToken(id)
      setRoom(data)
      setStage('in-room')
    } catch {
      setError('Не удалось запустить урок')
      setStage('idle')
    }
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-[80vh] gap-4">
        <p className="text-destructive">{error}</p>
        <Button variant="outline" onClick={() => router.back()}>Назад</Button>
      </div>
    )
  }

  if (stage === 'idle') {
    return (
      <div className="flex flex-col items-center justify-center h-[80vh] gap-4">
        <p className="text-muted-foreground">Нажмите кнопку, чтобы открыть комнату для учеников</p>
        <Button onClick={handleStart}>Начать урок</Button>
      </div>
    )
  }

  if (!room) {
    return (
      <div className="flex items-center justify-center h-[80vh]">
        <p className="text-muted-foreground">
          {stage === 'starting' ? 'Открываем комнату...' : 'Подключение...'}
        </p>
      </div>
    )
  }

  return (
    <div style={{ height: 'calc(100vh - 64px)', position: 'relative' }}>
      <LiveKitRoom
        key={room.token}
        serverUrl={room.server_url}
        token={room.token}
        onDisconnected={() => router.back()}
        data-lk-theme="default"
        style={{ height: '100%' }}
      >
        <VideoConferenceBoundary>
          <VideoConference />
        </VideoConferenceBoundary>
      </LiveKitRoom>

      <Button
        variant="secondary"
        size="icon"
        onClick={handleCopyLink}
        title="Скопировать ссылку для ученика"
        style={{ position: 'absolute', top: '12px', right: '12px', zIndex: 50 }}
      >
        {copied ? <Check className="h-4 w-4" /> : <Link className="h-4 w-4" />}
      </Button>
    </div>
  )
}
