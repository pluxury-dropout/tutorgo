'use client'

import { useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { ArrowLeft, Pencil, Trash2, UserPlus, X, Plus, ClipboardList } from 'lucide-react'

import {
  useCourse,
  useCourseBalance,
  useCourseEnrollments,
  useUpdateCourse,
  useDeleteCourse,
  useAddEnrollment,
  useRemoveEnrollment,
} from '@/lib/hooks/useCourses'
import {
  useLessons,
  useCreateLesson,
  useUpdateLesson,
  useDeleteLesson,
} from '@/lib/hooks/useLessons'
import { usePayments, useCreatePayment } from '@/lib/hooks/usePayments'
import { useStudents } from '@/lib/hooks/useStudents'
import { CourseForm } from '@/components/courses/CourseForm'
import { LessonForm, RecurrenceOptions } from '@/components/lessons/LessonForm'
import { AttendanceDialog } from '@/components/lessons/AttendanceDialog'
import { PaymentForm } from '@/components/payments/PaymentForm'
import { PageHeader } from '@/components/common/PageHeader'
import { CourseFormValues } from '@/schemas/course'
import { LessonFormValues } from '@/schemas/lesson'
import { PaymentFormValues } from '@/schemas/payment'
import { Lesson } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

function generateDates(baseISO: string, opts: RecurrenceOptions, courseEndAt?: string | null): string[] {
  const base    = new Date(baseISO)
  const limit   = opts.count ?? 200                          // hard cap
  const endDate = opts.count === undefined && courseEndAt
    ? new Date(courseEndAt)
    : null

  function within(d: Date) {
    if (endDate && d > endDate) return false
    return true
  }

  const results: Date[] = []

  if (opts.type === 'weekly_same') {
    for (let i = 0; i < limit; i++) {
      const d = new Date(base)
      d.setDate(base.getDate() + 7 * i)
      if (!within(d)) break
      results.push(d)
    }
  } else if (opts.type === 'every_n_weeks') {
    const n = opts.n ?? 2
    for (let i = 0; i < limit; i++) {
      const d = new Date(base)
      d.setDate(base.getDate() + 7 * n * i)
      if (!within(d)) break
      results.push(d)
    }
  } else if (opts.type === 'weekly_custom') {
    const days = (opts.days ?? []).slice().sort((a, b) => a - b)
    if (days.length === 0) return [base.toISOString()]
    const jsDay  = base.getDay()
    const toMon  = jsDay === 0 ? -6 : 1 - jsDay
    const monday = new Date(base)
    monday.setDate(base.getDate() + toMon)
    monday.setHours(base.getHours(), base.getMinutes(), 0, 0)

    for (let week = 0; results.length < limit; week++) {
      for (const isoDay of days) {
        if (results.length >= limit) break
        const d = new Date(monday)
        d.setDate(monday.getDate() + 7 * week + (isoDay - 1))
        if (!within(d)) { week = 9999; break }
        if (d >= base) results.push(d)
      }
      if (week > 200) break
    }
  }

  return results.map((d) => d.toISOString())
}

const STATUS_LABELS: Record<string, string> = {
  scheduled: 'Запланирован',
  completed: 'Проведён',
  cancelled: 'Отменён',
  missed:    'Пропущен',
}

const STATUS_COLORS: Record<string, string> = {
  scheduled: 'bg-blue-100 text-blue-700',
  completed: 'bg-green-100 text-green-700',
  cancelled: 'bg-gray-100 text-gray-500',
  missed:    'bg-red-100 text-red-700',
}

export default function CourseDetailPage() {
  const { id } = useParams<{ id: string }>()
  const router  = useRouter()

  const { data: course, isLoading } = useCourse(id)
  const { data: balance }           = useCourseBalance(id)
  const { data: enrollments = [] }  = useCourseEnrollments(id)
  const { data: students = [] }     = useStudents()
  const { data: lessons = [] }      = useLessons(id)
  const { data: payments = [] }     = usePayments(id)

  const [courseFormOpen, setCourseFormOpen]   = useState(false)
  const [lessonFormOpen, setLessonFormOpen]   = useState(false)
  const [editingLesson, setEditingLesson]     = useState<Lesson | undefined>()
  const [attendanceLesson, setAttendanceLesson] = useState<string | null>(null)
  const [paymentFormOpen, setPaymentFormOpen]   = useState(false)
  const [selectedStudent, setSelected]        = useState('')

  const updateCourse      = useUpdateCourse(id)
  const deleteCourse      = useDeleteCourse()
  const addEnrollment     = useAddEnrollment(id)
  const removeEnrollment  = useRemoveEnrollment(id)
  const createLesson      = useCreateLesson(id)
  const updateLesson      = useUpdateLesson(editingLesson?.id ?? '', id)
  const deleteLesson      = useDeleteLesson(id)
  const createPayment     = useCreatePayment(id)

  async function handleUpdateCourse(values: CourseFormValues) {
    const { type: _type, student_id: _sid, started_at, ended_at, ...rest } = values
    await updateCourse.mutateAsync({
      ...rest,
      started_at: `${started_at}T00:00:00Z`,
      ended_at:   ended_at ? `${ended_at}T00:00:00Z` : undefined,
    })
    toast.success('Курс обновлён')
  }

  async function handleDeleteCourse() {
    if (!course || !confirm(`Удалить курс "${course.subject}"?`)) return
    try {
      await deleteCourse.mutateAsync(course.id)
      router.push('/courses')
    } catch {
      toast.error('Нельзя удалить курс с уроками')
    }
  }

  async function handleAddEnrollment() {
    if (!selectedStudent) return
    try {
      await addEnrollment.mutateAsync(selectedStudent)
      setSelected('')
      toast.success('Ученик добавлен')
    } catch {
      toast.error('Ошибка добавления')
    }
  }

  async function handleRemoveEnrollment(studentId: string) {
    await removeEnrollment.mutateAsync(studentId)
    toast.success('Ученик удалён')
  }

  async function handleLessonSubmit(values: LessonFormValues, recurrence?: RecurrenceOptions) {
    const baseISO = new Date(values.scheduled_at).toISOString()
    if (editingLesson) {
      await updateLesson.mutateAsync({ ...values, scheduled_at: baseISO })
      toast.success('Урок обновлён')
    } else if (recurrence) {
      const dates = generateDates(baseISO, recurrence, course?.ended_at)
      await Promise.all(
        dates.map((scheduled_at) =>
          createLesson.mutateAsync({ ...values, scheduled_at, course_id: id })
        )
      )
      toast.success(`Создано ${dates.length} уроков`)
    } else {
      await createLesson.mutateAsync({ ...values, scheduled_at: baseISO, course_id: id })
      toast.success('Урок добавлен')
    }
  }

  async function handleDeleteLesson(lesson: Lesson) {
    if (!confirm('Удалить этот урок?')) return
    await deleteLesson.mutateAsync(lesson.id)
    toast.success('Урок удалён')
  }

  async function handlePaymentSubmit(values: PaymentFormValues) {
    await createPayment.mutateAsync({
      course_id:     id,
      amount:        values.amount,
      lessons_count: values.lessons_count,
      paid_at:       `${values.paid_at}T00:00:00Z`,
    })
    toast.success('Оплата добавлена')
  }

  function openCreateLesson() {
    setEditingLesson(undefined)
    setLessonFormOpen(true)
  }

  function openEditLesson(lesson: Lesson) {
    setEditingLesson(lesson)
    setLessonFormOpen(true)
  }

  if (isLoading) {
    return <div className="space-y-3">{[...Array(4)].map((_, i) => (
      <div key={i} className="h-10 rounded-md bg-muted animate-pulse" />
    ))}</div>
  }

  if (!course) return <p className="text-muted-foreground">Курс не найден</p>

  const isGroup = !course.student_id
  const enrolledIds = new Set(enrollments.map((e) => e.student_id))
  const availableStudents = students.filter((s) => !enrolledIds.has(s.id))

  function Row({ label, value }: { label: string; value: React.ReactNode }) {
    return (
      <div className="flex justify-between py-2 border-b last:border-0 text-sm">
        <span className="text-muted-foreground">{label}</span>
        <span className="font-medium">{value}</span>
      </div>
    )
  }

  return (
    <>
      <div className="mb-4">
        <Button variant="ghost" size="sm" onClick={() => router.push('/courses')}>
          <ArrowLeft className="h-4 w-4 mr-1.5" /> Курсы
        </Button>
      </div>

      <PageHeader
        title={course.subject}
        description={isGroup ? 'Групповой курс' : 'Индивидуальный курс'}
        actions={
          <div className="flex gap-2">
            <Button size="sm" variant="outline" onClick={() => setCourseFormOpen(true)}>
              <Pencil className="h-4 w-4 mr-1.5" /> Редактировать
            </Button>
            <Button size="sm" variant="destructive" onClick={handleDeleteCourse}>
              <Trash2 className="h-4 w-4 mr-1.5" /> Удалить
            </Button>
          </div>
        }
      />

      <div className="grid gap-4 md:grid-cols-2 mt-4">
        {/* Info */}
        <div className="border rounded-lg p-4">
          <h2 className="text-sm font-semibold mb-2">Информация</h2>
          <Row label="Предмет" value={course.subject} />
          <Row label="Тип" value={
            <Badge variant={isGroup ? 'secondary' : 'default'}>
              {isGroup ? 'Групповой' : 'Индивидуальный'}
            </Badge>
          } />
          <Row label="Цена за урок" value={`${course.price_per_lesson.toLocaleString()} ₸`} />
          <Row label="Начало" value={new Date(course.started_at).toLocaleDateString('ru-RU')} />
          {course.ended_at && (
            <Row label="Окончание" value={new Date(course.ended_at).toLocaleDateString('ru-RU')} />
          )}
        </div>

        {/* Balance */}
        <div className="border rounded-lg p-4">
          <h2 className="text-sm font-semibold mb-3">Баланс уроков</h2>
          {balance ? (
            <div className="grid grid-cols-3 gap-3 text-center">
              <div>
                <p className="text-2xl font-bold">{balance.lessons_paid}</p>
                <p className="text-xs text-muted-foreground mt-1">Оплачено</p>
              </div>
              <div>
                <p className="text-2xl font-bold">{balance.lessons_completed}</p>
                <p className="text-xs text-muted-foreground mt-1">Проведено</p>
              </div>
              <div>
                <p className="text-2xl font-bold text-primary">{balance.lessons_remaining}</p>
                <p className="text-xs text-muted-foreground mt-1">Осталось</p>
              </div>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Загрузка...</p>
          )}
        </div>
      </div>

      {/* Payments */}
      <div className="border rounded-lg p-4 mt-4">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-sm font-semibold">Оплаты ({payments.length})</h2>
          <Button size="sm" variant="outline" onClick={() => setPaymentFormOpen(true)}>
            <Plus className="h-4 w-4 mr-1.5" /> Добавить оплату
          </Button>
        </div>
        {payments.length === 0 ? (
          <p className="text-sm text-muted-foreground">Нет оплат</p>
        ) : (
          <div className="space-y-1">
            {payments.map((p) => (
              <div key={p.id} className="flex items-center justify-between py-2 border-b last:border-0 text-sm">
                <span className="text-muted-foreground">
                  {new Date(p.paid_at).toLocaleDateString('ru-RU')}
                </span>
                <span className="font-medium">{p.amount.toLocaleString()} ₸</span>
                <span className="text-muted-foreground">{p.lessons_count} ур.</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Enrollments — group only */}
      {isGroup && (
        <div className="border rounded-lg p-4 mt-4">
          <h2 className="text-sm font-semibold mb-3">Ученики группы</h2>
          {availableStudents.length > 0 && (
            <div className="flex gap-2 mb-4">
              <select
                value={selectedStudent}
                onChange={(e) => setSelected(e.target.value)}
                className="flex h-9 flex-1 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                <option value="">Выберите ученика...</option>
                {availableStudents.map((s) => (
                  <option key={s.id} value={s.id}>{s.first_name} {s.last_name}</option>
                ))}
              </select>
              <Button size="sm" onClick={handleAddEnrollment} disabled={!selectedStudent}>
                <UserPlus className="h-4 w-4 mr-1.5" /> Добавить
              </Button>
            </div>
          )}
          {enrollments.length === 0 ? (
            <p className="text-sm text-muted-foreground">Нет записанных учеников</p>
          ) : (
            <ul className="space-y-1">
              {enrollments.map((e) => (
                <li key={e.student_id} className="flex items-center justify-between py-1.5 border-b last:border-0 text-sm">
                  <span>{e.student_first_name} {e.student_last_name}</span>
                  <Button size="icon" variant="ghost" className="h-7 w-7 text-destructive hover:text-destructive"
                    onClick={() => handleRemoveEnrollment(e.student_id)}>
                    <X className="h-3.5 w-3.5" />
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {/* Lessons */}
      <div className="border rounded-lg p-4 mt-4">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-sm font-semibold">Уроки ({lessons.length})</h2>
          <Button size="sm" variant="outline" onClick={openCreateLesson}>
            <Plus className="h-4 w-4 mr-1.5" /> Добавить урок
          </Button>
        </div>

        {lessons.length === 0 ? (
          <p className="text-sm text-muted-foreground">Нет уроков</p>
        ) : (
          <div className="space-y-1">
            {lessons.map((lesson) => (
              <div key={lesson.id}
                className="flex items-center justify-between py-2 border-b last:border-0 text-sm"
              >
                <div className="flex items-center gap-3 min-w-0">
                  <span className="text-muted-foreground shrink-0">
                    {new Date(lesson.scheduled_at).toLocaleString('ru-RU', {
                      day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit',
                    })}
                  </span>
                  <span className="text-muted-foreground shrink-0">{lesson.duration_minutes} мин</span>
                  <span className={`text-xs px-2 py-0.5 rounded-full shrink-0 ${STATUS_COLORS[lesson.status] ?? ''}`}>
                    {STATUS_LABELS[lesson.status] ?? lesson.status}
                  </span>
                  {lesson.notes && (
                    <span className="text-muted-foreground truncate">{lesson.notes}</span>
                  )}
                </div>
                <div className="flex items-center gap-1 shrink-0 ml-2">
                  {isGroup && (
                    <Button size="icon" variant="ghost" className="h-8 w-8"
                      onClick={() => setAttendanceLesson(lesson.id)}
                      title="Посещаемость"
                    >
                      <ClipboardList className="h-3.5 w-3.5" />
                    </Button>
                  )}
                  <Button size="icon" variant="ghost" className="h-8 w-8"
                    onClick={() => openEditLesson(lesson)}>
                    <Pencil className="h-3.5 w-3.5" />
                  </Button>
                  <Button size="icon" variant="ghost" className="h-8 w-8 text-destructive hover:text-destructive"
                    onClick={() => handleDeleteLesson(lesson)}>
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <CourseForm
        open={courseFormOpen}
        onClose={() => setCourseFormOpen(false)}
        onSubmit={handleUpdateCourse}
        initial={course}
      />

      <LessonForm
        open={lessonFormOpen}
        onClose={() => setLessonFormOpen(false)}
        onSubmit={handleLessonSubmit}
        initial={editingLesson}
        courseEndAt={course.ended_at ?? undefined}
      />

      <PaymentForm
        open={paymentFormOpen}
        onClose={() => setPaymentFormOpen(false)}
        onSubmit={handlePaymentSubmit}
        pricePerLesson={course.price_per_lesson}
      />

      {attendanceLesson && (
        <AttendanceDialog
          lessonId={attendanceLesson}
          courseId={id}
          open={!!attendanceLesson}
          onClose={() => setAttendanceLesson(null)}
        />
      )}
    </>
  )
}
