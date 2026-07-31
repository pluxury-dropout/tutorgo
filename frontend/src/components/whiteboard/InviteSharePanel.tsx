'use client'

import { useState } from 'react'
import { Check } from 'lucide-react'
import { toast } from 'sonner'
import { useCreateInvite } from '@/lib/hooks/useWhiteboard'
import { useBoardContext } from './BoardContext'

export function InviteSharePanel() {
  const { boardId, isGuest } = useBoardContext()
  const [copying, setCopying] = useState(false)
  const createInvite = useCreateInvite(boardId)

  if (isGuest) return null

  const handleCopyInvite = async () => {
    try {
      const inv = await createInvite.mutateAsync()
      const url = `${window.location.origin}/board/join/${inv.id}`
      await navigator.clipboard.writeText(url)
      setCopying(true)
      setTimeout(() => setCopying(false), 2000)
    } catch {
      toast.error('Не удалось создать ссылку-приглашение')
    }
  }

  return (
    <div className="tlui-share-zone" draggable={false}>
      <button
        onClick={handleCopyInvite}
        className="tlui-button tlui-button__normal"
        style={{ minWidth: 140 }}
      >
        {copying ? (
          <span className="inline-flex items-center gap-1.5">
            <Check className="size-4" />
            Скопировано!
          </span>
        ) : (
          'Пригласить ученика'
        )}
      </button>
    </div>
  )
}
