'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Menu, GraduationCap } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { Sidebar } from '@/components/layout/Sidebar'

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { token } = useAuthStore()
  const isAuthenticated = !!token
  const router = useRouter()
  const [mounted, setMounted] = useState(false)
  const [sidebarOpen, setSidebarOpen] = useState(false)

  useEffect(() => { setMounted(true) }, [])

  useEffect(() => {
    if (mounted && !isAuthenticated) router.replace('/login')
  }, [mounted, isAuthenticated, router])

  if (!mounted || !isAuthenticated) return null

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      {/* Мобильная шапка — только на телефоне */}
      <header className="md:hidden flex items-center gap-3 h-12 px-4 border-b bg-sidebar shrink-0">
        <button
          onClick={() => setSidebarOpen(true)}
          className="text-[var(--sidebar-text)] hover:text-foreground transition-colors"
          aria-label="Открыть меню"
        >
          <Menu className="h-5 w-5" />
        </button>
        <GraduationCap className="h-[16px] w-[16px] text-primary" strokeWidth={2} />
        <span className="font-semibold text-sm tracking-tight">TutorGo</span>
      </header>

      <div className="flex flex-1 overflow-hidden">
        <Sidebar mobileOpen={sidebarOpen} setMobileOpen={setSidebarOpen} />
        <main className="flex-1 overflow-y-auto bg-muted/20">
          <div className="px-4 md:px-8 py-4 md:py-3">{children}</div>
        </main>
      </div>
    </div>
  )
}
//vercel suka