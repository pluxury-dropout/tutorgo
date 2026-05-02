'use client'

import { useQueries } from '@tanstack/react-query'
import { useRouter } from 'next/navigation'
import { ChevronRight, CreditCard } from 'lucide-react'

import { useCourses } from '@/lib/hooks/useCourses'
import { paymentKeys } from '@/lib/hooks/usePayments'
import { paymentsApi } from '@/lib/api/payments'
import { Payment } from '@/types/api'
import { PageHeader } from '@/components/common/PageHeader'

function filterByMonth(payments: Payment[], year: number, month: number) {
  return payments.filter((p) => {
    const d = new Date(p.paid_at)
    return d.getFullYear() === year && d.getMonth() === month
  })
}

function total(payments: Payment[]) {
  return payments.reduce((sum, p) => sum + p.amount, 0)
}

function StatCard({ title, amount, count, highlight }: {
  title: string
  amount: number
  count: number
  highlight?: boolean
}) {
  return (
    <div className="bg-card border border-border rounded-[var(--radius-lg)] p-5 shadow-[var(--shadow-card)]">
      <p className="text-xs font-medium text-muted-foreground mb-1">{title}</p>
      {highlight ? (
        <p className="text-[28px] font-bold leading-none bg-gradient-to-r from-amber-500 to-yellow-400 bg-clip-text text-transparent">
          {amount.toLocaleString()} ₸
        </p>
      ) : (
        <p className="text-[26px] font-bold leading-none text-foreground">{amount.toLocaleString()} ₸</p>
      )}
      <p className="text-xs text-muted-foreground mt-1">{count} оплат</p>
    </div>
  )
}

export default function PaymentsPage() {
  const router = useRouter()
  const { data: courses = [] } = useCourses()

  const results = useQueries({
    queries: courses.map((c) => ({
      queryKey: paymentKeys.byCourse(c.id),
      queryFn:  () => paymentsApi.list(c.id),
    })),
  })

  const allPayments = results.flatMap((r) => r.data ?? [])
    .sort((a, b) => new Date(b.paid_at).getTime() - new Date(a.paid_at).getTime())

  const now        = new Date()
  const thisMonth  = filterByMonth(allPayments, now.getFullYear(), now.getMonth())
  const prevDate   = new Date(now.getFullYear(), now.getMonth() - 1, 1)
  const lastMonth  = filterByMonth(allPayments, prevDate.getFullYear(), prevDate.getMonth())

  const courseMap  = Object.fromEntries(courses.map((c) => [c.id, c.subject]))
  const isLoading  = results.some((r) => r.isLoading) && allPayments.length === 0

  return (
    <>
      <PageHeader
        title="Платежи"
        description={`${allPayments.length} записей`}
        icon={CreditCard}
        iconBg="var(--accent-light)"
        iconColor="oklch(0.52 0.18 55)"
      />

      <div className="grid grid-cols-3 gap-4 mt-4">
        <StatCard title="За всё время"  amount={total(allPayments)} count={allPayments.length} />
        <StatCard title="Прошлый месяц" amount={total(lastMonth)}   count={lastMonth.length} />
        <StatCard title="Этот месяц"    amount={total(thisMonth)}   count={thisMonth.length}  highlight />
      </div>

      <div className="border rounded-lg mt-4 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/40">
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">Дата</th>
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">Курс</th>
              <th className="text-right px-4 py-3 font-medium text-muted-foreground">Сумма</th>
              <th className="text-right px-4 py-3 font-medium text-muted-foreground">Уроков</th>
              <th className="w-4" />
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              [...Array(4)].map((_, i) => (
                <tr key={i}>
                  <td colSpan={4} className="px-4 py-3">
                    <div className="h-4 rounded bg-muted animate-pulse" />
                  </td>
                </tr>
              ))
            ) : allPayments.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-4 py-6 text-center text-muted-foreground">
                  Нет оплат
                </td>
              </tr>
            ) : (
              allPayments.map((p) => (
                <tr
                  key={p.id}
                  className="border-b last:border-0 hover:bg-muted/30 cursor-pointer group"
                  onClick={() => router.push(`/courses/${p.course_id}`)}
                >
                  <td className="px-4 py-3 text-muted-foreground">
                    {new Date(p.paid_at).toLocaleDateString('ru-RU')}
                  </td>
                  <td className="px-4 py-3 font-medium">
                    {courseMap[p.course_id] ?? '—'}
                  </td>
                  <td className="px-4 py-3 text-right font-medium">
                    {p.amount.toLocaleString()} ₸
                  </td>
                  <td className="px-4 py-3 text-right text-muted-foreground">
                    {p.lessons_count} ур.
                  </td>
                  <td className="pr-3 py-3 w-4">
                    <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity duration-150" />
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </>
  )
}
