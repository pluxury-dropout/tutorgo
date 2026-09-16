'use client'

import { useState, useEffect, useMemo } from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import {
  LayoutDashboard,
  CalendarDays,
  Users,
  CreditCard,
  User,
  Video,
  GraduationCap,
  Sun,
  Moon,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react'
import { useTheme } from 'next-themes'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/lib/api/auth'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { useCalendarFeed } from '@/lib/hooks/useCalendar'
import { KIND_COLORS } from '@/lib/eventKind'
import { pickActiveLesson } from './nextLesson'
import type { LessonStatus, CalendarItem } from '@/types/api'

// ─── helpers ──────────────────────────────────────────────────────────────────

const MONTHS_RU = [
  'Январь','Февраль','Март','Апрель','Май','Июнь',
  'Июль','Август','Сентябрь','Октябрь','Ноябрь','Декабрь',
]
const DOWS_SHORT = ['Пн','Вт','Ср','Чт','Пт','Сб','Вс']

const STATUS_DOT: Record<LessonStatus, string> = {
  scheduled: 'var(--cal-scheduled-text)',
  completed: 'var(--cal-completed-text)',
  cancelled: 'var(--cal-cancelled-text)',
  missed:    'var(--cal-missed-text)',
}

type BusyItem = Extract<CalendarItem, { type: 'lesson' | 'event' }>

/** Занятость: уроки и события. Задачи из ленты отбрасываем — «сделать
 *  когда-нибудь» не занимает время (см. спеку календаря, раздел 2).
 *  Предикат в filter обязателен: без него тип остаётся полным union,
 *  и сужение по it.type ниже не работает. */
function busyOnly(items: CalendarItem[]): BusyItem[] {
  return items.filter((it): it is BusyItem => it.type !== 'task')
}

function getWeekStart(d: Date): Date {
  const day  = d.getDay()
  const diff = day === 0 ? -6 : 1 - day
  const mon  = new Date(d)
  mon.setDate(d.getDate() + diff)
  mon.setHours(0, 0, 0, 0)
  return mon
}

function isSameLocalDay(iso: string, ref: Date): boolean {
  const d = new Date(iso)
  return d.getFullYear() === ref.getFullYear()
      && d.getMonth()    === ref.getMonth()
      && d.getDate()     === ref.getDate()
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
}

// ─── MiniCalendar ─────────────────────────────────────────────────────────────

function toDateKey(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function MiniCalendar({
  displayedDates,
  onNavigate,
}: {
  displayedDates: { start: Date; end: Date }
  onNavigate: (date: Date) => void
}) {
  const today = useMemo(() => new Date(), [])

  // Листание стрелками живёт здесь; синхронизация с основным календарём — через
  // key на месте вызова, поэтому смена его месяца просто монтирует нас заново.
  const [miniMonth, setMiniMonth] = useState(
    () => new Date(displayedDates.start.getFullYear(), displayedDates.start.getMonth(), 1)
  )

  const year      = miniMonth.getFullYear()
  const month     = miniMonth.getMonth()
  const firstDay  = new Date(year, month, 1)
  const totalDays = new Date(year, month + 1, 0).getDate()
  const offset    = (firstDay.getDay() + 6) % 7

  const monthFrom = useMemo(() => new Date(year, month, 1).toISOString(), [year, month])
  const monthTo   = useMemo(() => new Date(year, month + 1, 1).toISOString(), [year, month])
  const { data: monthItems = [] } = useCalendarFeed(monthFrom, monthTo)

  // Точка значит «день занят», поэтому её ставят и урок, и событие: врач и
  // спортзал — такая же занятость, иначе по сетке нельзя планировать. Задачи
  // не в счёт: у задачи слот условный и двигается свободно.
  const busyDays = useMemo(() => {
    const s = new Set<string>()
    busyOnly(monthItems).forEach(it => s.add(toDateKey(new Date(it.starts_at))))
    return s
  }, [monthItems])

  const cells: (number | null)[] = Array(offset).fill(null)
  for (let d = 1; d <= totalDays; d++) cells.push(d)

  const isInRange = (d: number) => {
    const date = new Date(year, month, d)
    return date >= displayedDates.start && date < displayedDates.end
  }
  const isToday = (d: number) =>
    year === today.getFullYear() && month === today.getMonth() && d === today.getDate()

  return (
    <div className="p-3 select-none shrink-0">
      <div className="flex items-center justify-between mb-1.5">
        <button
          onClick={() => setMiniMonth(new Date(year, month - 1, 1))}
          className="h-6 w-6 flex items-center justify-center rounded hover:bg-[var(--sidebar-hover-bg)] text-[var(--sidebar-text)] transition-colors"
        >
          <ChevronLeft className="h-3.5 w-3.5" />
        </button>
        <span className="text-[11.5px] font-semibold text-foreground">
          {MONTHS_RU[month]} {year}
        </span>
        <button
          onClick={() => setMiniMonth(new Date(year, month + 1, 1))}
          className="h-6 w-6 flex items-center justify-center rounded hover:bg-[var(--sidebar-hover-bg)] text-[var(--sidebar-text)] transition-colors"
        >
          <ChevronRight className="h-3.5 w-3.5" />
        </button>
      </div>

      <div className="grid grid-cols-7">
        {DOWS_SHORT.map(d => (
          <span key={d} className="text-center text-[9.5px] font-medium text-[var(--sidebar-text)] py-0.5">
            {d}
          </span>
        ))}
      </div>

      <div className="grid grid-cols-7 gap-y-0.5">
        {cells.map((d, i) => {
          const hasDot = d ? busyDays.has(toDateKey(new Date(year, month, d))) : false
          const todayCell = d ? isToday(d) : false
          return (
            <button
              key={i}
              onClick={() => d && onNavigate(new Date(year, month, d))}
              disabled={!d}
              className={cn(
                'h-[24px] relative flex items-center justify-center rounded text-[11px] transition-colors',
                !d && 'invisible',
                todayCell
                  ? 'bg-foreground text-background font-semibold'
                  : d && isInRange(d)
                  ? 'bg-[var(--sidebar-hover-bg)] text-foreground font-medium'
                  : d
                  ? 'text-[var(--sidebar-text)] hover:bg-[var(--sidebar-hover-bg)] hover:text-foreground cursor-pointer'
                  : '',
              )}
            >
              {d}
              {hasDot && (
                <span
                  className="absolute bottom-[2px] left-1/2 -translate-x-1/2 w-[3px] h-[3px] rounded-full"
                  style={{ background: todayCell ? 'var(--background)' : 'var(--muted-foreground)' }}
                />
              )}
            </button>
          )
        })}
      </div>
    </div>
  )
}

// ─── TodayList ────────────────────────────────────────────────────────────────

/** Строка списка «Сегодня» — одна разметка на урок и на событие; различия
 *  вычисляет вызывающий, где тип уже сужен. */
function BusyRow({ startsAt, accent, title, subtitle, struck }: {
  startsAt: string
  accent:   string
  title:    string
  subtitle: string
  struck?:  boolean
}) {
  return (
    <div className="flex gap-2 px-2 py-1.5 rounded-md mx-1 hover:bg-[var(--sidebar-hover-bg)] cursor-pointer transition-colors items-start">
      <div
        className="w-[3px] self-stretch rounded-full shrink-0 mt-[3px]"
        style={{ background: accent }}
      />
      <div className="min-w-0">
        <div className="text-[10px] text-[var(--sidebar-text)] tabular-nums leading-tight">
          {formatTime(startsAt)}
        </div>
        <div className={cn(
          'text-[12px] font-medium leading-snug truncate',
          struck ? 'line-through text-[var(--sidebar-text)]' : 'text-foreground',
        )}>
          {title}
        </div>
        {/* У события место указано не всегда — тогда второй строки нет. */}
        {subtitle && (
          <div className="text-[11px] text-[var(--sidebar-text)] truncate leading-tight">
            {subtitle}
          </div>
        )}
      </div>
    </div>
  )
}

function TodayList({ items }: { items: CalendarItem[] }) {
  const today = useMemo(() => new Date(), [])

  // Уроки и события в одном списке: это ответ на вопрос «чем занят день»,
  // а не «сколько у меня уроков».
  const todayItems = useMemo(
    () => busyOnly(items)
      .filter(it => isSameLocalDay(it.starts_at, today))
      .sort((a, b) => a.starts_at.localeCompare(b.starts_at)),
    [items, today],
  )

  const label = today.toLocaleDateString('ru-RU', { weekday: 'short', day: 'numeric', month: 'short' })

  return (
    <div className="flex flex-col min-h-0 flex-1 overflow-hidden">
      <div className="px-3 pt-2.5 pb-1 flex items-center gap-1.5">
        <span className="text-[10px] font-semibold uppercase tracking-wider text-[var(--sidebar-text)]">
          Сегодня · {label}
        </span>
        {todayItems.length > 0 && (
          <span className="text-[10px] text-[var(--sidebar-text)]/60">{todayItems.length}</span>
        )}
      </div>

      <div className="overflow-y-auto flex-1 pb-2">
        {todayItems.length === 0 ? (
          <p className="px-3 py-1 text-[11.5px] text-[var(--sidebar-text)]/60">День свободен</p>
        ) : (
          todayItems.map(it => it.type === 'lesson' ? (
            <BusyRow
              key={`lesson:${it.id}`}
              startsAt={it.starts_at}
              // Полоска слева у урока — его статус, у события — вид.
              accent={STATUS_DOT[it.lesson.status]}
              title={it.lesson.subject}
              subtitle={it.lesson.is_group ? 'Групповой урок' : (it.lesson.student_name ?? '—')}
              struck={it.lesson.status === 'cancelled'}
            />
          ) : (
            <BusyRow
              key={`event:${it.id}`}
              startsAt={it.starts_at}
              accent={it.event.color || KIND_COLORS[it.event.kind].border}
              title={it.title}
              subtitle={it.event.location}
            />
          ))
        )}
      </div>
    </div>
  )
}

// ─── CalendarSidebarPanel ─────────────────────────────────────────────────────
// Отдельный компонент — чтобы хуки вызывались безусловно (Rules of Hooks)

function CalendarSidebarPanel() {

  const todayRange = useMemo(() => {
    const n     = new Date()
    const start = new Date(n.getFullYear(), n.getMonth(), n.getDate())
    const end   = new Date(start)
    // +2 суток: список «Сегодня» фильтрует сам, а кнопке нужен урок,
    // который может начаться уже после полуночи.
    end.setDate(end.getDate() + 2)
    return { from: start.toISOString(), to: end.toISOString() }
  }, [])

  const { data: todayItems = [] } = useCalendarFeed(todayRange.from, todayRange.to)

  // Тикер: без него кнопка не появится сама на уже открытой вкладке.
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 30_000)
    return () => clearInterval(t)
  }, [])

  // Кнопка «Начать урок» — только про уроки, событию начинать нечего. Время
  // берём с верхнего уровня ленты: оптимистичный патч переноса правит его,
  // а не вложенный урок.
  const activeLesson = useMemo(
    () => pickActiveLesson(
      todayItems.flatMap(it => it.type === 'lesson'
        ? [{ ...it.lesson, scheduled_at: it.starts_at, duration_minutes: it.duration_minutes }]
        : []),
      now,
    ),
    [todayItems, now],
  )
  const started = activeLesson
    ? now.getTime() >= new Date(activeLesson.scheduled_at).getTime()
    : false

  function handleStartLesson() {
    if (!activeLesson) return
    window.open(`/lessons/${activeLesson.id}/call`, '_blank')
  }

  const [displayedDates, setDisplayedDates] = useState<{ start: Date; end: Date }>(() => {
    const n     = new Date()
    const start = getWeekStart(n)
    const end   = new Date(start)
    end.setDate(start.getDate() + 7)
    return { start, end }
  })

  useEffect(() => {
    const handler = (e: Event) => {
      const ce = e as CustomEvent<{ start: string; end: string }>
      setDisplayedDates({ start: new Date(ce.detail.start), end: new Date(ce.detail.end) })
    }
    window.addEventListener('fc:datesSet', handler)
    return () => window.removeEventListener('fc:datesSet', handler)
  }, [])

  function handleNavigate(date: Date) {
    window.dispatchEvent(new CustomEvent('fc:goto', { detail: date.toISOString() }))
  }

  return (
    <div className="flex-1 overflow-hidden flex flex-col min-h-0 gap-3 px-3 pt-1 pb-3">
      <div className="rounded-[12px] border border-border bg-card shrink-0 overflow-hidden">
        <MiniCalendar
          key={`${displayedDates.start.getFullYear()}-${displayedDates.start.getMonth()}`}
          displayedDates={displayedDates}
          onNavigate={handleNavigate}
        />
      </div>
      <div className="rounded-[12px] border border-border bg-card flex flex-col min-h-0 overflow-hidden">
        <TodayList items={todayItems} />
      </div>
      <div className="flex-1 min-h-0" />
      {activeLesson && (
        <>
          <style>{`
            @keyframes liveDot {
              0%,100% { opacity:1; box-shadow: 0 0 0 0 rgba(34,197,94,0.5); }
              50% { opacity:.75; box-shadow: 0 0 0 4px rgba(34,197,94,0); }
            }
          `}</style>
          <div className="shrink-0 rounded-[12px] border border-border bg-card p-2.5">
            <div className="text-[12px] font-semibold text-foreground truncate leading-tight">
              {activeLesson.subject}
            </div>
            <div className="text-[11px] text-[var(--sidebar-text)] tabular-nums mb-2">
              {formatTime(activeLesson.scheduled_at)}
            </div>
            <button
              onClick={handleStartLesson}
              className="w-full flex items-center justify-center gap-2 px-3 py-2 rounded-[10px] transition-colors hover:brightness-110"
              style={{ background: 'var(--secondary)' }}
            >
              <span
                style={{
                  width: 7, height: 7, borderRadius: '50%', flexShrink: 0,
                  background: '#2D9964',
                  animation: 'liveDot 2s ease-in-out infinite',
                }}
              />
              <span style={{ fontSize: 13, color: 'var(--foreground)', fontWeight: 600 }}>
                {started ? 'Присоединиться' : 'Начать урок'}
              </span>
            </button>
          </div>
        </>
      )}
    </div>
  )
}

