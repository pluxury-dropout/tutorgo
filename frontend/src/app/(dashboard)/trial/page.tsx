'use client'

import { Video } from 'lucide-react'

import { useStartQuickRoom } from '@/lib/hooks/useStartQuickRoom'
import { PageHeader } from '@/components/common/PageHeader'
import { SectionCard } from '@/components/common/SectionCard'
import { Button } from '@/components/ui/button'

export default function TrialPage() {
  const { start: handleStart, starting } = useStartQuickRoom()

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
