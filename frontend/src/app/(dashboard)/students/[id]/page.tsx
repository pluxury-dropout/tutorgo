'use client'

import { useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { ArrowLeft, Pencil, RotateCcw, Snowflake, Trash2, Wallet } from 'lucide-react'

import { useStudent, useUpdateStudent, useRemoveStudent, useRestoreStudent, usePauses, useDeletePause } from '@/lib/hooks/useStudents'
import { useStudentCourses, useUpdateCourse } from '@/lib/hooks/useCourses'
import { useCreatePayment } from '@/lib/hooks/usePayments'
import { StudentForm } from '@/components/students/StudentForm'
import { PauseDialog } from '@/components/students/PauseDialog'
import { PaymentForm } from '@/components/payments/PaymentForm'
import { BulkPaymentDialog } from '@/components/payments/BulkPaymentDialog'
import { PageHeader, HeaderMetric } from '@/components/common/PageHeader'
import { CourseTypeBadge } from '@/components/common/CourseTypeBadge'
import { StudentFormValues } from '@/schemas/student'
import { PaymentFormValues } from '@/schemas/payment'
import { Course, StudentPause } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export default function StudentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const router  = useRouter()

  const { data: student, isLoading }  = useStudent(id)
  const { data: courses = [] }        = useStudentCourses(id)
  const updateStudent = useUpdateStudent(id)
  const removeStudent  = useRemoveStudent()
  const restoreStudent = useRestoreStudent()

  const { data: pauses = [] } = usePauses(id)
  const deletePause = useDeletePause(id)
  const [paymentOpen, setPaymentOpen] = useState(false)
  const [pauseOpen, setPauseOpen]     = useState(false)
  // Один курс — прежняя простая форма; разбивка по предметам — только с двух
  // курсов: большинство учеников на одном предмете (спека 2026-09-06, п. 6.8).
  const single        = courses.length === 1 ? courses[0] : undefined
  const createPayment = useCreatePayment(single?.id ?? '')

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
    // Архивный остаётся на карточке — она покажет плашку «В архиве».
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

  return (
    <>
      <button
        onClick={() => router.push('/students')}
        className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground mb-4"
      >
        <ArrowLeft className="h-4 w-4" /> Все ученики
      </button>

      <PageHeader
        title={`${student.first_name}${student.last_name ? ` ${student.last_name}` : ''}`}
        meta={!student.active ? <HeaderMetric color="var(--muted-foreground)">В архиве</HeaderMetric> : undefined}
        actions={
          <div className="flex gap-2">
            {student.active && courses.length > 0 && (
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

      <div className="border rounded-xl bg-card p-5 max-w-md space-y-3">
        <Row label="Email"   value={student.email} />
        <Row label="Телефон" value={student.phone || '—'} />
      </div>

      <div className="border rounded-xl bg-card p-4 mt-4">
        <h2 className="text-sm font-semibold mb-3">Курсы ({courses.length})</h2>
        {courses.length === 0 ? (
          <p className="text-sm text-muted-foreground">Нет курсов</p>
        ) : (
          <div className="space-y-1">
            {courses.map((course) => (
              <div
                key={course.id}
                className="flex items-center justify-between py-2 border-b last:border-0 text-sm cursor-pointer hover:bg-muted/30 -mx-4 px-4 rounded"
                onClick={() => router.push(`/courses/${course.id}`)}
              >
                <span className="font-medium">{course.subject}</span>
                <div className="flex items-center gap-4 text-muted-foreground shrink-0">
                  <CourseTypeBadge isGroup={!course.student_id} />
                  <CoursePrice course={course} />
                  {/* Цена урока — только подсказка: канон — пакет, и у «85 000 за 12» она дробная */}
                  <span>
                    {course.lessons_per_cycle === 1
                      ? 'за урок'
                      : `за ${course.lessons_per_cycle} ур. · ≈ ${Math.round(course.price_per_cycle / course.lessons_per_cycle).toLocaleString()} / урок`}
                  </span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="border rounded-xl bg-card p-4 mt-4">
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
                  size="icon"
                  variant="ghost"
                  aria-label="Разморозить"
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

      <StudentForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSubmit={handleUpdate}
        initial={student}
      />
      {single && (
        <PaymentForm
          open={paymentOpen}
          onClose={() => setPaymentOpen(false)}
          onSubmit={handleSinglePayment}
          pricePerLesson={single.price_per_cycle / single.lessons_per_cycle}
          lessonsPerCycle={single.lessons_per_cycle}
        />
      )}
      {courses.length >= 2 && (
        <BulkPaymentDialog
          open={paymentOpen}
          onClose={() => setPaymentOpen(false)}
          student={student}
          courses={courses}
        />
      )}
      <PauseDialog
        open={pauseOpen}
        onClose={() => setPauseOpen(false)}
        studentId={id}
        studentName={student.last_name ? `${student.first_name} ${student.last_name}` : student.first_name}
      />
    </>
  )
}

/** Даты паузы приходят полночью UTC — показываем в UTC, иначе на западе от
 *  Гринвича день уехал бы на вчера. */
const fmtDay = (iso: string) => new Date(iso).toLocaleDateString('ru-RU', { timeZone: 'UTC' })

/** Цена курса правится прямо в строке: ради одного числа гонять пользователя
 *  на страницу курса и обратно незачем. Правится сумма пакета, а не цена урока:
 *  пакет вроде «85 000 за 12» на уроки нацело не делится, и запись «цена урока
 *  × N» превращала его в 84 996 от одного клика и потери фокуса. */
function CoursePrice({ course }: { course: Course }) {
  const update = useUpdateCourse(course.id)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft]     = useState('')

  async function save() {
    setEditing(false)
    // Пустое поле — не «цена 0», а передумал: Number('') даёт 0 и молча
    // обнулил бы прайс при потере фокуса.
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
      <Input
        autoFocus
        type="number"
        min={0}
        value={draft}
        onClick={(e) => e.stopPropagation()}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={save}
        onKeyDown={(e) => {
          if (e.key === 'Enter') save()
          if (e.key === 'Escape') setEditing(false)
        }}
        className="h-6 w-24 text-right"
      />
    )
  }

  return (
    <button
      type="button"
      onClick={(e) => { e.stopPropagation(); setDraft(String(course.price_per_cycle)); setEditing(true) }}
      className="hover:text-foreground hover:underline"
    >
      {course.price_per_cycle.toLocaleString()} ₸
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
