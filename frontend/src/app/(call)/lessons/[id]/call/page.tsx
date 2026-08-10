'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { DisconnectReason } from 'livekit-client'
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
        // start-room только помечает урок активным для ученика, токен от него
        // не зависит — гоняем оба запроса разом.
        const [, data, lesson] = await Promise.all([
          callsApi.startRoom(id),
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

  // Урок завершаем только при осознанном выходе (кнопка «Выйти» → room.disconnect()
  // → CLIENT_INITIATED). Рефреш/обрыв сети сюда тоже прилетают, но комнату не
  // трогаем: она остаётся active, при новой загрузке startRoom+getRoomToken
  // (идемпотентны) заходят заново, ученика не выкидывает. Пустую комнату закроет
  // LiveKit-webhook room_finished по empty_timeout.
  async function handleDisconnected(reason?: DisconnectReason) {
    if (reason !== DisconnectReason.CLIENT_INITIATED) return
    try { await callsApi.endRoom(id) } catch {}
    // replace, а не back(): урок всегда открывается в новой вкладке
    // (window.open в LessonQuickDialog/Sidebar), там history пуст и back() — no-op.
    router.replace('/dashboard')
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-[80vh] gap-4">
        <p className="text-destructive">{error}</p>
        <Button variant="outline" onClick={() => router.replace('/dashboard')}>Назад</Button>
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

  const inviteUrl = `${window.location.origin}/student/lessons/${id}/call`

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