// ─── NAV ──────────────────────────────────────────────────────────────────────

const NAV = [
  { href: '/dashboard',  label: 'Главная',      icon: LayoutDashboard },
  { href: '/calendar',   label: 'Расписание',   icon: CalendarDays },
  { href: '/students',   label: 'Ученики',      icon: Users },
  { href: '/groups',     label: 'Группы',       icon: Users },
  { href: '/trial',      label: 'Пробный урок', icon: Video },
  { href: '/payments',   label: 'Платежи',      icon: CreditCard },
  { href: '/profile',    label: 'Профиль',      icon: User },
]

function initials(firstName?: string, lastName?: string) {
  return `${(firstName?.[0] ?? '').toUpperCase()}${(lastName?.[0] ?? '').toUpperCase()}`
}

// ─── SidebarInner ─────────────────────────────────────────────────────────────

function SidebarInner() {
  const pathname = usePathname()
  const { user, clearAuth } = useAuthStore()
  const { resolvedTheme, setTheme } = useTheme()

  async function handleLogout() {
    await authApi.logout()
    clearAuth()
  }

  return (
    <>
      <div className="flex items-center gap-2.5 px-5 py-5 border-b border-border shrink-0">
        <div className="h-7 w-7 rounded-lg bg-primary flex items-center justify-center shrink-0">
          <GraduationCap className="h-4 w-4 text-primary-foreground" strokeWidth={2.5} />
        </div>
        <span className="font-heading text-[15px] font-bold tracking-tight">Amida</span>
      </div>

      <nav className="px-3 py-4 space-y-0.5 shrink-0">
        {NAV.map(({ href, label, icon: Icon }) => {
          const active =
            pathname === href || (href !== '/dashboard' && pathname.startsWith(href + '/'))
          return (
            <Link
              key={href}
              href={href}
              className={cn(
                'flex items-center gap-2.5 px-3 py-1.5 rounded-md text-xs transition-colors',
                active
                  ? 'bg-[var(--sidebar-active-bg)] text-[var(--sidebar-active-text)] font-semibold'
                  : 'text-[var(--sidebar-text)] hover:bg-[var(--sidebar-hover-bg)] hover:text-foreground',
              )}
            >
              <Icon className="h-[15px] w-[15px] shrink-0" strokeWidth={2} />
              {label}
            </Link>
          )
        })}
      </nav>

      <CalendarSidebarPanel />

      {user && (
        <div className="px-4 py-4 border-t border-border flex items-center gap-3 shrink-0">
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
              onClick={handleLogout}
              className="text-[11px] text-[var(--sidebar-text)] hover:text-foreground transition-colors"
            >
              Выйти
            </button>
          </div>
          <button
            onClick={() => setTheme(resolvedTheme === 'dark' ? 'light' : 'dark')}
            className="h-7 w-7 flex items-center justify-center rounded-md text-[var(--sidebar-text)] hover:bg-[var(--sidebar-hover-bg)] hover:text-foreground transition-colors shrink-0"
          >
            {resolvedTheme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </button>
        </div>
      )}
    </>
  )
}

// ─── Sidebar ──────────────────────────────────────────────────────────────────

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
      <aside className="hidden md:flex flex-col w-60 shrink-0 border-r bg-sidebar h-full">
        <SidebarInner />
      </aside>

      <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
        <SheetContent side="left" className="w-60 p-0 bg-sidebar flex flex-col" showCloseButton={false}>
          <SidebarInner />
        </SheetContent>
      </Sheet>
    </>
  )
}
