import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { tutorsApi, icsApi, TutorUpdateInput } from '@/lib/api/tutors'

export function useTutor(id: string) {
  return useQuery({
    queryKey: ['tutor', id],
    queryFn:  () => tutorsApi.get(id),
    enabled:  !!id,
  })
}

export function useUpdateTutor() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: TutorUpdateInput }) =>
      tutorsApi.update(id, data),
    onSuccess: (tutor) => {
      qc.setQueryData(['tutor', tutor.id], tutor)
    },
  })
}

// Токен нигде не кешируется React Query — GET-ручки «а есть ли ссылка» нет,
// состояние живёт локально на странице профиля (см. её комментарий).
export function useEnsureIcsLink() {
  return useMutation({ mutationFn: icsApi.ensureLink })
}

export function useRevokeIcsLink() {
  return useMutation({ mutationFn: icsApi.revokeLink })
}
