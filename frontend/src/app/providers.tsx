'use client'

import { QueryClientProvider } from '@tanstack/react-query'
import { queryClient } from '@/config/query-client'
import { Toaster } from '@/components/ui/sonner'

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      {children}
      <Toaster richColors closeButton />
    </QueryClientProvider>
  )
}
