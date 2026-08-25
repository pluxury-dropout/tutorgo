'use client'

import { useMemo, useState } from 'react'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'

import { useAttendance, useUpdateAttendance } from '@/lib/hooks/useLessons'
import { useCourseEnrollments } from '@/lib/hooks/useCourses'
import { useUpdateLessonStatus } from '@/lib/hooks/useCalendar'
import { STATUS_LABELS } from '@/lib/lessonStatus'

import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTitle } from '@/components/ui/popover'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { LessonStatus } from '@/types/api'
import { Video, Link2 } from 'lucide-react'

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
  /** Блок занятия в сетке — поповер встаёт рядом с ним. */
  anchor:  Element | null
  onClose: () => void
}

type Attendance = 'present' | 'absent'

export function LessonQuickPopover({ lesson, anchor, onClose }: Props) {
  return (
    <Popover open={!!lesson} onOpenChange={(open) => !open && onClose()}>
      <PopoverContent anchor={anchor}>
        {/* key: смена урока пересоздаёт форму, поэтому поля инициализируются
            из пропа напрямую — без синхронизирующих эффектов. */}
        {lesson && <QuickLessonForm key={lesson.id} lesson={lesson} onClose={onClose} />}
      </PopoverContent>
    </Popover>
  )
}

function QuickLessonForm({ lesson, onClose }: { lesson: QuickLesson; onClose: () => void }) {
  const router = useRouter()

  const [status, setStatus] = useState<LessonStatus>(lesson.status)
  const [notes, setNotes]   = useState(lesson.notes ?? '')
  // Только правки пользователя. Сохранённые отметки приходят асинхронно и живут
  // в кеше запроса — копировать их в state значило бы держать две версии правды.
  const [overrides, setOverrides] = useState<Map<string, Attendance>>(new Map())

  const updateStatus     = useUpdateLessonStatus(lesson.id)
  const { data: enrollments = [] } = useCourseEnrollments(lesson.isGroup ? lesson.courseId : '')
  const { data: existing = [] }    = useAttendance(lesson.isGroup ? lesson.id : '')
  const updateAttendance = useUpdateAttendance(lesson.id)

  const saved = useMemo(
    () => new Map(existing.map((a) => [a.student_id, a.status as Attendance])),
    [existing],
  )

  // Незаполненная посещаемость трактуется как «присутствует» — как и было.
  const attendanceOf = (studentId: string): Attendance =>
    overrides.get(studentId) ?? saved.get(studentId) ?? 'present'

  function toggle(studentId: string) {
    const next: Attendance = attendanceOf(studentId) === 'present' ? 'absent' : 'present'
    setOverrides((prev) => new Map(prev).set(studentId, next))
  }

  async function handleSave() {
    try {
      await updateStatus.mutateAsync({
        scheduled_at:     lesson.scheduledAt,
        duration_minutes: lesson.durationMinutes,
        status,
        notes,
      })
      if (lesson.isGroup && enrollments.length > 0) {
        await updateAttendance.mutateAsync(
          enrollments.map((e) => ({ student_id: e.student_id, status: attendanceOf(e.student_id) })),
        )
      }
      toast.success('Сохранено')
      onClose()
    } catch {
      toast.error('Ошибка сохранения')
    }
  }

  const start   = new Date(lesson.scheduledAt)
  const end     = new Date(start.getTime() + lesson.durationMinutes * 60_000)
  const fmt     = (d: Date) => d.toLocaleTimeString('ru', { hour: '2-digit', minute: '2-digit' })
  const fmtDate = start.toLocaleDateString('ru', { weekday: 'long', day: 'numeric', month: 'long' })

  const isPending = updateStatus.isPending || updateAttendance.isPending

  return (
    <>
      <div className="flex flex-col gap-2 pr-8">
        <PopoverTitle className="text-base">{lesson.title}</PopoverTitle>
        <p className="text-sm text-muted-foreground capitalize">
          {fmtDate}, {fmt(start)}–{fmt(end)}
        </p>
      </div>

      <div className="space-y-3 py-1">
        <div>
          <label className="text-xs font-medium text-muted-foreground mb-1 block">Статус</label>
          <Select value={status} onValueChange={(v) => setStatus(v as LessonStatus)}>
            <SelectTrigger className="w-full">
              <SelectValue>{STATUS_LABELS[status]}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              {(Object.keys(STATUS_LABELS) as LessonStatus[]).map((s) => (
                <SelectItem key={s} value={s}>{STATUS_LABELS[s]}</SelectItem>
              ))}
            </SelectContent>
          </Select>
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

        {lesson.isGroup && enrollments.length > 0 && (
          <div>
            <label className="text-xs font-medium text-muted-foreground mb-2 block">Посещаемость</label>
            <div className="space-y-1">
              {enrollments.map((e) => {
                const st = attendanceOf(e.student_id)
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

      <div className="flex gap-2 pt-1">
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="flex-1 gap-1.5"
          onClick={() => { window.open(`/lessons/${lesson.id}/call`, '_blank'); onClose() }}
        >
          <Video size={14} />
          Начать звонок
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="flex-1 gap-1.5"
          onClick={() => {
            const link = `${window.location.origin}/student/lessons/${lesson.id}/call`
            navigator.clipboard.writeText(link)
            toast.success('Ссылка скопирована')
          }}
        >
          <Link2 size={14} />
          Ссылка ученику
        </Button>
      </div>

      <div className="flex items-center justify-between pt-2">
        <button
          type="button"
          onClick={() => { router.push(`/courses/${lesson.courseId}`); onClose() }}
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
    </>
  )
}
