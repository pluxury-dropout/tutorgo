'use client'

import { useEffect, useRef, useState } from 'react'
import Link from 'next/link'
import { useParams } from 'next/navigation'
import '@livekit/components-styles'

import { callsApi, type RoomTokenResponse } from '@/lib/api/calls'
import { studentApi } from '@/lib/api/student'
import { CallRoom } from '@/components/call/CallRoom'
import { buttonVariants } from '@/components/ui/button'
import { GraduationCap } from 'lucide-react'
import type { ApiError } from '@/types/api'

type Stage = 'waiting' | 'in-room' | 'ended' | 'forbidden'

export default function StudentCallPage() {
  const { id } = useParams<{ id: string }>()

  const [stage, setStage] = useState<Stage>('waiting')
  const [room, setRoom] = useState<RoomTokenResponse | null>(null)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const cancelledRef = useRef(false)

  function clearPolling() {
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
      intervalRef.current = null
    }
  }

  async function tryJoin(): Promise<boolean> {
    try {
      const { status } = await callsApi.getRoomStatus(id)
      if (cancelledRef.current) return true
      if (status === 'ended') {
        clearPolling()
        setStage('ended')
        return true
      }
      if (status !== 'active') return false
      const data = await studentApi.roomToken(id)
      if (cancelledRef.current) return true
      clearPolling()
      setRoom(data)
      setStage('in-room')
      return true
    } catch (err) {
      if (cancelledRef.current) return true
      // 403 — выписали из курса; дальше поллить бессмысленно
      if ((err as ApiError).status === 403) {
        clearPolling()
        setStage('forbidden')
        return true
      }
      return false
    }
  }

  useEffect(() => {
    cancelledRef.current = false
    tryJoin().then((done) => {
      if (!done && !cancelledRef.current) intervalRef.current = setInterval(tryJoin, 5000)
    })
    return () => {
      cancelledRef.current = true
      clearPolling()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  async function handleDisconnected() {
    setRoom(null)
    try {
      const { status } = await callsApi.getRoomStatus(id)
      if (cancelledRef.current) return
      if (status === 'ended') {
        setStage('ended')
        return
      }
    } catch {}
    if (cancelledRef.current) return
    setStage('waiting')
    intervalRef.current = setInterval(tryJoin, 5000)
  }

  if (stage === 'in-room' && room) {
    return (
      <div style={{ height: '100dvh' }}>
        <CallRoom
          serverUrl={room.server_url}
          token={room.token}
          role="guest"
          enableMedia
          onDisconnected={handleDisconnected}
        />
      </div>
    )
  }

  const message =
    stage === 'ended'
      ? { title: 'Урок завершён', text: 'Спасибо за занятие!' }
      : stage === 'forbidden'
        ? { title: 'Нет доступа к уроку', text: 'Обратитесь к репетитору' }
        : { title: 'Урок ещё не начался', text: 'Ожидаем, когда репетитор начнёт урок...' }

  return (
    <div className="min-h-[100dvh] flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="flex items-center gap-2 justify-center">
          <GraduationCap className="h-6 w-6 text-primary" />
          <span className="font-semibold text-lg">TutorHub</span>
        </div>
        <div className="rounded-lg border p-6 text-center space-y-3">
          <p className="font-semibold">{message.title}</p>
          <p className="text-sm text-muted-foreground">{message.text}</p>
          <Link href="/student/lessons" className={buttonVariants({ variant: 'outline', size: 'sm' })}>
            К моим урокам
          </Link>
        </div>
      </div>
    </div>
  )
}
