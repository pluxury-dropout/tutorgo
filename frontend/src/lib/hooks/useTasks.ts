import { useQuery, useMutation, useQueryClient, keepPreviousData } from '@tanstack/react-query'
import { tasksApi, TaskInput, TaskUpdateInput } from '@/lib/api/tasks'
import { Task } from '@/types/api'

const BOARD_KEY = ['tasks', 'board'] as const

// Задача видна в двух местах: канбан (['tasks']) и лента календаря (['calendar']).
function invalidateTaskViews(qc: ReturnType<typeof useQueryClient>) {
  qc.invalidateQueries({ queryKey: ['tasks'] })
  qc.invalidateQueries({ queryKey: ['calendar'] })
}

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
    // Optimistic: сразу добавляем задачу во все кэши tasks (доска + диапазоны календаря),
    // временный id заменится настоящим при инвалидации в onSettled.
    onMutate: async (data) => {
      await qc.cancelQueries({ queryKey: ['tasks'] })
      const prev = qc.getQueriesData<Task[]>({ queryKey: ['tasks'] })
      const optimistic: Task = {
        id:               `tmp-${Date.now()}`,
        tutor_id:         '',
        title:            data.title,
        scheduled_at:     data.scheduled_at ?? null,
        duration_minutes: data.duration_minutes ?? null,
        status:           (data.status as Task['status']) ?? 'not_urgent',
        created_at:       new Date().toISOString(),
      }
      qc.setQueriesData<Task[]>({ queryKey: ['tasks'] }, old => (old ? [...old, optimistic] : old))
      return { prev }
    },
    onError:   (_e, _v, ctx) => ctx?.prev.forEach(([key, data]) => qc.setQueryData(key, data)),
    onSettled: () => invalidateTaskViews(qc),
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
    onSettled: () => invalidateTaskViews(qc),
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
    onSettled: () => invalidateTaskViews(qc),
  })
}
