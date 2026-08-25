'use client'

import Link from 'next/link'
import { GraduationCap } from 'lucide-react'

import { StudentGate } from '@/components/student/StudentGate'
import { NextLessonActions } from '@/components/student/NextLessonActions'
import { useStudentAuthStore } from '@/stores/studentAuth'

export default function StudentCabinetLayout({ children }: { children: React.ReactNode }) {
  const user = useStudentAuthStore((s) => s.user)

  return (
    <StudentGate>
      <div className="min-h-[100dvh] bg-background">
        <header style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="mx-auto max-w-5xl px-4 h-14 flex items-center justify-between">
            <Link href="/student/lessons" className="flex items-center gap-2 font-semibold">
              <GraduationCap className="h-5 w-5 text-primary" />
              Amida
            </Link>
            <div className="flex items-center gap-2 sm:gap-3 min-w-0">
              <NextLessonActions />
              <Link
                href="/student/profile"
                className="text-sm text-muted-foreground hover:text-foreground truncate max-w-[6.5rem] sm:max-w-none"
              >
                {user ? `${user.first_name} ${user.last_name}`.trim() : 'Профиль'}
              </Link>
            </div>
          </div>
        </header>
        <main className="mx-auto max-w-5xl px-4 py-6">{children}</main>
      </div>
    </StudentGate>
  )
}
