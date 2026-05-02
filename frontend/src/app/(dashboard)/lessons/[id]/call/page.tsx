'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { LiveKitRoom, VideoConference } from '@livekit/components-react'
import '@livekit/components-styles'

import { callsApi, type RoomTokenResponse } from '@/lib/api/calls'
import { Button } from '@/components/ui/button'

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
        serverUrl={room.server_url}
        token={room.token}
        video={true}
        audio={true}
        onDisconnected={() => router.back()}
        data-lk-theme="default"
        style={{ height: '100%' }}
      >
        <VideoConference />
      </LiveKitRoom>
    </div>
  )
}
