'use client'

import { useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { ArrowLeft, Pencil, RotateCcw, Snowflake, Trash2, Wallet, Video, Plus } from 'lucide-react'

import { useStudentOverview, useUpdateStudent, useRemoveStudent, useRestoreStudent, usePauses, useDeletePause } from '@/lib/hooks/useStudents'
import { useUpdateCourse } from '@/lib/hooks/useCourses'
import { useCreatePayment, useUpdatePayment, useDeletePayment } from '@/lib/hooks/usePayments'
import { useCreateLesson } from '@/lib/hooks/useLessons'
import { StudentForm } from '@/components/students/StudentForm'
import { PauseDialog } from '@/components/students/PauseDialog'
import { PaymentForm } from '@/components/payments/PaymentForm'
import { BulkPaymentDialog } from '@/components/payments/BulkPaymentDialog'
import { LessonForm } from '@/components/lessons/LessonForm'
import { toRecurrenceInput, RecurrenceOptions } from '@/lib/recurrence'
import { CourseBalanceStat } from '@/components/students/CourseBalanceStat'
import { StudentLessonsTab } from '@/components/students/StudentLessonsTab'
import { StudentPaymentsTab } from '@/components/students/StudentPaymentsTab'
import { StudentHomeworkTab } from '@/components/students/StudentHomeworkTab'
import { PageHeader, HeaderMetric } from '@/components/common/PageHeader'
import { CourseTypeBadge } from '@/components/common/CourseTypeBadge'
import { STATUS_LABELS, STATUS_COLORS } from '@/lib/lessonStatus'
import { StudentFormValues } from '@/schemas/student'
import { PaymentFormValues } from '@/schemas/payment'
import { LessonFormValues } from '@/schemas/lesson'
import { StudentCourseSummary, StudentPause, Payment } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'

export default function StudentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const router  = useRouter()

  const { data: overview, isLoading } = useStudentOverview(id)
  const student = overview?.student
  const courses = overview?.courses ?? []
  const payableCourses = overview?.payable_courses ?? []

  const updateStudent = useUpdateStudent(id)
  const removeStudent  = useRemoveStudent()
  const restoreStudent = useRestoreStudent()

  const { data: pauses = [] } = usePauses(id)
  const deletePause = useDeletePause(id)
  const [paymentOpen, setPaymentOpen] = useState(false)
  const [pauseOpen, setPauseOpen]     = useState(false)
  const [lessonOpen, setLessonOpen]   = useState(false)
  const [editingPayment, setEditingPayment] = useState<Payment | null>(null)
  // Один оплачиваемый курс — прежняя простая форма; разбивка — с двух и
  // больше, включая архивные/ушедшие с долгом (спека, п. 6.8, 7.0).
  const single        = payableCourses.length === 1 ? payableCourses[0] : undefined
  const createPayment = useCreatePayment(single?.id ?? '')
  const updatePayment  = useUpdatePayment(editingPayment?.course_id, id)
  const deletePayment  = useDeletePayment(editingPayment?.course_id, id)
  // Урок ставится на первый активный курс по умолчанию — на карточке ученика
  // курсов обычно 1-2, выбор предмета в форме не нужен для частого случая.
  const createLesson = useCreateLesson(courses[0]?.course_id ?? '', id)

  const [formOpen, setFormOpen] = useState(false)

  async function handleUpdate(values: StudentFormValues) {
    await updateStudent.mutateAsync(values)
    toast.success('Ученик обновлён')
  }

  async function handleDelete() {
    if (!student) return
    const result = await removeStudent(student)
    if (result === 'deleted') {
      toast.success('Ученик удалён')
      router.push('/students')
    }
    if (result === 'archived') toast.success('Ученик перенесён в архив')
  }

  async function handleRestore() {
    try {
      await restoreStudent.mutateAsync(id)
      toast.success('Ученик восстановлен')
    } catch {
      toast.error('Не удалось восстановить ученика')
    }
  }

  async function handleSinglePayment(values: PaymentFormValues) {
    if (!single) return
    await createPayment.mutateAsync({
      course_id:     single.id,
      student_id:    id,
      amount:        values.amount,
      lessons_count: values.lessons_count,
      paid_at:       values.paid_at,
    })
    toast.success('Оплата записана')
  }

  async function handlePaymentEdit(values: PaymentFormValues) {
    if (!editingPayment) return
    await updatePayment.mutateAsync({
      id:   editingPayment.id,
      data: { student_id: id, amount: values.amount, lessons_count: values.lessons_count, paid_at: values.paid_at },
    })
    toast.success('Оплата обновлена')
  }

  async function handlePaymentDelete(p: Payment) {
    if (!confirm('Удалить оплату?')) return
    await deletePayment.mutateAsync(p.id)
    toast.success('Оплата удалена')
  }

  async function handleLessonSubmit(values: LessonFormValues, recurrence?: RecurrenceOptions) {
    const baseISO  = new Date(values.scheduled_at).toISOString()
    const courseId = courses[0].course_id
    if (recurrence) {
      // Даты раскатывает сервер — тот же приём, что на странице курса
      // (courses/[id]/page.tsx:161-174).
      await createLesson.mutateAsync({
        course_id:        courseId,
        scheduled_at:     baseISO,
        duration_minutes: values.duration_minutes,
        notes:            values.notes,
        recurrence:       { ...toRecurrenceInput(recurrence), ends_on: courses[0].ended_at ?? undefined },
      })
      toast.success('Серия создана')
    } else {
      await createLesson.mutateAsync({ ...values, scheduled_at: baseISO, course_id: courseId })
      toast.success('Урок добавлен')
    }
    setLessonOpen(false)
  }

  async function handleUnpause(p: StudentPause) {
    if (!confirm('Разморозить? Уроки этого периода снова спишутся с оплаты. Отменённые уроки и продлённая серия останутся как есть.')) return
    try {
      await deletePause.mutateAsync(p.id)
      toast.success('Заморозка снята')
    } catch {
      toast.error('Не удалось снять заморозку')
    }
  }

  if (isLoading) {
    return <div className="h-32 rounded-lg bg-muted animate-pulse" />
  }

  if (!student) {
    return <p className="text-sm text-muted-foreground">Ученик не найден</p>
  }

  const name = student.last_name ? `${student.first_name} ${student.last_name}` : student.first_name

  return (
    <>
      <button
        onClick={() => router.push('/students')}
        className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground mb-4"
      >
        <ArrowLeft className="h-4 w-4" /> Все ученики
      </button>

      <PageHeader
        title={name}
        meta={!student.active ? <HeaderMetric color="var(--muted-foreground)">В архиве</HeaderMetric> : undefined}
        actions={
          <div className="flex gap-2">
            {student.active && payableCourses.length > 0 && (
              <Button size="sm" onClick={() => setPaymentOpen(true)}>
                <Wallet className="h-4 w-4 mr-1.5" /> Оплата
              </Button>
            )}
            {student.active && (
              <Button size="sm" variant="outline" onClick={() => setPauseOpen(true)}>
                <Snowflake className="h-4 w-4 mr-1.5" /> Заморозить
              </Button>
            )}
            <Button size="sm" variant="outline" onClick={() => setFormOpen(true)}>
              <Pencil className="h-4 w-4 mr-1.5" /> Редактировать
            </Button>
            {student.active ? (
              <Button size="sm" variant="destructive" onClick={handleDelete}>
                <Trash2 className="h-4 w-4 mr-1.5" /> Удалить
              </Button>
            ) : (
              <Button size="sm" variant="outline" onClick={handleRestore}>
                <RotateCcw className="h-4 w-4 mr-1.5" /> Восстановить
              </Button>
            )}
          </div>
        }
      />

      <div className="border rounded-xl bg-card p-5 max-w-md space-y-3 mt-4">
        <Row label="Email"   value={student.email} />
        <Row label="Телефон" value={student.phone || '—'} />
      </div>

      <Tabs defaultValue="overview" className="mt-4">
        <TabsList>
          <TabsTrigger value="overview">Обзор</TabsTrigger>
          <TabsTrigger value="lessons">Уроки</TabsTrigger>
          <TabsTrigger value="payments">Оплаты</TabsTrigger>
          <TabsTrigger value="homework">ДЗ и материалы</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4 mt-4">
          {overview.total_owed > 0 && (
            <div className="border border-destructive/30 bg-destructive/5 rounded-xl p-4 text-sm">
              Долг: <span className="font-semibold">{Math.round(overview.total_owed).toLocaleString()} ₸</span>
            </div>
          )}

          <div className="border rounded-xl bg-card p-4">
            <h2 className="text-sm font-semibold mb-3">Предметы ({courses.length})</h2>
            {courses.length === 0 ? (
              <p className="text-sm text-muted-foreground">Нет курсов</p>
            ) : (
              <div className="space-y-3">
                {courses.map((c) => (
                  <div key={c.course_id} className="border-b last:border-0 pb-3 last:pb-0">
                    <div className="flex items-center justify-between text-sm mb-2">
                      <span className="font-medium flex items-center gap-2">
                        {c.subject}
                        <CourseTypeBadge isGroup={c.is_group} />
                      </span>
                      <CoursePrice course={c} />
                    </div>
                    <CourseBalanceStat balance={c.balance} />
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="border rounded-xl bg-card p-4">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-sm font-semibold">Ближайший урок</h2>
              <Button size="sm" variant="outline" onClick={() => setLessonOpen(true)}>
                <Plus className="h-4 w-4 mr-1.5" /> Поставить урок
              </Button>
            </div>
            {overview.next_lesson ? (
              <div className="flex items-center justify-between text-sm">
                <span>
                  {new Date(overview.next_lesson.scheduled_at).toLocaleString('ru-RU', {
                    day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit',
                  })} · {overview.next_lesson.subject}
                </span>
                <Button size="sm" onClick={() => router.push(`/lessons/${overview.next_lesson!.id}/call`)}>
                  <Video className="h-4 w-4 mr-1.5" /> Войти в комнату
                </Button>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">Нет запланированных уроков</p>
            )}
          </div>

          <div className="border rounded-xl bg-card p-4">
            <h2 className="text-sm font-semibold mb-3">Последние уроки</h2>
            {overview.recent_lessons.length === 0 ? (
              <p className="text-sm text-muted-foreground">Уроков ещё не было</p>
            ) : (
              <div className="space-y-1">
                {overview.recent_lessons.map((l) => (
                  <div key={l.id} className="flex items-center justify-between py-1.5 border-b last:border-0 text-sm">
                    <span className="text-muted-foreground">
                      {new Date(l.scheduled_at).toLocaleDateString('ru-RU')} · {l.subject}
                    </span>
                    <span className={`text-xs px-2 py-0.5 rounded-full ${STATUS_COLORS[l.status] ?? ''}`}>
                      {STATUS_LABELS[l.status] ?? l.status}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="border rounded-xl bg-card p-4">
            <h2 className="text-sm font-semibold mb-3">Последние оплаты</h2>
            {overview.payments.length === 0 ? (
              <p className="text-sm text-muted-foreground">Оплат ещё не было</p>
            ) : (
              <div className="space-y-1">
                {overview.payments.map((p) => (
                  <div key={p.id} className="flex items-center justify-between py-1.5 border-b last:border-0 text-sm group">
                    <span className="text-muted-foreground">{new Date(p.paid_at).toLocaleDateString('ru-RU')} · {p.subject}</span>
                    <span className="font-medium">{p.amount.toLocaleString()} ₸ · {p.lessons_count} ур.</span>
                    <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <Button size="icon" variant="ghost" className="h-7 w-7"
                        onClick={() => { setEditingPayment(p); setPaymentOpen(true) }}>
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button size="icon" variant="ghost" className="h-7 w-7 text-destructive hover:text-destructive"
                        onClick={() => handlePaymentDelete(p)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="border rounded-xl bg-card p-4">
            <h2 className="text-sm font-semibold mb-3">Заморозки</h2>
            {pauses.length === 0 ? (
              <p className="text-sm text-muted-foreground">Не замораживался</p>
            ) : (
              <div className="space-y-1">
                {pauses.map((p) => (
                  <div key={p.id} className="flex items-center justify-between py-2 border-b last:border-0 text-sm group">
                    <span>
                      {fmtDay(p.starts_on)} — {fmtDay(p.ends_on)}
                      {p.reason && <span className="text-muted-foreground"> · {p.reason}</span>}
                    </span>
                    <Button
                      size="icon" variant="ghost" aria-label="Разморозить"
                      className="h-7 w-7 text-destructive hover:text-destructive opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
                      onClick={() => handleUnpause(p)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </TabsContent>

        <TabsContent value="lessons" className="mt-4">
          <StudentLessonsTab courses={courses} />
        </TabsContent>
        <TabsContent value="payments" className="mt-4">
          <StudentPaymentsTab courses={courses} />
        </TabsContent>
        <TabsContent value="homework" className="mt-4">
          <StudentHomeworkTab courses={courses} />
        </TabsContent>
      </Tabs>

      <StudentForm open={formOpen} onClose={() => setFormOpen(false)} onSubmit={handleUpdate} initial={student} />
      {single && (
        <PaymentForm
          open={paymentOpen}
          onClose={() => { setPaymentOpen(false); setEditingPayment(null) }}
          onSubmit={editingPayment ? handlePaymentEdit : handleSinglePayment}
          pricePerLesson={single.price_per_cycle / single.lessons_per_cycle}
          lessonsPerCycle={single.lessons_per_cycle}
          initialValues={editingPayment ? {
            amount: editingPayment.amount, lessons_count: editingPayment.lessons_count,
            paid_at: new Date(editingPayment.paid_at).toISOString().slice(0, 10),
          } : undefined}
          paymentId={editingPayment?.id}
        />
      )}
      {payableCourses.length >= 2 && !editingPayment && (
        <BulkPaymentDialog open={paymentOpen} onClose={() => setPaymentOpen(false)} student={student} courses={payableCourses} />
      )}
      <PauseDialog open={pauseOpen} onClose={() => setPauseOpen(false)} studentId={id} studentName={name} />
      {courses[0] && (
        <LessonForm
          open={lessonOpen}
          onClose={() => setLessonOpen(false)}
          onSubmit={handleLessonSubmit}
          courseEndAt={courses[0].ended_at ?? undefined}
        />
      )}
    </>
  )
}

const fmtDay = (iso: string) => new Date(iso).toLocaleDateString('ru-RU', { timeZone: 'UTC' })

function CoursePrice({ course }: { course: StudentCourseSummary }) {
  const update = useUpdateCourse(course.course_id)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft]     = useState('')

  async function save() {
    setEditing(false)
    if (draft.trim() === '') return
    const next = Number(draft)
    if (!Number.isFinite(next) || next < 0 || next === course.price_per_cycle) return
    try {
      await update.mutateAsync({
        subject:           course.subject,
        price_per_cycle:   next,
        lessons_per_cycle: course.lessons_per_cycle,
        started_at:        course.started_at,
        ended_at:          course.ended_at ?? undefined,
      })
      toast.success('Цена обновлена')
    } catch {
      toast.error('Не удалось обновить цену')
    }
  }

  if (editing) {
    return (
      <Input autoFocus type="number" min={0} value={draft}
        onChange={(e) => setDraft(e.target.value)} onBlur={save}
        onKeyDown={(e) => { if (e.key === 'Enter') save(); if (e.key === 'Escape') setEditing(false) }}
        className="h-6 w-24 text-right" />
    )
  }

  return (
    <button type="button"
      onClick={() => { setDraft(String(course.price_per_cycle)); setEditing(true) }}
      className="text-sm hover:text-foreground hover:underline text-right"
    >
      {course.price_per_cycle.toLocaleString()} ₸
      <span className="text-muted-foreground ml-1">
        {course.lessons_per_cycle === 1 ? 'за урок' : `за ${course.lessons_per_cycle} ур.`}
      </span>
    </button>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center gap-4">
      <span className="text-sm text-muted-foreground w-20 shrink-0">{label}</span>
      <span className="text-sm font-medium">{value}</span>
    </div>
  )
}
