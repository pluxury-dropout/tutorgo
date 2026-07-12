'use client'

import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { GraduationCap, LogOut } from 'lucide-react'

import { StudentGate } from '@/components/student/StudentGate'
import { useStudentAuthStore } from '@/stores/studentAuth'
import { studentApi } from '@/lib/api/student'
import { Button } from '@/components/ui/button'

export default function StudentCabinetLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const user = useStudentAuthStore((s) => s.user)
  const clearAuth = useStudentAuthStore((s) => s.clearAuth)

  async function handleLogout() {
    await studentApi.logout()
    clearAuth()
    router.replace('/student/login')
  }

  return (
    <StudentGate>
      <div className="min-h-screen bg-background">
        <header style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="mx-auto max-w-3xl px-4 h-14 flex items-center justify-between">
            <Link href="/student/lessons" className="flex items-center gap-2 font-semibold">
              <GraduationCap className="h-5 w-5 text-primary" />
              TutorHub
            </Link>
            <div className="flex items-center gap-3">
              <Link
                href="/student/profile"
                className="text-sm text-muted-foreground hover:text-foreground"
              >
                {user ? `${user.first_name} ${user.last_name}`.trim() : 'Профиль'}
              </Link>
              <Button size="icon" variant="ghost" className="h-8 w-8" onClick={handleLogout} title="Выйти">
                <LogOut className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </header>
        <main className="mx-auto max-w-3xl px-4 py-6">{children}</main>
      </div>
    </StudentGate>
  )
}
