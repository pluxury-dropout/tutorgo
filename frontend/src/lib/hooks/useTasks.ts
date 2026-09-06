import { useQuery, useMutation, useQueryClient, keepPreviousData } from '@tanstack/react-query'
import { tasksApi, TaskInput, TaskUpdateInput } from '@/lib/api/tasks'
import {
  patchCalendarEntry, insertFeedEntry, removeCalendarEntries, rollbackCalendar, tempId,
} from '@/lib/hooks/useCalendar'
import { Task } from '@/types/api'

const BOARD_KEY = ['tasks', 'board'] as const

// Столько сервер даёт задаче без длительности (defaultTaskMinutes в
// service/calendar.go) — оптимистичный патч ленты должен совпадать, иначе блок
// на секунду сменит высоту.
const DEFAULT_TASK_MINUTES = 30

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
        id:               tempId(),
        tutor_id:         '',
        title:            data.title,
        scheduled_at:     data.scheduled_at ?? null,
        duration_minutes: data.duration_minutes ?? null,
        status:           (data.status as Task['status']) ?? 'not_urgent',
        created_at:       new Date().toISOString(),
      }
      qc.setQueriesData<Task[]>({ queryKey: ['tasks'] }, old => (old ? [...old, optimistic] : old))
      // Со слотом задача живёт ещё и в ленте календаря — она отдельный запрос.
      const previousEntries = data.scheduled_at
        ? await insertFeedEntry(qc, {
            id:               optimistic.id,
            type:             'task',
            title:            optimistic.title,
            starts_at:        data.scheduled_at,
            duration_minutes: data.duration_minutes ?? DEFAULT_TASK_MINUTES,
            task:             { ...optimistic },
          })
        : undefined
      return { prev, previousEntries }
    },
    onError: (_e, _v, ctx) => {
      ctx?.prev.forEach(([key, data]) => qc.setQueryData(key, data))
      rollbackCalendar(qc, ctx?.previousEntries)
    },
    onSettled: () => invalidateTaskViews(qc),
  })
}

export function useRescheduleTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: TaskUpdateInput }) => tasksApi.update(id, data),
    // Optimistic: сразу применяем изменения к доске и к ленте календаря —
    // задачу тащат в обоих календарях, и без патча ленты блок отскакивает.
    onMutate: async ({ id, data }) => {
      await qc.cancelQueries({ queryKey: BOARD_KEY })
      const prev = qc.getQueryData<Task[]>(BOARD_KEY)
      if (prev) {
        qc.setQueryData<Task[]>(BOARD_KEY, prev.map(t => (t.id === id ? ({ ...t, ...data } as Task) : t)))
      }
      // Слот у задачи может быть снят (scheduled_at: null) — тогда из ленты она
      // просто уйдёт по инвалидации, оптимистично двигать нечего.
      const previousEntries = data.scheduled_at
        ? await patchCalendarEntry(
            qc, id,
            {
              starts_at:        data.scheduled_at,
              duration_minutes: data.duration_minutes ?? DEFAULT_TASK_MINUTES,
              title:            data.title,
            },
            { status: data.status },
          )
        : undefined
      return { prev, previousEntries }
    },
    onError: (_e, _v, ctx) => {
      if (ctx?.prev) qc.setQueryData(BOARD_KEY, ctx.prev)
      rollbackCalendar(qc, ctx?.previousEntries)
    },
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
      const previousEntries = await removeCalendarEntries(qc, (e) => e.id === id)
      return { prev, previousEntries }
    },
    onError: (_e, _v, ctx) => {
      if (ctx?.prev) qc.setQueryData(BOARD_KEY, ctx.prev)
      rollbackCalendar(qc, ctx?.previousEntries)
    },
    onSettled: () => invalidateTaskViews(qc),
  })
}
