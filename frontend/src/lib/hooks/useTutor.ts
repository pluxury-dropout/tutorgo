import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { tutorsApi, TutorUpdateInput } from '@/lib/api/tutors'

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
