'use client'

import { useEffect, useState } from 'react'
import { useRouter, usePathname } from 'next/navigation'
import Link from 'next/link'
import { Menu, GraduationCap, X } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { Sidebar } from '@/components/layout/Sidebar'
import { MobileBottomNav } from '@/components/layout/MobileBottomNav'
import { subscriptionApi, SubState } from '@/lib/api/subscription'
import { decideAccess, stateOnLoadError, PAYWALL_PATH } from '@/lib/subscriptionGuard'
import { withRetry } from '@/lib/retry'

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { token } = useAuthStore()
  const isAuthenticated = !!token
  const router = useRouter()
  const pathname = usePathname()
  const [mounted, setMounted] = useState(false)
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [subState, setSubState] = useState<SubState | null>(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [bannerDismissed, setBannerDismissed] = useState(false)

  useEffect(() => { setMounted(true) }, [])

  useEffect(() => {
    if (mounted && !isAuthenticated) router.replace('/login')
  }, [mounted, isAuthenticated, router])

  // Guard подписки: грузим только когда аутентифицированы. Разбор ошибки —
  // в stateOnLoadError: обрыв связи оставляет null (спиннер), перезапрос ниже.
  useEffect(() => {
    if (!mounted || !isAuthenticated) return
    let alive = true
    withRetry(() => subscriptionApi.get())
      .then((s) => { if (alive) setSubState(s.state) })
      .catch((e: { status?: number }) => {
        if (alive) setSubState(stateOnLoadError(e?.status))
      })
    return () => { alive = false }
  }, [mounted, isAuthenticated, reloadKey])

  // Мобильный браузер морозит фоновую вкладку и рвёт соединение. При возврате
  // компонент не перемонтируется — эффект выше сам не выстрелит, дёргаем руками.
  useEffect(() => {
    if (subState !== null) return
    const retry = () => {
      if (document.visibilityState === 'visible') setReloadKey((k) => k + 1)
    }
    window.addEventListener('online', retry)
    document.addEventListener('visibilitychange', retry)
    return () => {
      window.removeEventListener('online', retry)
      document.removeEventListener('visibilitychange', retry)
    }
  }, [subState])

  // Редирект на paywall — в эффекте, а не в теле рендера (иначе "Cannot update
  // Router while rendering"). Тот же паттерн, что у auth-редиректа выше.
  useEffect(() => {
    if (subState && decideAccess(subState, pathname).action === 'redirect') {
      router.replace(PAYWALL_PATH)
    }
  }, [subState, pathname, router])

  if (!mounted || !isAuthenticated) return null

  // Ждём статус подписки — не мигаем содержимым.
  if (subState === null) {
    return (
      <div className="flex h-[100dvh] items-center justify-center bg-background">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-muted border-t-primary" />
      </div>
    )
  }

  // Навигацию выполняет эффект выше; здесь только не рендерим children,
  // пока идёт редирект (иначе мелькнёт защищённая страница).
  const decision = decideAccess(subState, pathname)
  if (decision.action === 'redirect') return null

  return (
    <div className="flex flex-col overflow-hidden" style={{ height: '100dvh' }}>
      {/* Мобильная шапка — только на телефоне (не показывается на /calendar: там своя шапка) */}
      {pathname !== '/calendar' && (
        <header className="md:hidden flex items-center gap-3 h-12 px-4 border-b bg-sidebar shrink-0">
          <button
            onClick={() => setSidebarOpen(true)}
            className="text-[var(--sidebar-text)] hover:text-foreground transition-colors"
            aria-label="Открыть меню"
          >
            <Menu className="h-5 w-5" />
          </button>
          <div className="h-6 w-6 rounded-md bg-primary flex items-center justify-center shrink-0">
            <GraduationCap className="h-3.5 w-3.5 text-primary-foreground" strokeWidth={2.5} />
          </div>
          <span className="font-heading text-sm font-bold tracking-tight">Amida</span>
        </header>
      )}

      <div className="flex flex-1 overflow-hidden min-h-0">
        <Sidebar mobileOpen={sidebarOpen} setMobileOpen={setSidebarOpen} />
        <main className={pathname === '/calendar' ? 'flex-1 overflow-hidden bg-background' : 'flex-1 overflow-y-auto bg-muted/20'}>
          <div
            key={pathname}
            className={pathname === '/calendar'
              ? 'h-full animate-in fade-in-0 duration-200'
              : 'px-4 md:px-8 py-4 md:py-3 animate-in fade-in-0 duration-200'
            }
          >
            {children}
          </div>
        </main>
      </div>

      <MobileBottomNav />

      {decision.banner && !bannerDismissed && (
        <div className="fixed bottom-4 left-4 z-50 max-w-xs rounded-lg border bg-card shadow-lg px-4 py-3 text-sm animate-in slide-in-from-bottom-2">
          <button
            onClick={() => setBannerDismissed(true)}
            className="absolute top-2 right-2 text-muted-foreground hover:text-foreground"
            aria-label="Закрыть"
          >
            <X className="h-4 w-4" />
          </button>
          <p className="pr-4 font-medium">Тариф истёк</p>
          <p className="pr-4 mt-0.5 text-muted-foreground">
            Оплатите — доступ скоро закроется.{' '}
            <Link href={PAYWALL_PATH} className="text-primary underline underline-offset-2">
              Перейти к оплате
            </Link>
          </p>
        </div>
      )}
    </div>
  )
}
//vercel suka