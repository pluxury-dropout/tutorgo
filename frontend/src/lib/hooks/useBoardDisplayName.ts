import { useQuery } from '@tanstack/react-query'
import { studentApi } from '@/lib/api/student'
import { useAuthStore } from '@/stores/auth'

export interface BoardIdentity {
  /** Отображаемое имя; undefined → синк покажет «Гость». */
  name?: string
  /** Стабильный id человека. У анонима по ссылке-приглашению его нет. */
  uid?: string
}

// Единственный источник личности участника доски (SOT). Репетитор — из
// auth-store, гость (ученик из кабинета) — из studentApi.me(); аноним получает
// 401 (retry:false) → и имя, и uid остаются undefined.
//
// uid схлопывает несколько соединений одного человека (вкладка звонка + вкладка
// доски) в одного участника: см. pushCollaborators в useExcalidrawSync.
export function useBoardDisplayName(role: 'tutor' | 'guest'): BoardIdentity {
  const tutor = useAuthStore((s) => s.user)
  const { data: student } = useQuery({
    queryKey: ['student', 'me'],
    queryFn: () => studentApi.me(),
    enabled: role === 'guest',
    retry: false,
  })
  const profile = role === 'tutor' ? tutor : student
  return {
    name:
      [profile?.first_name, profile?.last_name].filter(Boolean).join(' ') ||
      undefined,
    uid: profile?.id,
  }
}
