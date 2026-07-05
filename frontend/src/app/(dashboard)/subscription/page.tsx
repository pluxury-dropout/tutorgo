// frontend/src/app/(dashboard)/subscription/page.tsx
'use client'

import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { subscriptionApi, Subscription, SubState, Plan } from '@/lib/api/subscription'
import { PageHeader } from '@/components/common/PageHeader'
import { Button } from '@/components/ui/button'

const STATE_META: Record<SubState, { label: string; cls: string }> = {
  active:  { label: 'Активна',  cls: 'bg-[var(--status-completed-bg)] text-[var(--status-completed-text)]' },
  grace:   { label: 'Истекла — оплатите', cls: 'bg-[var(--status-scheduled-bg)] text-[var(--status-scheduled-text)]' },
  blocked: { label: 'Заблокирована', cls: 'bg-[var(--status-cancelled-bg)] text-[var(--status-cancelled-text)]' },
}

function formatPrice(amount: number, currency: string): string {
  return new Intl.NumberFormat('ru-RU', {
    style: 'currency',
    currency,
    maximumFractionDigits: 0,
  }).format(amount)
}

export default function SubscriptionPage() {
  const [sub, setSub] = useState<Subscription | null>(null)
  const [paying, setPaying] = useState<Plan | null>(null)

  async function load() {
    try {
      setSub(await subscriptionApi.get())
    } catch {
      toast.error('Не удалось загрузить статус подписки')
    }
  }

  useEffect(() => { load() }, [])

  async function onPay(plan: Plan) {
    setPaying(plan)
    try {
      await subscriptionApi.checkout(plan)
      await subscriptionApi.confirm(plan)
      const fresh = await subscriptionApi.get()
      setSub(fresh)
      if (fresh.state === 'active') {
        toast.success('Подписка активирована')
        // ВАЖНО: жёсткая навигация, а не router.replace. Layout группы (dashboard)
        // остаётся смонтированным при клиентском переходе и держит устаревший
        // subState='blocked' → guard отбросил бы обратно на paywall. Hard-nav
        // перемонтирует layout и перечитывает статус. Побочно: кейс «пришёл из
        // профиля → остаться» схлопывается в редирект на dashboard — ок для MVP.
        window.location.href = '/dashboard'
      }
    } catch {
      toast.error('Оплата не прошла, попробуйте ещё раз')
    } finally {
      setPaying(null)
    }
  }

  if (!sub) {
    return (
      <>
        <PageHeader title="Подписка" />
        <div className="mt-6 text-sm text-muted-foreground">Загрузка…</div>
      </>
    )
  }

  const meta = STATE_META[sub.state]
  const { monthly, yearly, currency } = sub.prices
  const discount = Math.round((1 - yearly / (monthly * 12)) * 100)

  return (
    <>
      <PageHeader title="Подписка" />

      <div className="mt-6 max-w-2xl space-y-6">
        <div className="flex items-center gap-3">
          <span className="text-sm text-muted-foreground">Статус:</span>
          <span className={`inline-flex items-center rounded-[20px] px-[9px] py-[3px] text-xs font-semibold ${meta.cls}`}>
            {meta.label}
          </span>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          {/* Месяц */}
          <div className="border rounded-lg p-5 space-y-3">
            <h2 className="text-sm font-semibold">Помесячно</h2>
            <p className="text-2xl font-bold">{formatPrice(monthly, currency)}<span className="text-sm font-normal text-muted-foreground"> / мес</span></p>
            <Button className="w-full" disabled={paying !== null} onClick={() => onPay('monthly')}>
              {paying === 'monthly' ? 'Оплата…' : 'Оплатить'}
            </Button>
          </div>

          {/* Год */}
          <div className="border rounded-lg p-5 space-y-3 relative">
            {discount > 0 && (
              <span className="absolute top-3 right-3 rounded-[20px] bg-primary/10 text-primary px-2 py-px text-[11px] font-semibold">
                −{discount}%
              </span>
            )}
            <h2 className="text-sm font-semibold">На год</h2>
            <p className="text-2xl font-bold">{formatPrice(yearly, currency)}<span className="text-sm font-normal text-muted-foreground"> / год</span></p>
            <Button className="w-full" disabled={paying !== null} onClick={() => onPay('yearly')}>
              {paying === 'yearly' ? 'Оплата…' : 'Оплатить'}
            </Button>
          </div>
        </div>
      </div>
    </>
  )
}
