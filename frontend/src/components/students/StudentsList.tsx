'use client'

import { useRef, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Pencil, Trash2, ChevronRight, UserPlus, Copy } from 'lucide-react'
import { toast } from 'sonner'

import { SectionCard } from '@/components/common/SectionCard'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { studentsApi } from '@/lib/api/students'
import { Student } from '@/types/api'

interface StudentsListProps {
  students: Student[]
  onEdit: (s: Student) => void
  onDelete: (s: Student) => void
}

export function StudentsList({ students, onEdit, onDelete }: StudentsListProps) {
  const router = useRouter()
  const [inviteFor, setInviteFor] = useState<Student | null>(null)
  const [invite, setInvite] = useState<{ invite_token: string; expires_at: string } | null>(null)

  // ref, а не state: setState асинхронный, двойной клик в одном тике обошёл бы
  // проверку, а повторный POST ротирует токен и убивает первую ссылку
  const invitePending = useRef(false)

  async function handleInvite(s: Student) {
    if (invitePending.current) return
    invitePending.current = true
    setInviteFor(s)
    setInvite(null)
    try {
      setInvite(await studentsApi.invite(s.id))
    } catch {
      toast.error('Не удалось создать приглашение')
      setInviteFor(null)
    } finally {
      invitePending.current = false
    }
  }

  const inviteUrl = invite
    ? `${window.location.origin}/student/invite/${invite.invite_token}`
    : ''

  function copyInvite() {
    try {
      navigator.clipboard
        .writeText(inviteUrl)
        .then(() => toast.success('Ссылка скопирована'))
        .catch(() => toast.error('Не удалось скопировать'))
    } catch {
      toast.error('Не удалось скопировать')
    }
  }

  return (
    <SectionCard>
      {students.map((student, i) => (
        <div
          key={student.id}
          style={{
            display: 'grid',
            gridTemplateColumns: 'minmax(0, 1fr) auto',
            alignItems: 'center',
            gap: 16,
            padding: '10px 18px',
            borderTop: i === 0 ? 'none' : '1px solid var(--row-border)',
            cursor: 'pointer',
          }}
          className="hover:bg-muted/30 group"
          role="button"
          tabIndex={0}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); router.push(`/students/${student.id}`) } }}
          onClick={() => router.push(`/students/${student.id}`)}
        >
          <div style={{ minWidth: 0 }}>
            <div style={{
              fontSize: 14, fontWeight: 600, color: 'var(--foreground)',
              overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
            }}>
              {student.first_name}{student.last_name ? ` ${student.last_name}` : ''}
            </div>
            {student.email && (
              <div style={{
                fontSize: 12.5, color: 'var(--muted-foreground)', marginTop: 1,
                overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
              }}>
                {student.email}
              </div>
            )}
          </div>
          <div className="flex items-center gap-3 shrink-0">
            <span style={{ fontSize: 13, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
              {student.phone || '—'}
            </span>
            <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity duration-150" />
            <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
              <Button size="icon" variant="ghost" className="h-8 w-8"
                title="Пригласить в кабинет ученика"
                onClick={() => handleInvite(student)}>
                <UserPlus className="h-3.5 w-3.5" />
              </Button>
              <Button size="icon" variant="ghost" className="h-8 w-8" onClick={() => onEdit(student)}>
                <Pencil className="h-3.5 w-3.5" />
              </Button>
              <Button size="icon" variant="ghost"
                className="h-8 w-8 text-destructive hover:text-destructive"
                onClick={() => onDelete(student)}>
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        </div>
      ))}

      <Dialog open={inviteFor !== null} onOpenChange={(open) => { if (!open) setInviteFor(null) }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Приглашение в кабинет</DialogTitle>
            <DialogDescription>
              {inviteFor
                ? `Отправьте ссылку ученику: ${inviteFor.first_name}${inviteFor.last_name ? ` ${inviteFor.last_name}` : ''}. По ней он создаст аккаунт и получит доступ к своим урокам.`
                : ''}
            </DialogDescription>
          </DialogHeader>
          {invite ? (
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <Input readOnly value={inviteUrl} onFocus={(e) => e.target.select()} />
                <Button size="icon" variant="outline" className="shrink-0" onClick={copyInvite} title="Скопировать">
                  <Copy className="h-4 w-4" />
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                Ссылка действует до {new Date(invite.expires_at).toLocaleDateString('ru-RU')}.
                Повторное приглашение заменит эту ссылку.
              </p>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Создание ссылки...</p>
          )}
        </DialogContent>
      </Dialog>
    </SectionCard>
  )
}
