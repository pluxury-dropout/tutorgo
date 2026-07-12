import { useQuery } from '@tanstack/react-query'
import { studentApi } from '@/lib/api/student'
import { useAuthStore } from '@/stores/auth'

// Единственный источник имени участника доски (SOT). Репетитор — из auth-store,
// гость (ученик из кабинета) — из studentApi.me(); аноним получает 401
// (retry:false) → имя остаётся undefined и синк показывает «Гость».
export function useBoardDisplayName(
  role: 'tutor' | 'guest'
): string | undefined {
  const tutor = useAuthStore((s) => s.user)
  const { data: student } = useQuery({
    queryKey: ['student', 'me'],
    queryFn: () => studentApi.me(),
    enabled: role === 'guest',
    retry: false,
  })
  const profile = role === 'tutor' ? tutor : student
  return (
    [profile?.first_name, profile?.last_name].filter(Boolean).join(' ') ||
    undefined
  )
}
