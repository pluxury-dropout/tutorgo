'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { LayoutDashboard, CalendarDays, Users, CreditCard, User } from 'lucide-react'

const NAV_TABS = [
  { href: '/dashboard', label: 'Главная',    Icon: LayoutDashboard },
  { href: '/calendar',  label: 'Расписание', Icon: CalendarDays    },
  { href: '/students',  label: 'Ученики',    Icon: Users           },
  { href: '/payments',  label: 'Платежи',    Icon: CreditCard      },
  { href: '/profile',   label: 'Профиль',    Icon: User            },
]

export function MobileBottomNav() {
  const pathname = usePathname()

  return (
    <nav
      className="md:hidden flex items-stretch justify-around border-t bg-background shrink-0"
      style={{ paddingBottom: 'env(safe-area-inset-bottom, 16px)' }}
    >
      {NAV_TABS.map(({ href, label, Icon }) => {
        const active =
          pathname === href || (href !== '/dashboard' && pathname.startsWith(href + '/'))
        return (
          <Link
            key={href}
            href={href}
            className="flex-1 flex flex-col items-center gap-[3px] py-[6px] px-0 no-underline"
            style={{ color: active ? 'var(--foreground)' : 'var(--muted-foreground)' }}
          >
            <Icon size={19} strokeWidth={2} />
            <span style={{ fontSize: 11, fontWeight: active ? 600 : 500, lineHeight: 1 }}>
              {label}
            </span>
          </Link>
        )
      })}
    </nav>
  )
}
