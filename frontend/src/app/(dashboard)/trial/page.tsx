'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { Video } from 'lucide-react'

import { callsApi } from '@/lib/api/calls'
import { PageHeader } from '@/components/common/PageHeader'
import { SectionCard } from '@/components/common/SectionCard'
import { Button } from '@/components/ui/button'

export default function TrialPage() {
  const router = useRouter()
  const [starting, setStarting] = useState(false)

  async function handleStart() {
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

  return (
    <div style={{ maxWidth: 900 }}>
      <PageHeader title="Пробный урок" />

      <SectionCard bodyPadding={18}>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start', gap: 14 }}>
          <p style={{ margin: 0, fontSize: 13, lineHeight: 1.6, color: 'var(--muted-foreground)', maxWidth: '60ch' }}>
            Пробный урок не привязан к курсу и не попадает в расписание — подойдёт для знакомства
            с учеником, которого ещё нет в базе. Ссылку для ученика можно скопировать внутри комнаты.
            Доска у всех пробных уроков общая: то, что вы на ней нарисовали, останется к следующему разу.
          </p>

          <Button onClick={handleStart} disabled={starting}>
            <Video className="h-4 w-4 mr-1.5" />
            {starting ? 'Создаём комнату...' : 'Начать пробный урок'}
          </Button>
        </div>
      </SectionCard>
    </div>
  )
}
