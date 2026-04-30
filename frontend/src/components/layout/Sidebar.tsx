'use client'

import { useEffect } from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import {
  LayoutDashboard,
  CalendarDays,
  Users,
  BookOpen,
  CreditCard,
  User,
  GraduationCap,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth'
import { Sheet, SheetContent } from '@/components/ui/sheet'

const NAV = [
  { href: '/dashboard',  label: 'Главная',    icon: LayoutDashboard },
  { href: '/calendar',   label: 'Расписание', icon: CalendarDays },
  { href: '/students',   label: 'Ученики',    icon: Users },
  { href: '/courses',    label: 'Курсы',      icon: BookOpen },
  { href: '/payments',   label: 'Платежи',    icon: CreditCard },
  { href: '/profile',    label: 'Профиль',    icon: User },
]

function initials(firstName?: string, lastName?: string) {
  return `${(firstName?.[0] ?? '').toUpperCase()}${(lastName?.[0] ?? '').toUpperCase()}`
}

function SidebarInner() {
  const pathname = usePathname()
  const { user, clearAuth } = useAuthStore()

  return (
    <>
      <div className="flex items-center gap-2 px-5 py-5 border-b border-border">
        <GraduationCap className="h-[17px] w-[17px] text-primary" strokeWidth={2} />
        <span className="font-semibold text-sm tracking-tight">TutorGo</span>
      </div>

      <nav className="flex-1 px-3 py-4 space-y-0.5">
        {NAV.map(({ href, label, icon: Icon }) => {
          const active = pathname === href || (href !== '/dashboard' && pathname.startsWith(href + '/'))
          return (
            <Link
              key={href}
              href={href}
              className={cn(
                'flex items-center gap-3 px-3 py-[9px] rounded-md text-sm transition-colors',
                active
                  ? 'bg-[var(--sidebar-active-bg)] text-[var(--sidebar-active-text)] font-semibold'
                  : 'text-[var(--sidebar-text)] hover:bg-[var(--sidebar-hover-bg)] hover:text-foreground',
              )}
            >
              <Icon className="h-[17px] w-[17px] shrink-0" strokeWidth={2} />
              {label}
            </Link>
          )
        })}
      </nav>

      {user && (
        <div className="px-4 py-4 border-t border-border flex items-center gap-3">
          <div
            className="h-[34px] w-[34px] shrink-0 rounded-full flex items-center justify-center text-xs font-bold text-primary"
            style={{ background: 'var(--primary-light)' }}
          >
            {initials(user.first_name, user.last_name)}
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-xs font-semibold truncate text-foreground">
              {user.first_name} {user.last_name}
            </p>
            <button
              onClick={clearAuth}
              className="text-[11px] text-[var(--sidebar-text)] hover:text-foreground transition-colors"
            >
              Выйти
            </button>
          </div>
        </div>
      )}
    </>
  )
}

interface SidebarProps {
  mobileOpen:    boolean
  setMobileOpen: (open: boolean) => void
}

export function Sidebar({ mobileOpen, setMobileOpen }: SidebarProps) {
  const pathname = usePathname()

  useEffect(() => {
    setMobileOpen(false)
  }, [pathname, setMobileOpen])

  return (
    <>
      {/* Desktop: всегда виден */}
      <aside className="hidden md:flex flex-col w-60 shrink-0 border-r bg-sidebar h-full">
        <SidebarInner />
      </aside>

      {/* Mobile: Sheet-drawer */}
      <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
        <SheetContent side="left" className="w-60 p-0 bg-sidebar flex flex-col" showCloseButton={false}>
          <SidebarInner />
        </SheetContent>
      </Sheet>
    </>
  )
}
