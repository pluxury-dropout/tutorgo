'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import '@livekit/components-styles'

import { callsApi, type RoomTokenResponse } from '@/lib/api/calls'
import { lessonsApi } from '@/lib/api/lessons'
import { Button } from '@/components/ui/button'
import { CallRoom } from '@/components/call/CallRoom'

export default function CallPage() {
  const { id } = useParams<{ id: string }>()
  const router = useRouter()

  const [room, setRoom] = useState<RoomTokenResponse | null>(null)
  const [courseId, setCourseId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  // Auto-start on mount — репетитор уже начал звонок из календаря.
  // start-room идемпотентен, безопасно перезапускать при рефреше.
  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        await callsApi.startRoom(id)
        const [data, lesson] = await Promise.all([
          callsApi.getRoomToken(id),
          lessonsApi.get(id).catch(() => null),
        ])
        if (cancelled) return
        if (lesson?.course_id) setCourseId(lesson.course_id)
        setRoom(data)
      } catch {
        if (!cancelled) setError('Не удалось запустить урок')
      }
    })()
    return () => { cancelled = true }
  }, [id])

  async function handleDisconnected() {
    try { await callsApi.endRoom(id) } catch {}
    router.back()
  }

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

  const inviteUrl = `${window.location.origin}/join/${id}`

  return (
    <div style={{ height: '100dvh', position: 'relative' }}>
      <CallRoom
        courseId={courseId ?? undefined}
        serverUrl={room.server_url}
        token={room.token}
        role="tutor"
        inviteUrl={inviteUrl}
        onDisconnected={handleDisconnected}
      />
    </div>
  )
}
