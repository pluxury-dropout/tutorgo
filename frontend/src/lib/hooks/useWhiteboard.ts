import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { whiteboardApi } from '@/lib/api/whiteboard'

export const boardKeys = {
  byCourse: (courseId: string) => ['board', 'course', courseId] as const,
}

export function useBoardByCourse(courseId: string) {
  return useQuery({
    queryKey: boardKeys.byCourse(courseId),
    queryFn: () => whiteboardApi.getBoardByCourse(courseId),
    enabled: !!courseId,
  })
}

export function useCreatePage(boardId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (title: string) => whiteboardApi.createPage(boardId, title),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['board'] }),
  })
}

export function useUpdatePage(boardId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      pageId,
      data,
    }: {
      pageId: string
      data: { title?: string; position?: number }
    }) => whiteboardApi.updatePage(pageId, boardId, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['board'] }),
  })
}

export function useDeletePage(boardId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (pageId: string) => whiteboardApi.deletePage(pageId, boardId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['board'] }),
  })
}

export function useCreateInvite(boardId: string) {
  return useMutation({
    mutationFn: () => whiteboardApi.createInvite(boardId),
  })
}

export function useDeleteInvite(boardId: string) {
  return useMutation({
    mutationFn: () => whiteboardApi.deleteInvite(boardId),
  })
}

export function useJoinByInvite(token: string) {
  return useQuery({
    queryKey: ['board', 'invite', token],
    queryFn: () => whiteboardApi.joinByInvite(token),
    enabled: !!token,
  })
}
