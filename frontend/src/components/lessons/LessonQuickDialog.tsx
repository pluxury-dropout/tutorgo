'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'

import { useAttendance, useUpdateAttendance } from '@/lib/hooks/useLessons'
import { useCourseEnrollments } from '@/lib/hooks/useCourses'
import { useUpdateLessonStatus } from '@/lib/hooks/useCalendar'
import { STATUS_LABELS } from '@/lib/lessonStatus'

import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import type { LessonStatus } from '@/types/api'

export interface QuickLesson {
  id:              string
  courseId:        string
  title:           string
  status:          LessonStatus
  notes:           string
  isGroup:         boolean
  scheduledAt:     string
  durationMinutes: number
}

interface Props {
  lesson:  QuickLesson | null
  onClose: () => void
}

export function LessonQuickDialog({ lesson, onClose }: Props) {
  const router = useRouter()

  const [status, setStatus]         = useState<LessonStatus>('scheduled')
  const [notes, setNotes]           = useState('')
  const [attendance, setAttendance] = useState<Map<string, 'present' | 'absent'>>(new Map())

  const updateStatus     = useUpdateLessonStatus(lesson?.id ?? '')
  const { data: enrollments = [] } = useCourseEnrollments(lesson?.isGroup ? (lesson.courseId) : '')
  const { data: existing = [] }    = useAttendance(lesson?.isGroup ? (lesson.id) : '')
  const updateAttendance = useUpdateAttendance(lesson?.id ?? '')

  useEffect(() => {
    if (!lesson) return
    setStatus(lesson.status)
    setNotes(lesson.notes ?? '')
  }, [lesson])

  useEffect(() => {
    if (!lesson?.isGroup) return
    const map = new Map<string, 'present' | 'absent'>(
      enrollments.map((e) => [e.student_id, 'present']),
    )
    existing.forEach((a) => map.set(a.student_id, a.status as 'present' | 'absent'))
    setAttendance(map)
  }, [enrollments, existing, lesson])

  function toggle(studentId: string) {
    setAttendance((prev) => {
      const next = new Map(prev)
      next.set(studentId, prev.get(studentId) === 'present' ? 'absent' : 'present')
      return next
    })
  }

  async function handleSave() {
    if (!lesson) return
    try {
      await updateStatus.mutateAsync({
        scheduled_at:     lesson.scheduledAt,
        duration_minutes: lesson.durationMinutes,
        status,
        notes,
      })
      if (lesson.isGroup && enrollments.length > 0) {
        const payload = Array.from(attendance.entries()).map(([student_id, st]) => ({
          student_id,
          status: st,
        }))
        await updateAttendance.mutateAsync(payload)
      }
      toast.success('Сохранено')
      onClose()
    } catch {
      toast.error('Ошибка сохранения')
    }
  }

  const start   = lesson ? new Date(lesson.scheduledAt) : null
  const end     = start ? new Date(start.getTime() + (lesson!.durationMinutes) * 60_000) : null
  const fmt     = (d: Date) => d.toLocaleTimeString('ru', { hour: '2-digit', minute: '2-digit' })
  const fmtDate = start?.toLocaleDateString('ru', { weekday: 'long', day: 'numeric', month: 'long' }) ?? ''

  const isPending = updateStatus.isPending || updateAttendance.isPending

  return (
    <Dialog open={!!lesson} onOpenChange={onClose}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle className="text-base">{lesson?.title}</DialogTitle>
          {start && end && (
            <p className="text-sm text-muted-foreground capitalize">
              {fmtDate}, {fmt(start)}–{fmt(end)}
            </p>
          )}
        </DialogHeader>

        <div className="space-y-3 py-1">
          <div>
            <label className="text-xs font-medium text-muted-foreground mb-1 block">Статус</label>
            <select
              value={status}
              onChange={(e) => setStatus(e.target.value as LessonStatus)}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            >
              {(Object.keys(STATUS_LABELS) as LessonStatus[]).map((s) => (
                <option key={s} value={s}>{STATUS_LABELS[s]}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="text-xs font-medium text-muted-foreground mb-1 block">Заметки</label>
            <textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              rows={2}
              placeholder="Добавить заметку..."
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm resize-none"
            />
          </div>

          {lesson?.isGroup && enrollments.length > 0 && (
            <div>
              <label className="text-xs font-medium text-muted-foreground mb-2 block">Посещаемость</label>
              <div className="space-y-1">
                {enrollments.map((e) => {
                  const st = attendance.get(e.student_id) ?? 'present'
                  return (
                    <div key={e.student_id} className="flex items-center justify-between py-1 border-b last:border-0">
                      <span className="text-sm">
                        {e.student_first_name}{e.student_last_name ? ` ${e.student_last_name}` : ''}
                      </span>
                      <button
                        type="button"
                        onClick={() => toggle(e.student_id)}
                        className={`text-xs font-medium px-2.5 py-1 rounded-full transition-colors ${
                          st === 'present'
                            ? 'bg-green-100 text-green-700 hover:bg-green-200'
                            : 'bg-red-100 text-red-700 hover:bg-red-200'
                        }`}
                      >
                        {st === 'present' ? 'Присутствует' : 'Отсутствует'}
                      </button>
                    </div>
                  )
                })}
              </div>
            </div>
          )}
        </div>

        <div className="flex items-center justify-between pt-2">
          <button
            type="button"
            onClick={() => { router.push(`/courses/${lesson?.courseId}`); onClose() }}
            className="text-sm text-muted-foreground underline-offset-4 hover:underline hover:text-foreground"
          >
            Перейти к курсу →
          </button>
          <div className="flex gap-2">
            <Button type="button" variant="outline" onClick={onClose}>Отмена</Button>
            <Button onClick={handleSave} disabled={isPending}>
              {isPending ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
