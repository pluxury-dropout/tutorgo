'use client'

import { useState, Suspense, useEffect } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Pencil, Trash2, Wallet } from 'lucide-react'
import { toast } from 'sonner'

import { useCourses } from '@/lib/hooks/useCourses'
import {
  usePaymentsPaged,
  useMonthlyIncome,
  useMonthlyExpected,
  useUpdatePayment,
  useDeletePayment,
} from '@/lib/hooks/usePayments'
import { Pagination } from '@/components/common/Pagination'
import { SectionCard } from '@/components/common/SectionCard'
import { EmptyState } from '@/components/common/EmptyState'
import { ErrorState } from '@/components/common/ErrorState'
import { PaymentForm } from '@/components/payments/PaymentForm'
import { DebtsList } from '@/components/payments/DebtsList'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import type { Payment } from '@/types/api'
import type { PaymentFormValues } from '@/schemas/payment'

const LIMIT = 20

const fmtAmt = (n: number) => '₸' + n.toLocaleString('ru-RU')

function PaymentsPageInner() {
  const router       = useRouter()
  const searchParams = useSearchParams()
  const [editingPayment, setEditingPayment] = useState<Payment | null>(null)
  const [formOpen, setFormOpen] = useState(false)

  const page = Math.max(1, Number(searchParams.get('page') ?? '1'))
  const tab  = searchParams.get('tab') === 'debts' ? 'debts' : 'history'

  function handlePageChange(newPage: number) {
    const p = new URLSearchParams(searchParams.toString())
    p.set('page', String(newPage))
    router.push(`/payments?${p}`)
  }

  function handleTabChange(next: string) {
    const p = new URLSearchParams(searchParams.toString())
    if (next === 'debts') p.set('tab', 'debts')
    else p.delete('tab')
    p.delete('page')
    router.push(`/payments?${p}`)
  }

  const { data: courses = [] }                                    = useCourses()
  const { data: pagedPayments, isLoading, isError, refetch }      = usePaymentsPaged({ page, limit: LIMIT })
  const { data: monthlyIncome = 0, isLoading: incomeLoading }     = useMonthlyIncome()
  const { data: monthlyExpected = 0, isLoading: expectedLoading } = useMonthlyExpected()

  const updatePayment = useUpdatePayment()
  const deletePayment = useDeletePayment()

  const payments   = pagedPayments?.data ?? []
  const total      = pagedPayments?.total ?? 0
  const totalPages = Math.ceil(total / LIMIT)

  useEffect(() => {
    if (tab === 'history' && !isLoading && total > 0 && page > totalPages) handlePageChange(totalPages)
  }, [tab, isLoading, total, page, totalPages]) // eslint-disable-line react-hooks/exhaustive-deps

  // Название курса приходит с платежом: useCourses отдаёт только активные курсы
  // и только первую страницу, поэтому маппингом по нему платежи по архивным
  // курсам показывались как «—». Курсы остались нужны ради цены урока и пачки.
  const coursePriceMap   = Object.fromEntries(courses.map((c) => [c.id, c.price_per_cycle / c.lessons_per_cycle]))
  const courseLessonsMap = Object.fromEntries(courses.map((c) => [c.id, c.lessons_per_cycle]))

  function openEdit(p: Payment) {
    // Адресата легаси-платежу группы выбирают из её состава — он есть только на
    // странице курса.
    if (!p.student_id) {
      toast.info('У платежа нет адресата — назначьте ученика на странице курса')
      router.push(`/courses/${p.course_id}`)
      return
    }
    setEditingPayment(p)
    setFormOpen(true)
  }

  async function handleEdit(values: PaymentFormValues) {
    if (!editingPayment?.student_id) return
    await updatePayment.mutateAsync({
      id:   editingPayment.id,
      data: {
        student_id:    editingPayment.student_id,
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

  const kpis = [
    { color: 'var(--success)', value: incomeLoading   ? '…' : fmtAmt(monthlyIncome),   label: 'получено'    },
    { color: 'var(--primary)', value: isLoading        ? '…' : String(total),          label: 'операций'    },
    { color: 'var(--warning)', value: '—',                                             label: 'средний чек' },
    { color: 'var(--purple)',  value: expectedLoading  ? '…' : fmtAmt(monthlyExpected), label: 'ожидается'   },
  ]

  return (
    <div style={{ maxWidth: 900 }}>
      {/* Заголовок */}
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
        <h1 style={{ margin: 0, fontSize: 20, fontWeight: 700, color: 'var(--foreground)', letterSpacing: '-0.01em' }}>
          Платежи
        </h1>
        <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>{total} записей</span>
      </div>

      {/* KPI — стандартный формат главной */}
      <div style={{
        border: '1px solid var(--border)', borderRadius: 12, background: 'var(--card)',
        padding: '10px 16px', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '6px 24px',
        width: 360, marginTop: 14,
      }}>
        {kpis.map((k, i) => (
          <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 0' }}>
            <span style={{ width: 6, height: 6, borderRadius: '50%', background: k.color, flexShrink: 0 }} />
            <span style={{ fontSize: 13.5, fontWeight: 700, color: 'var(--foreground)', fontVariantNumeric: 'tabular-nums' }}>
              {k.value}
            </span>
            <span style={{ fontSize: 12, color: 'var(--muted-foreground)' }}>{k.label}</span>
          </div>
        ))}
      </div>

      <Tabs value={tab} onValueChange={(v) => handleTabChange(v as string)}>
        <TabsList className="mt-4">
          <TabsTrigger value="history">История</TabsTrigger>
          <TabsTrigger value="debts">Долги</TabsTrigger>
        </TabsList>

        <TabsContent value="history">
          <SectionCard style={{ marginTop: 16 }}>
            {/* Column headers */}
            <div style={{
              display: 'grid',
              gridTemplateColumns: '90px 1fr 110px 70px 64px',
              alignItems: 'baseline',
              gap: 12,
              padding: '10px 18px 8px',
              borderBottom: '1px solid var(--border)',
            }}>
              {(['Дата', 'Курс и ученик', 'Сумма', 'Уроков', ''] as const).map((label, i) => (
                <span key={i} style={{
                  fontSize: 11.5, fontWeight: 500,
                  color: 'var(--muted-foreground)',
                  letterSpacing: '0.05em',
                  textTransform: 'uppercase',
                  ...(i === 2 || i === 3 ? { textAlign: 'right' } : {}),
                }}>{label}</span>
              ))}
            </div>
            {/* Rows */}
            {isLoading ? (
              <div className="space-y-2" style={{ padding: '10px 18px' }}>
                {[...Array(4)].map((_, i) => (
                  <div key={i} className="h-4 rounded bg-muted animate-pulse" />
                ))}
              </div>
            ) : isError ? (
              <ErrorState what="платежи" onRetry={() => refetch()} />
            ) : payments.length === 0 ? (
              <EmptyState
                icon={Wallet}
                title="Оплат пока нет"
                description="Оплата отмечается на странице курса — приложение посчитает, на сколько уроков хватит баланса, и напомнит, когда пора брать следующую"
                action={{ label: 'Перейти к курсам', href: '/courses' }}
              />
            ) : (
              payments.map((p, i) => (
                <div
                  key={p.id}
                  role="button"
                  tabIndex={0}
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '90px 1fr 110px 70px 64px',
                    alignItems: 'center',
                    gap: 12,
                    padding: '10px 18px',
                    borderTop: i === 0 ? 'none' : '1px solid var(--row-border)',
                    cursor: 'pointer',
                  }}
                  className="hover:bg-muted/30 group"
                  onClick={() => router.push(`/courses/${p.course_id}`)}
                  onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); router.push(`/courses/${p.course_id}`) } }}
                >
                  <span style={{ fontSize: 13, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
                    {new Date(p.paid_at).toLocaleDateString('ru-RU')}
                  </span>
                  <div style={{ minWidth: 0 }}>
                    <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--foreground)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {p.subject ?? '—'}
                    </div>
                    <div style={{ fontSize: 12, color: 'var(--muted-foreground)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {p.student_name ?? 'Группа · без адресата'}
                    </div>
                  </div>
                  <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--foreground)', textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
                    {p.amount.toLocaleString()} ₸
                  </span>
                  <span style={{ fontSize: 13, color: 'var(--muted-foreground)', textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
                    {p.lessons_count} ур.
                  </span>
                  <div
                    className="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity duration-150"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => openEdit(p)}>
                      <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button size="icon" variant="ghost"
                      className="h-7 w-7 text-destructive hover:text-destructive"
                      onClick={() => handleDelete(p)}>
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
              ))
            )}
          </SectionCard>

          {totalPages > 1 && (
            <div className="flex items-center justify-between mt-3 px-1">
              <span className="text-xs text-muted-foreground">
                Страница {page} из {totalPages}
              </span>
              <Pagination page={page} totalPages={totalPages} onPageChange={handlePageChange} />
            </div>
          )}
        </TabsContent>

        <TabsContent value="debts" className="mt-4">
          <DebtsList />
        </TabsContent>
      </Tabs>

      <PaymentForm
        open={formOpen}
        onClose={() => { setFormOpen(false); setEditingPayment(null) }}
        onSubmit={handleEdit}
        pricePerLesson={editingPayment ? (coursePriceMap[editingPayment.course_id] ?? 0) : 0}
        lessonsPerCycle={editingPayment ? (courseLessonsMap[editingPayment.course_id] ?? 0) : 0}
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
    </div>
  )
}

export default function PaymentsPage() {
  return (
    <Suspense>
      <PaymentsPageInner />
    </Suspense>
  )
}
