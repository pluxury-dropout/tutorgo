'use client'

import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { useAttendance, useUpdateAttendance } from '@/lib/hooks/useLessons'
import { useCourseEnrollments } from '@/lib/hooks/useCourses'

import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

interface AttendanceDialogProps {
  lessonId: string
  courseId: string
  open:     boolean
  onClose:  () => void
}

export function AttendanceDialog({ lessonId, courseId, open, onClose }: AttendanceDialogProps) {
  const { data: enrollments = [] } = useCourseEnrollments(courseId)
  const { data: existing = [] }    = useAttendance(lessonId)
  const updateAttendance           = useUpdateAttendance(lessonId)

  const [attendance, setAttendance] = useState<Map<string, 'present' | 'absent'>>(new Map())

  // Шаг 1: при открытии диалога строим Map из enrolled студентов
  // Шаг 2: перезаписываем значения из уже сохранённых записей (existing)
  useEffect(() => {
    const map = new Map<string, 'present' | 'absent'>(enrollments.map((e) => [e.student_id, 'present']))
    existing.forEach((a) => map.set(a.student_id, a.status as 'present' | 'absent'))
    setAttendance(map)
  }, [enrollments, existing, open])

  // Шаг 3: toggle — создаём НОВЫЙ Map (копию), меняем одно значение
  function toggle(studentId: string) {
    setAttendance((prev) => {
      const next = new Map(prev)
      next.set(studentId, prev.get(studentId) === 'present' ? 'absent' : 'present')
      return next
    })
  }

  async function handleSave() {
    const payload = Array.from(attendance.entries()).map(([student_id, status]) => ({
      student_id,
      status,
    }))
    try {
      await updateAttendance.mutateAsync(payload)
      toast.success('Посещаемость сохранена')
      onClose()
    } catch {
      toast.error('Ошибка сохранения')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>Посещаемость</DialogTitle>
        </DialogHeader>

        <div className="space-y-2 py-2">
          {enrollments.length === 0 ? (
            <p className="text-sm text-muted-foreground">Нет записанных учеников</p>
          ) : (
            enrollments.map((e) => {
              const status = attendance.get(e.student_id) ?? 'present'
              return (
                <div
                  key={e.student_id}
                  className="flex items-center justify-between py-1.5 border-b last:border-0"
                >
                  <span className="text-sm">
                    {e.student_first_name}{e.student_last_name ? ` ${e.student_last_name}` : ''}
                  </span>
                  <button
                    type="button"
                    onClick={() => toggle(e.student_id)}
                    className={`text-xs font-medium px-2.5 py-1 rounded-full transition-colors ${
                      status === 'present'
                        ? 'bg-green-100 text-green-700 hover:bg-green-200'
                        : 'bg-red-100 text-red-700 hover:bg-red-200'
                    }`}
                  >
                    {status === 'present' ? 'Присутствует' : 'Отсутствует'}
                  </button>
                </div>
              )
            })
          )}
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="outline" onClick={onClose}>Отмена</Button>
          <Button onClick={handleSave} disabled={updateAttendance.isPending}>
            {updateAttendance.isPending ? 'Сохранение...' : 'Сохранить'}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
