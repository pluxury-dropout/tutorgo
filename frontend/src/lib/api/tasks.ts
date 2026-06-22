import { api } from './client'
import { Task } from '@/types/api'

export interface TaskInput {
  title: string
  scheduled_at?: string | null
  duration_minutes?: number | null
  status?: string
}

export interface TaskUpdateInput extends TaskInput {
  status: string
}

export const tasksApi = {
  list: (from: string, to: string) =>
    api.get<Task[]>('/tasks', { params: { from, to } }).then((r) => r.data ?? []),
  board: () =>
    api.get<Task[]>('/tasks/board').then((r) => r.data ?? []),
  create: (data: TaskInput) =>
    api.post<Task>('/tasks', data).then((r) => r.data),
  update: (id: string, data: TaskUpdateInput) =>
    api.put<Task>(`/tasks/${id}`, data).then((r) => r.data),
  delete: (id: string) =>
    api.delete(`/tasks/${id}`).then(() => id),
}
