'use client'

import { useEffect, useState } from 'react'
import { useRouter, usePathname } from 'next/navigation'
import { Menu, GraduationCap } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { Sidebar } from '@/components/layout/Sidebar'

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { token } = useAuthStore()
  const isAuthenticated = !!token
  const router = useRouter()
  const pathname = usePathname()
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
        <div className="h-6 w-6 rounded-md bg-primary flex items-center justify-center shrink-0">
          <GraduationCap className="h-3.5 w-3.5 text-primary-foreground" strokeWidth={2.5} />
        </div>
        <span className="font-heading text-sm font-bold tracking-tight">TutorGo</span>
      </header>

      <div className="flex flex-1 overflow-hidden">
        <Sidebar mobileOpen={sidebarOpen} setMobileOpen={setSidebarOpen} />
        <main className="flex-1 overflow-y-auto bg-muted/20">
          <div key={pathname} className="px-4 md:px-8 py-4 md:py-3 animate-in fade-in-0 duration-200">
            {children}
          </div>
        </main>
      </div>
    </div>
  )
}
//vercel suka