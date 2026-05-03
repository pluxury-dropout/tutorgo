'use client'

import { Component, useEffect, useState, type ReactNode } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { LiveKitRoom, VideoConference } from '@livekit/components-react'
import '@livekit/components-styles'

import { callsApi, type RoomTokenResponse } from '@/lib/api/calls'
import { Button } from '@/components/ui/button'

// Catches the transient "Element not part of the array" error from LiveKit when
// a placeholder track is swapped for the real track, then remounts to recover.
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

export default function CallPage() {
  const { id } = useParams<{ id: string }>()
  const router  = useRouter()

  const [room, setRoom]   = useState<RoomTokenResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    callsApi.getRoomToken(id)
      .then(setRoom)
      .catch(() => setError('Не удалось подключиться к видеозвонку'))
  }, [id])

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-[80vh] gap-4">
        <p className="text-destructive">{error}</p>
        <Button variant="outline" onClick={() => router.back()}>Назад</Button>
      </div>
    )
  }

  if (!room) {
    return (
      <div className="flex items-center justify-center h-[80vh]">
        <p className="text-muted-foreground">Подключение...</p>
      </div>
    )
  }

  return (
    <div style={{ height: 'calc(100vh - 64px)' }}>
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
    </div>
  )
}
