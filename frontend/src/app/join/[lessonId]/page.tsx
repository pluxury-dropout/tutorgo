'use client'

import { useState } from 'react'
import { useParams } from 'next/navigation'
import { LiveKitRoom, VideoConference } from '@livekit/components-react'
import '@livekit/components-styles'

import { callsApi, type RoomTokenResponse } from '@/lib/api/calls'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { GraduationCap } from 'lucide-react'

export default function JoinPage() {
  const { lessonId } = useParams<{ lessonId: string }>()

  const [name, setName]     = useState('')
  const [room, setRoom]     = useState<RoomTokenResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError]   = useState<string | null>(null)

  async function handleJoin() {
    if (!name.trim()) return
    setLoading(true)
    setError(null)
    try {
      const data = await callsApi.getGuestToken(lessonId)
      setRoom(data)
    } catch {
      setError('Не удалось подключиться. Проверьте ссылку.')
    } finally {
      setLoading(false)
    }
  }

  if (room) {
    return (
      <div style={{ height: '100dvh' }}>
        <LiveKitRoom
          serverUrl={room.server_url}
          token={room.token}
          video={true}
          audio={true}
          onDisconnected={() => setRoom(null)}
          data-lk-theme="default"
          style={{ height: '100%' }}
        >
          <VideoConference />
        </LiveKitRoom>
      </div>
    )
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="flex items-center gap-2 justify-center">
          <GraduationCap className="h-6 w-6 text-primary" />
          <span className="font-semibold text-lg">TutorGo</span>
        </div>

        <div className="rounded-lg border p-6 space-y-4">
          <h1 className="text-base font-semibold">Присоединиться к уроку</h1>

          <div className="space-y-2">
            <label className="text-sm text-muted-foreground">Ваше имя</label>
            <Input
              placeholder="Введите ваше имя"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleJoin()}
              autoFocus
            />
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}

          <Button
            className="w-full"
            onClick={handleJoin}
            disabled={!name.trim() || loading}
          >
            {loading ? 'Подключение...' : 'Войти в урок'}
          </Button>
        </div>
      </div>
    </div>
  )
}
