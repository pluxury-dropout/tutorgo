'use client'

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

const NAV = [
  { href: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { href: '/calendar', label: 'Расписание', icon: CalendarDays },
  { href: '/students', label: 'Ученики', icon: Users },
  { href: '/courses', label: 'Курсы', icon: BookOpen },
  { href: '/payments', label: 'Платежи', icon: CreditCard },
  { href: '/profile', label: 'Профиль', icon: User },
]

export function Sidebar() {
  const pathname = usePathname()
  const { user, clearAuth } = useAuthStore()

  return (
    <aside className="w-60 shrink-0 flex flex-col border-r bg-card h-full">
      <div className="flex items-center gap-2 px-5 py-5 border-b">
        <GraduationCap className="h-5 w-5 text-primary" />
        <span className="font-semibold text-sm tracking-tight">TutorGo</span>
      </div>

      <nav className="flex-1 px-3 py-4 space-y-0.5">
        {NAV.map(({ href, label, icon: Icon }) => {
          const active = pathname === href || pathname.startsWith(href + '/')
          return (
            <Link
              key={href}
              href={href}
              className={cn(
                'flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors',
                active
                  ? 'bg-primary/10 text-primary font-medium border-l-2 border-primary pl-[10px]'
                  : 'text-muted-foreground hover:bg-muted hover:text-foreground',
              )}
            >
              <Icon className="h-4 w-4 shrink-0" />
              {label}
            </Link>
          )
        })}
      </nav>

      {user && (
        <div className="px-5 py-4 border-t">
          <p className="text-xs font-medium truncate">
            {user.first_name} {user.last_name}
          </p>
          <button
            onClick={clearAuth}
            className="text-xs text-muted-foreground hover:text-foreground mt-1"
          >
            Выйти
          </button>
        </div>
      )}
    </aside>
  )
}
