import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { tasksApi, TaskInput, TaskUpdateInput } from '@/lib/api/tasks'

export function useTasks(from: string, to: string) {
  return useQuery({
    queryKey: ['tasks', from, to],
    queryFn:  () => tasksApi.list(from, to),
    enabled:  !!from && !!to,
  })
}

export function useCreateTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: TaskInput) => tasksApi.create(data),
    onSuccess:  () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}

export function useToggleTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => tasksApi.toggleDone(id),
    onSuccess:  () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}

export function useRescheduleTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: TaskUpdateInput }) => tasksApi.update(id, data),
    onSuccess:  () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}

export function useDeleteTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => tasksApi.delete(id),
    onSuccess:  () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}
