import { useQuery, useMutation, useQueryClient, keepPreviousData } from '@tanstack/react-query'
import { tasksApi, TaskInput, TaskUpdateInput } from '@/lib/api/tasks'
import { Task } from '@/types/api'

const BOARD_KEY = ['tasks', 'board'] as const

export function useTasks(from: string, to: string) {
  return useQuery({
    queryKey:        ['tasks', from, to],
    queryFn:         () => tasksApi.list(from, to),
    enabled:         !!from && !!to,
    placeholderData: keepPreviousData,
  })
}

export function useBoardTasks() {
  return useQuery({
    queryKey: ['tasks', 'board'],
    queryFn:  () => tasksApi.board(),
  })
}

export function useCreateTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: TaskInput) => tasksApi.create(data),
    onSuccess:  () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}

export function useRescheduleTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: TaskUpdateInput }) => tasksApi.update(id, data),
    // Optimistic: сразу применяем изменения к доске, откатываем при ошибке.
    onMutate: async ({ id, data }) => {
      await qc.cancelQueries({ queryKey: BOARD_KEY })
      const prev = qc.getQueryData<Task[]>(BOARD_KEY)
      if (prev) {
        qc.setQueryData<Task[]>(BOARD_KEY, prev.map(t => (t.id === id ? ({ ...t, ...data } as Task) : t)))
      }
      return { prev }
    },
    onError:   (_e, _v, ctx) => { if (ctx?.prev) qc.setQueryData(BOARD_KEY, ctx.prev) },
    onSettled: () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}

export function useDeleteTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => tasksApi.delete(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: BOARD_KEY })
      const prev = qc.getQueryData<Task[]>(BOARD_KEY)
      if (prev) {
        qc.setQueryData<Task[]>(BOARD_KEY, prev.filter(t => t.id !== id))
      }
      return { prev }
    },
    onError:   (_e, _v, ctx) => { if (ctx?.prev) qc.setQueryData(BOARD_KEY, ctx.prev) },
    onSettled: () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}
