'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'

import { callsApi } from '@/lib/api/calls'

/** Создать комнату пробного урока и уйти в неё.
 *
 *  Живёт хуком, а не на странице `/trial`: с фазы 1 пробный ставится в
 *  расписание событием `kind = 'trial'`, и та же кнопка нужна в поповере
 *  события. Токен кладётся в sessionStorage — комната достаёт его оттуда,
 *  чтобы не светить в URL. */
export function useStartQuickRoom() {
  const router = useRouter()
  const [starting, setStarting] = useState(false)

  async function start() {
    if (starting) return
    setStarting(true)
    try {
      const { room_id, token, server_url } = await callsApi.startQuickRoom()
      sessionStorage.setItem(`quick-room-${room_id}`, JSON.stringify({ token, server_url }))
      router.push(`/room/${room_id}`)
    } catch {
      toast.error('Не удалось создать комнату')
      setStarting(false)
    }
  }

  return { start, starting }
}
