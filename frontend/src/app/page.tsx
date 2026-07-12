'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'

// Ученик и препод делят один корневой URL. Есть ученический токен — уводим
// в кабинет ученика, иначе на преподский дашборд (тот сам гейтит на /login).
// ponytail: наличие токена = «это ученик»; протухший починит StudentGate/refresh.
export default function RootPage() {
  const router = useRouter()
  useEffect(() => {
    const isStudent = localStorage.getItem('tg_student_token')
    router.replace(isStudent ? '/student/lessons' : '/dashboard')
  }, [router])
  return null
}
