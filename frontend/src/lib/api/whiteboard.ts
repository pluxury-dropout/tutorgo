import { api } from './client'
import type { BoardWithPages, BoardPage, BoardInvite, BoardAssetResponse } from '@/types/api'

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'

export const whiteboardApi = {
  getBoardByCourse: (courseId: string) =>
    api.get<BoardWithPages>(`/boards/course/${courseId}`).then((r) => r.data),

  createPage: (boardId: string, title: string) =>
    api.post<BoardPage>(`/boards/${boardId}/pages`, { title }).then((r) => r.data),

  updatePage: (pageId: string, boardId: string, data: { title?: string; position?: number }) =>
    api.put<BoardPage>(`/board-pages/${pageId}?boardId=${boardId}`, data).then((r) => r.data),

  deletePage: (pageId: string, boardId: string) =>
    api.delete(`/board-pages/${pageId}?boardId=${boardId}`),

  createInvite: (boardId: string) =>
    api.post<BoardInvite>(`/boards/${boardId}/invite`).then((r) => r.data),

  deleteInvite: (boardId: string) =>
    api.delete(`/boards/${boardId}/invite`),

  joinByInvite: (token: string) =>
    api.get<BoardWithPages>(`/public/board/join/${token}`).then((r) => r.data),

  uploadAsset: (boardId: string, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return api
      .post<BoardAssetResponse>(`/boards/${boardId}/assets`, form, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
      .then((r) => r.data)
  },
}

export function getWsUrl(pageId: string, token?: string): string {
  const base = BASE_URL.replace(/^http/, 'ws')
  // The /ws/board route is public and can't read the Authorization header, so
  // the access token must travel as ?token=. A guest invite UUID is passed in
  // explicitly; otherwise fall back to the tutor's access JWT (same storage as
  // the axios request interceptor in lib/api/client.ts).
  const wsToken =
    token ??
    (typeof window !== 'undefined'
      ? localStorage.getItem('tg_token') ?? undefined
      : undefined)
  const params = wsToken ? `?token=${encodeURIComponent(wsToken)}` : ''
  return `${base}/ws/board/${pageId}${params}`
}

export { BASE_URL }
