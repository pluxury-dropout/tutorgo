'use client'

import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { useCreateBulkPayment, useDebts } from '@/lib/hooks/usePayments'
import { amountFor } from '@/lib/money'
import type { ApiError, Course, Student } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

interface Row {
  lessons: number
  amount:  number
  /** Сумму правили руками — пересчёт от уроков её больше не трогает. */
  amountEdited: boolean
}

const EMPTY_ROW: Row = { lessons: 0, amount: 0, amountEdited: false }

const fmt = (n: number) => Math.round(n).toLocaleString('ru-RU')
const today = () => new Date().toISOString().slice(0, 10)

function priceLabel(c: Course) {
  return c.lessons_per_cycle > 1
    ? `${fmt(c.price_per_cycle)} ₸ за ${c.lessons_per_cycle} ур.`
    : `${fmt(c.price_per_cycle)} ₸ / урок`
}

interface BulkPaymentDialogProps {
  open:    boolean
  onClose: () => void
  student: Student
  /** Активные курсы ученика — их два и больше (спека 2026-09-06, п. 6.8). */
  courses: Course[]
}

/** Одна оплата на несколько предметов (спека 2026-09-06, п. 6.8). Истина —
 *  строки: в БД уезжают только они, поле «К распределению» лишь считает. */
export function BulkPaymentDialog({ open, onClose, student, courses }: BulkPaymentDialogProps) {
  const createBulk = useCreateBulkPayment()
  const { data: debts = [] } = useDebts()

  const [paidAt, setPaidAt] = useState(today)
  const [toDistribute, setToDistribute] = useState('')
  const [rows, setRows] = useState<Record<string, Row>>({})

  // Каждое открытие — с нулей: раскладывать чужие деньги по предметам за
  // тьютора форма не должна, в деньгах предсказуемость дороже клика.
  useEffect(() => {
    if (open) {
      setPaidAt(today())
      setToDistribute('')
      setRows({})
    }
  }, [open])

  const owedByCourse = new Map(
    (debts.find((d) => d.student_id === student.id)?.courses ?? [])
      .map((c) => [c.course_id, c.lessons_owed] as const),
  )

  const rowOf = (courseId: string) => rows[courseId] ?? EMPTY_ROW

  function setLessons(course: Course, value: number) {
    const lessons = Number.isFinite(value) && value > 0 ? Math.floor(value) : 0
    setRows((prev) => {
      const row = prev[course.id] ?? EMPTY_ROW
      const amount = row.amountEdited
        ? row.amount
        : amountFor(lessons, course.price_per_cycle, course.lessons_per_cycle)
      return { ...prev, [course.id]: { ...row, lessons, amount } }
    })
  }

  function setAmount(courseId: string, value: number) {
    const amount = Number.isFinite(value) && value > 0 ? value : 0
    setRows((prev) => ({ ...prev, [courseId]: { ...(prev[courseId] ?? EMPTY_ROW), amount, amountEdited: true } }))
  }

  const recorded   = courses.reduce((sum, c) => sum + rowOf(c.id).amount, 0)
  const target     = Number(toDistribute)
  const hasTarget  = toDistribute.trim() !== '' && Number.isFinite(target) && target > 0

  async function submit() {
    const items = courses
      .filter((c) => rowOf(c.id).lessons > 0 && rowOf(c.id).amount > 0)
      .map((c) => ({
        course_id:     c.id,
        student_id:    student.id,
        amount:        rowOf(c.id).amount,
        lessons_count: rowOf(c.id).lessons,
      }))
    if (items.length === 0) {
      toast.error('Укажите уроки хотя бы по одному предмету')
      return
    }
    try {
      await createBulk.mutateAsync({ paid_at: paidAt, items })
      toast.success('Оплата записана')
      onClose()
    } catch (err) {
      toast.error((err as ApiError).message ?? 'Не удалось записать оплату')
    }
  }

  const name = student.last_name ? `${student.first_name} ${student.last_name}` : student.first_name

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Оплата — {name}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 pt-2">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              {/* Не «Получено»: эта сумма нигде не хранится, в отчёт идут строки ниже. */}
              <Label htmlFor="bulk-to-distribute">К распределению, ₸</Label>
              <Input
                id="bulk-to-distribute"
                type="number"
                min={0}
                placeholder="необязательно"
                value={toDistribute}
                onChange={(e) => setToDistribute(e.target.value)}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="bulk-paid-at">Дата оплаты</Label>
              <Input id="bulk-paid-at" type="date" value={paidAt} onChange={(e) => setPaidAt(e.target.value)} />
            </div>
          </div>

          <div>
            {courses.map((course) => {
              const row  = rowOf(course.id)
              const owed = owedByCourse.get(course.id) ?? 0
              return (
                <div key={course.id} className="flex items-center justify-between gap-3 py-2 border-b last:border-0">
                  <div className="min-w-0">
                    <p className="text-sm font-medium truncate">{course.subject}</p>
                    <p className="text-xs text-muted-foreground">
                      {priceLabel(course)}
                      {owed > 0 && (
                        <>
                          {' · '}долг {owed} ур.{' '}
                          <button
                            type="button"
                            className="underline hover:text-foreground"
                            onClick={() => setLessons(course, owed)}
                          >
                            погасить
                          </button>
                        </>
                      )}
                    </p>
                  </div>
                  <div className="flex items-center gap-1.5 shrink-0">
                    <Input
                      type="number"
                      min={0}
                      aria-label={`Уроков: ${course.subject}`}
                      className="h-8 w-16 text-right"
                      placeholder="0"
                      value={row.lessons || ''}
                      onChange={(e) => setLessons(course, Number(e.target.value))}
                    />
                    <span className="text-xs text-muted-foreground">ур. =</span>
                    <Input
                      type="number"
                      min={0}
                      aria-label={`Сумма: ${course.subject}`}
                      className="h-8 w-24 text-right"
                      placeholder="0"
                      value={row.amount || ''}
                      onChange={(e) => setAmount(course.id, Number(e.target.value))}
                    />
                    <span className="text-xs text-muted-foreground">₸</span>
                  </div>
                </div>
              )
            })}
          </div>

          {/* Информирует, но не блокирует: округлили, простили тысячу, доплатят потом. */}
          <p className="text-xs text-muted-foreground">
            {hasTarget
              ? `Записывается ${fmt(recorded)} из ${fmt(target)} ₸` +
                (Math.round(recorded) !== Math.round(target) ? ` · разница ${fmt(Math.abs(target - recorded))} ₸` : '')
              : `Записывается ${fmt(recorded)} ₸`}
          </p>

          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={onClose}>Отмена</Button>
            <Button type="button" onClick={submit} disabled={createBulk.isPending}>
              {createBulk.isPending ? 'Сохранение...' : 'Записать'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
