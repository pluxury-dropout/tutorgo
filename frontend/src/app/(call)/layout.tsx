'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/stores/auth'

export default function CallLayout({ children }: { children: React.ReactNode }) {
  const { token } = useAuthStore()
  const isAuthenticated = !!token
  const router = useRouter()
  const [mounted, setMounted] = useState(false)

  useEffect(() => { setMounted(true) }, [])

  useEffect(() => {
    if (mounted && !isAuthenticated) router.replace('/login')
  }, [mounted, isAuthenticated, router])

  if (!mounted || !isAuthenticated) return null

  return (
    <div className="h-screen overflow-hidden">
      {children}
    </div>
  )
}
