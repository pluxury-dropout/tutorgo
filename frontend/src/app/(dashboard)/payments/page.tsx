'use client'

import { Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { ChevronRight, CreditCard } from 'lucide-react'

import { useCourses } from '@/lib/hooks/useCourses'
import { usePaymentsPaged, useMonthlyIncome } from '@/lib/hooks/usePayments'
import { PageHeader } from '@/components/common/PageHeader'
import { Pagination } from '@/components/common/Pagination'

const LIMIT = 20

function PaymentsPageInner() {
  const router       = useRouter()
  const searchParams = useSearchParams()

  const page = Math.max(1, Number(searchParams.get('page') ?? '1'))

  function handlePageChange(newPage: number) {
    const p = new URLSearchParams(searchParams.toString())
    p.set('page', String(newPage))
    router.push(`/payments?${p}`)
  }

  const { data: courses = [] }                       = useCourses()
  const { data: pagedPayments, isLoading }           = usePaymentsPaged({ page, limit: LIMIT })
  const { data: monthlyIncome = 0 }                  = useMonthlyIncome()

  const payments   = pagedPayments?.data ?? []
  const total      = pagedPayments?.total ?? 0
  const totalPages = Math.ceil(total / LIMIT)

  const courseMap = Object.fromEntries(courses.map((c) => [c.id, c.subject]))

  return (
    <>
      <PageHeader
        title="Платежи"
        description={`${total} записей`}
        icon={CreditCard}
        iconBg="var(--accent-light)"
        iconColor="oklch(0.52 0.18 55)"
      />

      <div className="bg-card border border-border rounded-[var(--radius-lg)] p-5 shadow-[var(--shadow-card)] mt-4 inline-block min-w-[200px]">
        <p className="text-xs font-medium text-muted-foreground mb-1">Этот месяц</p>
        <p className="text-[28px] font-bold leading-none bg-gradient-to-r from-amber-500 to-yellow-400 bg-clip-text text-transparent">
          {monthlyIncome.toLocaleString()} ₸
        </p>
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
                  <td className="pr-3 py-3 w-4">
                    <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity duration-150" />
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
