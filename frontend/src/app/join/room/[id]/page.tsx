'use client'

import { useEffect, useRef, useState } from 'react'
import { useParams } from 'next/navigation'
import '@livekit/components-styles'

import { callsApi, type RoomTokenResponse } from '@/lib/api/calls'
import { CallRoom } from '@/components/call/CallRoom'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { GraduationCap } from 'lucide-react'

type Stage = 'form' | 'waiting' | 'in-room' | 'ended'

export default function JoinRoomPage() {
  const { id }  = useParams<{ id: string }>()

  const [stage, setStage]     = useState<Stage>('form')
  const [name, setName]       = useState('')
  const [room, setRoom]       = useState<RoomTokenResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  function clearPolling() {
    if (intervalRef.current) { clearInterval(intervalRef.current); intervalRef.current = null }
  }
  useEffect(() => () => clearPolling(), [])

  async function tryJoin(): Promise<boolean> {
    try {
      const { status } = await callsApi.getQuickRoomStatus(id)
      if (status === 'ended') { clearPolling(); setLoading(false); setStage('ended'); return true }
      if (status !== 'active') return false
      const data = await callsApi.getQuickGuestToken(id, name.trim())
      clearPolling(); setLoading(false); setRoom(data); setStage('in-room')
      return true
    } catch { return false }
  }

  async function handleJoin() {
    if (!name.trim()) return
    setLoading(true)
    const joined = await tryJoin()
    if (!joined) { setLoading(false); setStage('waiting'); intervalRef.current = setInterval(tryJoin, 5000) }
  }

  async function handleDisconnected() {
    try {
      const { status } = await callsApi.getQuickRoomStatus(id)
      if (status === 'ended') { setRoom(null); setStage('ended'); return }
    } catch {}
    setRoom(null); setStage('form')
  }

  if (stage === 'in-room' && room) {
    return (
      <div style={{ height: '100dvh' }}>
        <CallRoom
          trial
          guestName={name.trim()}
          serverUrl={room.server_url}
          token={room.token}
          role="guest"
          enableMedia
          onDisconnected={handleDisconnected}
        />
      </div>
    )
  }

  if (stage === 'ended') {
    return (
      <div className="min-h-[100dvh] flex items-center justify-center bg-background p-4">
        <div className="w-full max-w-sm space-y-6">
          <div className="flex items-center gap-2 justify-center">
            <GraduationCap className="h-6 w-6 text-primary" />
            <span className="font-semibold text-lg">Amida</span>
          </div>
          <div className="rounded-lg border p-6 text-center space-y-2">
            <p className="font-semibold">Урок завершён</p>
            <p className="text-sm text-muted-foreground">Спасибо за занятие!</p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-[100dvh] flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="flex items-center gap-2 justify-center">
          <GraduationCap className="h-6 w-6 text-primary" />
          <span className="font-semibold text-lg">Amida</span>
        </div>
        <div className="rounded-lg border p-6 space-y-4">
          <h1 className="text-base font-semibold">Присоединиться к уроку</h1>
          {stage === 'waiting' ? (
            <p className="text-sm text-muted-foreground text-center py-2">
              Урок ещё не начался. Ожидаем начала...
            </p>
          ) : (
            <>
              <div className="space-y-2">
                <label className="text-sm text-muted-foreground">Ваше имя</label>
                <Input placeholder="Введите ваше имя" value={name}
                  onChange={(e) => setName(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleJoin()} autoFocus />
              </div>
              <Button className="w-full" onClick={handleJoin} disabled={!name.trim() || loading}>
                {loading ? 'Подключение...' : 'Войти в урок'}
              </Button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
