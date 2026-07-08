'use client'

import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { subscriptionApi } from '@/lib/api/subscription'
import { pollDecision } from '@/lib/subscriptionPoll'
import { PageHeader } from '@/components/common/PageHeader'
import { Button } from '@/components/ui/button'

const MAX_ATTEMPTS = 15
const INTERVAL_MS = 2000

export default function SubscriptionSuccessPage() {
  const [timedOut, setTimedOut] = useState(false)
  const started = useRef(false)

  useEffect(() => {
    if (started.current) return // StrictMode dev-double-mount guard
    started.current = true

    let attempts = 0
    let timer: ReturnType<typeof setTimeout>

    async function poll() {
      attempts++
      const attemptsLeft = MAX_ATTEMPTS - attempts
      try {
        const sub = await subscriptionApi.get()
        const decision = pollDecision(sub.state, attemptsLeft)
        if (decision === 'activated') {
          toast.success('Подписка активирована')
          // hard-nav: layout группы (dashboard) держит устаревший subState → нужен
          // ремоунт, иначе guard отбросит на paywall. См. subscription/page.tsx.
          window.location.href = '/dashboard'
          return
        }
        if (decision === 'timeout') {
          setTimedOut(true)
          return
        }
      } catch {
        // сеть/5xx — не срываем поллинг, пробуем ещё, пока есть попытки
        if (attemptsLeft <= 0) {
          setTimedOut(true)
          return
        }
      }
      timer = setTimeout(poll, INTERVAL_MS)
    }

    poll()
    return () => clearTimeout(timer)
  }, [])

  return (
    <>
      <PageHeader title="Оплата" />
      <div className="mt-6 max-w-md space-y-4">
        {timedOut ? (
          <>
            <p className="text-sm text-muted-foreground">
              Оплата обрабатывается. Это может занять пару минут — обновите страницу позже.
            </p>
            <Button onClick={() => (window.location.href = '/dashboard')}>В дашборд</Button>
          </>
        ) : (
          <p className="text-sm text-muted-foreground">Проверяем оплату…</p>
        )}
      </div>
    </>
  )
}
