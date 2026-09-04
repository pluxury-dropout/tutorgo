'use client'

import { useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { toast } from 'sonner'
import Link from 'next/link'
import { ArrowLeft, Pencil, Trash2, UserPlus, X, Plus, ClipboardList, ListX, Wallet, Users, CalendarDays } from 'lucide-react'

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
  useLessonsByPeriod,
  useCreateLesson,
  useUpdateLesson,
  useDeleteLesson,
  useDeleteLessonsByCourse,
} from '@/lib/hooks/useLessons'
import { usePayments, useCreatePayment, useUpdatePayment, useDeletePayment } from '@/lib/hooks/usePayments'
import { useStudents } from '@/lib/hooks/useStudents'
import { CourseForm } from '@/components/courses/CourseForm'
import { LessonForm } from '@/components/lessons/LessonForm'
import { AttendanceDialog } from '@/components/lessons/AttendanceDialog'
import { PaymentForm } from '@/components/payments/PaymentForm'
import { HomeworkEditPopover } from '@/components/homework/HomeworkEditPopover'
import { PageHeader } from '@/components/common/PageHeader'
import { EmptyState } from '@/components/common/EmptyState'
import { ErrorState } from '@/components/common/ErrorState'
import { CourseFormValues } from '@/schemas/course'
import { LessonFormValues } from '@/schemas/lesson'
import { LessonUpdateInput } from '@/lib/api/lessons'
import { RecurrenceScopeDialog } from '@/components/calendar/RecurrenceScopeDialog'
import { toRecurrenceInput, RecurrenceOptions } from '@/lib/recurrence'
import { PaymentFormValues } from '@/schemas/payment'
import { Lesson, Payment, RecurrenceScope } from '@/types/api'

import { Button } from '@/components/ui/button'
import { CycleBadge } from '@/components/lessons/CycleBadge'
import { CourseTypeBadge } from '@/components/common/CourseTypeBadge'
import { PeriodPicker } from '@/components/lessons/PeriodPicker'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { STATUS_LABELS, STATUS_COLORS } from '@/lib/lessonStatus'

// Возвращает { from, to } для текущей недели (Пн–Пн+7)
function currentWeekRange(): { from: Date; to: Date } {
  const now = new Date()
  const day = now.getDay()                   // 0=Вс, 1=Пн...
  const diff = day === 0 ? -6 : 1 - day
  const from = new Date(now)
  from.setDate(now.getDate() + diff)
  from.setHours(0, 0, 0, 0)
  const to = new Date(from)
  to.setDate(from.getDate() + 7)
  return { from, to }
}

