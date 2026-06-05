'use client'

import { useState, Suspense, useEffect } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Pencil, Trash2 } from 'lucide-react'
import { toast } from 'sonner'

import { useCourses } from '@/lib/hooks/useCourses'
import {
  usePaymentsPaged,
  useMonthlyIncome,
  useMonthlyExpected,
  useUpdatePayment,
  useDeletePayment,
} from '@/lib/hooks/usePayments'
import { HeaderPanel } from '@/components/HeaderPanel'
import type { KpiSegment } from '@/components/HeaderPanel'
import { Pagination } from '@/components/common/Pagination'
import { PaymentForm } from '@/components/payments/PaymentForm'
import { Button } from '@/components/ui/button'
import type { Payment } from '@/types/api'
import type { PaymentFormValues } from '@/schemas/payment'

const LIMIT = 20

function PaymentsPageInner() {
  const router       = useRouter()
  const searchParams = useSearchParams()
  const [activeSegment, setActiveSegment] = useState('received')
  const [editingPayment, setEditingPayment] = useState<Payment | null>(null)
  const [formOpen, setFormOpen] = useState(false)

  const page = Math.max(1, Number(searchParams.get('page') ?? '1'))

  function handlePageChange(newPage: number) {
    const p = new URLSearchParams(searchParams.toString())
    p.set('page', String(newPage))
    router.push(`/payments?${p}`)
  }

  const { data: courses = [] }                                    = useCourses()
  const { data: pagedPayments, isLoading }                        = usePaymentsPaged({ page, limit: LIMIT })
  const { data: monthlyIncome = 0, isLoading: incomeLoading }     = useMonthlyIncome()
  const { data: monthlyExpected = 0, isLoading: expectedLoading } = useMonthlyExpected()

  const updatePayment = useUpdatePayment()
  const deletePayment = useDeletePayment()

  const payments   = pagedPayments?.data ?? []
  const total      = pagedPayments?.total ?? 0
  const totalPages = Math.ceil(total / LIMIT)

  useEffect(() => {
    if (!isLoading && total > 0 && page > totalPages) handlePageChange(totalPages)
  }, [isLoading, total, page, totalPages]) // eslint-disable-line react-hooks/exhaustive-deps

  const courseMap      = Object.fromEntries(courses.map((c) => [c.id, c.subject]))
  const coursePriceMap = Object.fromEntries(courses.map((c) => [c.id, c.price_per_cycle / c.lessons_per_cycle]))

  function openEdit(p: Payment) {
    setEditingPayment(p)
    setFormOpen(true)
  }

  async function handleEdit(values: PaymentFormValues) {
    if (!editingPayment) return
    await updatePayment.mutateAsync({
      id:   editingPayment.id,
      data: {
        amount:        values.amount,
        lessons_count: values.lessons_count,
        paid_at:       values.paid_at,
      },
    })
    toast.success('Платёж обновлён')
  }

  async function handleDelete(p: Payment) {
    if (!confirm(`Удалить платёж на ${p.amount.toLocaleString()} ₸?`)) return
    await deletePayment.mutateAsync(p.id)
    toast.success('Платёж удалён')
  }

  const segments: KpiSegment[] = [
    {
      id:       'received',
      label:    'Получено',
      value:    '₸ ' + monthlyIncome.toLocaleString('ru-RU'),
      dotColor: 'var(--success)',
      meta:     'этот месяц',
      loading:  incomeLoading,
    },
    {
      id:       'count',
      label:    'Операций',
      value:    String(total),
      dotColor: 'var(--primary)',
      meta:     'всего записей',
      loading:  isLoading,
    },
    {
      id:       'avg',
      label:    'Средний чек',
      value:    '—',
      dotColor: 'var(--warning)',
      meta:     'нет данных',
    },
    {
      id:       'pending',
      label:    'Ожидается',
      value:    '₸ ' + monthlyExpected.toLocaleString('ru-RU'),
      dotColor: 'var(--purple)',
      meta:     'этот месяц',
      loading:  expectedLoading,
    },
  ]

  return (
    <>
      <HeaderPanel
        title="Платежи"
        subtitle={`${total} записей`}
        segments={segments}
        activeSegment={activeSegment}
        onSegmentChange={setActiveSegment}
      />

      <div className="border rounded-lg mt-4 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/40">
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">Дата</th>
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">Курс</th>
              <th className="text-right px-4 py-3 font-medium text-muted-foreground">Сумма</th>
              <th className="text-right px-4 py-3 font-medium text-muted-foreground">Уроков</th>
              <th className="w-20" />
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              [...Array(4)].map((_, i) => (
                <tr key={i}>
                  <td colSpan={5} className="px-4 py-3">
                    <div className="h-4 rounded bg-muted animate-pulse" />
                  </td>
                </tr>
              ))
            ) : payments.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-6 text-center text-muted-foreground">
                  Нет оплат
                </td>
              </tr>
            ) : (
              payments.map((p) => (
                <tr
                  key={p.id}
                  className="border-b last:border-0 hover:bg-muted/30 cursor-pointer group"
                  onClick={() => router.push(`/courses/${p.course_id}`)}
                >
                  <td className="px-4 py-3 text-muted-foreground">
                    {new Date(p.paid_at).toLocaleDateString('ru-RU')}
                  </td>
                  <td className="px-4 py-3 font-medium">{courseMap[p.course_id] ?? '—'}</td>
                  <td className="px-4 py-3 text-right font-medium">{p.amount.toLocaleString()} ₸</td>
                  <td className="px-4 py-3 text-right text-muted-foreground">{p.lessons_count} ур.</td>
                  <td className="pr-2 py-3" onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity duration-150">
                      <Button
                        size="icon"
                        variant="ghost"
                        className="h-7 w-7"
                        onClick={() => openEdit(p)}
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        className="h-7 w-7 text-destructive hover:text-destructive"
                        onClick={() => handleDelete(p)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-between mt-3 px-1">
          <span className="text-xs text-muted-foreground">
            Страница {page} из {totalPages}
          </span>
          <Pagination page={page} totalPages={totalPages} onPageChange={handlePageChange} />
        </div>
      )}

      <PaymentForm
        open={formOpen}
        onClose={() => { setFormOpen(false); setEditingPayment(null) }}
        onSubmit={handleEdit}
        pricePerLesson={editingPayment ? (coursePriceMap[editingPayment.course_id] ?? 0) : 0}
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
    </>
  )
}

export default function PaymentsPage() {
  return (
    <Suspense>
      <PaymentsPageInner />
    </Suspense>
  )
}
