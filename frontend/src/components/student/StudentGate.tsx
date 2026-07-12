'use client'

import { useEffect, useState } from 'react'
import { usePathname, useRouter } from 'next/navigation'

// Клиентский гейт кабинета ученика: без tg_student_token уводит на логин,
// сохранив целевой путь в ?next=. Протухший токен чинит interceptor.
export function StudentGate({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const pathname = usePathname()
  const [ready, setReady] = useState(false)

  useEffect(() => {
    const token = localStorage.getItem('tg_student_token')
    if (!token) {
      router.replace(`/student/login?next=${encodeURIComponent(pathname)}`)
      return
    }
    setReady(true)
  }, [router, pathname])

  if (!ready) return null
  return <>{children}</>
}