export default function CourseDetailPage() {
  const { id } = useParams<{ id: string }>()
  const router  = useRouter()

  const { data: course, isLoading } = useCourse(id)
  const { data: balance }           = useCourseBalance(id)
  const {
    data: enrollments = [], isPending: enrollmentsPending,
    isError: enrollmentsError, refetch: refetchEnrollments,
  } = useCourseEnrollments(id)
  const { data: students = [] }     = useStudents()
  const [period, setPeriod] = useState(currentWeekRange)
  const {
    data: lessons = [], isLoading: lessonsLoading,
    isError: lessonsError, refetch: refetchLessons,
  } = useLessonsByPeriod(
    id,
    period.from.toISOString(),
    period.to.toISOString(),
  )
  const lessonsTotal = lessons.length
  const {
    data: payments = [], isPending: paymentsPending,
    isError: paymentsError, refetch: refetchPayments,
  } = usePayments(id)

  const [courseFormOpen, setCourseFormOpen]     = useState(false)
  const [lessonFormOpen, setLessonFormOpen]     = useState(false)
  const [editingLesson, setEditingLesson]       = useState<Lesson | undefined>()
  const [attendanceLesson, setAttendanceLesson] = useState<string | null>(null)
  const [paymentFormOpen, setPaymentFormOpen]   = useState(false)
  const [homeworkAnchor, setHomeworkAnchor]     = useState<Element | null>(null)
  const [editingPayment, setEditingPayment] = useState<Payment | null>(null)
  // Отложенное действие над вхождением серии: ждёт ответа об области.
  const [scopeAsk, setScopeAsk] = useState<
    { action: 'edit' | 'delete'; lesson: Lesson; values?: LessonUpdateInput } | null
  >(null)
  const [selectedStudent, setSelected]          = useState('')

  const updateCourse         = useUpdateCourse(id)
  const deleteCourse         = useDeleteCourse()
  const addEnrollment        = useAddEnrollment(id)
  const removeEnrollment     = useRemoveEnrollment(id)
  const createLesson         = useCreateLesson(id)
  const updateLesson         = useUpdateLesson(editingLesson?.id ?? '', id)
  const deleteLesson         = useDeleteLesson(id)
  const deleteLessonsByCourse = useDeleteLessonsByCourse(id)
  const createPayment        = useCreatePayment(id)
  const updatePayment = useUpdatePayment(id)
  const deletePayment = useDeletePayment(id)

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
    if (!course || !confirm(`Архивировать курс "${course.subject}"? Завершённые уроки останутся в календаре.`)) return
    try {
      await deleteCourse.mutateAsync(course.id)
      router.push('/courses')
    } catch {
      toast.error('Ошибка архивирования')
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
      // Правка вхождения серии сначала спрашивает область — сам запрос уйдёт
      // из onPick диалога.
      if (editingLesson.rule_id) {
        setScopeAsk({ action: 'edit', lesson: editingLesson, values: { ...values, scheduled_at: baseISO } })
        return
      }
      await updateLesson.mutateAsync({ data: { ...values, scheduled_at: baseISO } })
      toast.success('Урок обновлён')
    } else if (recurrence) {
      // Даты раскатывает сервер: правило хранится в БД, поэтому бессрочную
      // серию есть чем продлевать, а до горизонта её дотягивает ночная джоба.
      await createLesson.mutateAsync({
        course_id:        id,
        scheduled_at:     baseISO,
        duration_minutes: values.duration_minutes,
        notes:            values.notes,
        recurrence: {
          ...toRecurrenceInput(recurrence),
          ends_on: course?.ended_at ?? undefined,
        },
      })
      toast.success('Серия создана')
    } else {
      await createLesson.mutateAsync({ ...values, scheduled_at: baseISO, course_id: id })
      toast.success('Урок добавлен')
    }
  }

  async function handleDeleteLesson(lesson: Lesson) {
    if (lesson.rule_id) {
      setScopeAsk({ action: 'delete', lesson })
      return
    }
    if (!confirm('Удалить этот урок?')) return
    await deleteLesson.mutateAsync({ id: lesson.id })
    toast.success('Урок удалён')
  }

  // Выбранная область применяется к тому уроку, ради которого спросили.
  async function applyScope(scope: RecurrenceScope) {
    if (!scopeAsk) return
    const { action, lesson, values } = scopeAsk
    setScopeAsk(null)
    if (action === 'delete') {
      await deleteLesson.mutateAsync({ id: lesson.id, scope })
      toast.success(scope === 'one' ? 'Урок отменён' : 'Серия обновлена')
      return
    }
    if (values) {
      await updateLesson.mutateAsync({ data: values, scope })
      toast.success('Урок обновлён')
    }
  }

  async function handleDeleteAllLessons() {
    if (!confirm(`Удалить все ${lessonsTotal} уроков курса?`)) return
    await deleteLessonsByCourse.mutateAsync()
    toast.success('Все уроки удалены')
  }

  async function handlePaymentSubmit(values: PaymentFormValues) {
    if (editingPayment) {
      await updatePayment.mutateAsync({
        id:   editingPayment.id,
        data: {
          amount:        values.amount,
          lessons_count: values.lessons_count,
          paid_at:       values.paid_at,
        },
      })
      toast.success('Платёж обновлён')
    } else {
      await createPayment.mutateAsync({
        course_id:     id,
        amount:        values.amount,
        lessons_count: values.lessons_count,
        paid_at:       values.paid_at,
      })
    }
  }

  async function handlePaymentDelete(p: Payment) {
    if (!confirm(`Удалить платёж на ${p.amount.toLocaleString()} ₸?`)) return
    await deletePayment.mutateAsync(p.id)
    toast.success('Платёж удалён')
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
        meta={isGroup ? 'Групповой курс' : 'Индивидуальный курс'}
        actions={
          <div className="flex gap-2">
            <Link
              href={`/boards/${id}`}
              target="_blank"
              className="text-sm px-3 py-1.5 rounded border border-border hover:bg-muted text-foreground inline-flex items-center"
            >
              Доска
            </Link>
            <Button size="sm" variant="outline" onClick={(e) => setHomeworkAnchor(e.currentTarget)}>
              Домашнее задание
            </Button>
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
        <div className="border rounded-xl bg-card p-4">
          <h2 className="text-sm font-semibold mb-2">Информация</h2>
          <Row label="Предмет" value={course.subject} />
          <Row label="Тип" value={<CourseTypeBadge isGroup={isGroup} />} />
          <Row label="Цена за цикл" value={`${course.price_per_cycle.toLocaleString()} ₸`} />
          <Row label="Уроков в цикле" value={String(course.lessons_per_cycle)} />
          <Row label="Начало" value={new Date(course.started_at).toLocaleDateString('ru-RU')} />
          {course.ended_at && (
            <Row label="Окончание" value={new Date(course.ended_at).toLocaleDateString('ru-RU')} />
          )}
        </div>

        {/* Balance */}
        <div className="border rounded-xl bg-card p-4">
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
        {paymentsPending ? (
          <div className="space-y-2">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="h-8 rounded bg-muted animate-pulse" />
            ))}
          </div>
        ) : paymentsError ? (
          <ErrorState size="sm" what="оплаты" onRetry={() => refetchPayments()} />
        ) : payments.length === 0 ? (
          <EmptyState
            size="sm"
            icon={Wallet}
            title="Оплат по курсу нет"
            description="Отметь оплату — от неё считается баланс уроков и подсказка, когда просить следующую"
            action={{ label: 'Добавить оплату', onClick: () => setPaymentFormOpen(true) }}
          />
        ) : (
          <div className="space-y-1">
            {payments.map((p) => (
              <div
                key={p.id}
                className="flex items-center justify-between py-2 border-b last:border-0 text-sm group"
              >
                <span className="text-muted-foreground">
                  {new Date(p.paid_at).toLocaleDateString('ru-RU')}
                </span>
                <span className="font-medium">{p.amount.toLocaleString()} ₸</span>
                <span className="text-muted-foreground">{p.lessons_count} ур.</span>
                <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity duration-150">
                  <Button
                    size="icon"
                    variant="ghost"
                    className="h-7 w-7"
                    onClick={() => { setEditingPayment(p); setPaymentFormOpen(true) }}
                  >
                    <Pencil className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="h-7 w-7 text-destructive hover:text-destructive"
                    onClick={() => handlePaymentDelete(p)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
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
              <Select value={selectedStudent} onValueChange={(v) => setSelected(v ?? '')}>
                <SelectTrigger className="flex-1">
                  <SelectValue placeholder="Выберите ученика..." />
                </SelectTrigger>
                <SelectContent>
                  {availableStudents.map((s) => {
                    const name = s.last_name ? `${s.first_name} ${s.last_name}` : s.first_name
                    return (
                      <SelectItem key={s.id} value={s.id} label={name}>
                        {name}
                      </SelectItem>
                    )
                  })}
                </SelectContent>
              </Select>
              <Button size="sm" onClick={handleAddEnrollment} disabled={!selectedStudent}>
                <UserPlus className="h-4 w-4 mr-1.5" /> Добавить
              </Button>
            </div>
          )}
          {enrollmentsPending ? (
            <div className="space-y-2">
              {[...Array(2)].map((_, i) => (
                <div key={i} className="h-8 rounded bg-muted animate-pulse" />
              ))}
            </div>
          ) : enrollmentsError ? (
            <ErrorState size="sm" what="состав группы" onRetry={() => refetchEnrollments()} />
          ) : enrollments.length === 0 ? (
            <EmptyState
              size="sm"
              icon={Users}
              title="В группе пока никого"
              description="Добавь учеников из списка выше — тогда на каждом уроке можно будет отмечать посещаемость"
            />
          ) : (
            <ul className="space-y-1">
              {enrollments.map((e) => (
                <li key={e.student_id} className="flex items-center justify-between py-1.5 border-b last:border-0 text-sm">
                  <span>{e.student_first_name}{e.student_last_name ? ` ${e.student_last_name}` : ''}</span>
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
          <div className="flex items-center gap-3">
            <h2 className="text-sm font-semibold">Уроки ({lessonsTotal})</h2>
            <PeriodPicker
              from={period.from}
              to={period.to}
              onChange={(from, to) => setPeriod({ from, to })}
            />
          </div>
          <div className="flex gap-2">
            {lessonsTotal > 0 && (
              <Button size="sm" variant="outline"
                className="text-destructive hover:text-destructive"
                onClick={handleDeleteAllLessons}
              >
                <ListX className="h-4 w-4 mr-1.5" /> Удалить все
              </Button>
            )}
            <Button size="sm" variant="outline" onClick={openCreateLesson}>
              <Plus className="h-4 w-4 mr-1.5" /> Добавить урок
            </Button>
          </div>
        </div>

        {lessonsLoading ? (
          [...Array(4)].map((_, i) => (
            <div key={i} className="h-8 rounded bg-muted animate-pulse mb-1" />
          ))
        ) : lessonsError ? (
          <ErrorState size="sm" what="уроки" onRetry={() => refetchLessons()} />
        ) : lessons.length === 0 ? (
          <EmptyState
            size="sm"
            icon={CalendarDays}
            title="Уроков в этом периоде нет"
            description="Добавь урок — он появится здесь и в календаре, а после проведения сам отметится как завершённый"
            action={{ label: 'Добавить урок', onClick: openCreateLesson }}
          />
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
                  {lesson.cycle_position != null && lesson.cycle_size != null && (
                    <CycleBadge position={lesson.cycle_position} size={lesson.cycle_size} />
                  )}
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

      <HomeworkEditPopover
        anchor={homeworkAnchor}
        onClose={() => setHomeworkAnchor(null)}
        courseId={id}
        side="bottom"
        align="end"
      />


      <PaymentForm
        open={paymentFormOpen}
        onClose={() => { setPaymentFormOpen(false); setEditingPayment(null) }}
        onSubmit={handlePaymentSubmit}
        pricePerLesson={course ? course.price_per_cycle / course.lessons_per_cycle : 0}
        initialValues={
          editingPayment
            ? {
                amount:        editingPayment.amount,
                lessons_count: editingPayment.lessons_count,
                paid_at:       new Date(editingPayment.paid_at).toISOString().slice(0, 10),
              }
            : undefined
        }
        paymentId={editingPayment?.id}
      />

      {attendanceLesson && (
        <AttendanceDialog
          lessonId={attendanceLesson}
          courseId={id}
          open={!!attendanceLesson}
          onClose={() => setAttendanceLesson(null)}
        />
      )}

      <RecurrenceScopeDialog
        open={!!scopeAsk}
        action={scopeAsk?.action ?? 'edit'}
        onPick={applyScope}
        onClose={() => setScopeAsk(null)}
      />
    </>
  )
}
