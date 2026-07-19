import { api } from './client'
import type {
  BoardWithPages,
  BoardPage,
  BoardInvite,
  BoardAssetResponse,
  PdfPreflightResponse,
  PdfStartResponse,
} from '@/types/api'

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'

export const whiteboardApi = {
  getBoardByCourse: (courseId: string) =>
    api.get<BoardWithPages>(`/boards/course/${courseId}`).then((r) => r.data),

  getTrialBoard: () =>
    api.get<BoardWithPages>('/boards/trial').then((r) => r.data),

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

  // inviteToken задан → гость (ученик): льём через публичный роут доски, tutor JWT
  // не нужен. Иначе — препод по защищённому роуту.
  uploadAsset: (boardId: string, file: File, inviteToken?: string) => {
    const form = new FormData()
    form.append('file', file)
    const path = inviteToken
      ? `/public/board/${inviteToken}/assets`
      : `/boards/${boardId}/assets`
    return api
      .post<BoardAssetResponse>(path, form, {
        // false → axios удаляет заголовок, и браузер сам выставит
        // multipart/form-data c boundary. Со строкой 'multipart/form-data'
        // boundary теряется и Go не может распарсить тело (FormFile → 400).
        headers: { 'Content-Type': false },
      })
      .then((r) => r.data)
  },

  // Preflight: оригинал PDF уезжает на сервер, обратно — паспорт документа.
  uploadPdf: (boardId: string, pageId: string, file: File) => {
    const form = new FormData()
    form.append('file', file)
    form.append('page_id', pageId)
    return api
      .post<PdfPreflightResponse>(`/boards/${boardId}/pdf`, form, {
        headers: { 'Content-Type': false }, // boundary выставит браузер
      })
      .then((r) => r.data)
  },

  startPdfImport: (importId: string, from: number, to: number) =>
    api
      .post<PdfStartResponse>(`/pdf-imports/${importId}/start`, { from, to })
      .then((r) => r.data),
}

export function getWsUrl(pageId: string, token?: string): string {
  // Strip trailing slash so a slash in NEXT_PUBLIC_API_URL doesn't produce
  // `//ws/board/...` — the double slash triggers a 301 redirect that breaks the
  // WS handshake (REST is fine because axios normalizes the join).
  const base = BASE_URL.replace(/\/+$/, '').replace(/^http/, 'ws')
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

// Персист доски идёт мимо axios: тело — уже готовая строка (см. serializeSnapshot),
// а токен — в query, как у WS, чтобы тот же роут работал и для гостя по invite.
function snapshotUrl(pageId: string, token?: string): string {
  const base = BASE_URL.replace(/\/+$/, '')
  const params = token ? `?token=${encodeURIComponent(token)}` : ''
  return `${base}/public/board/pages/${pageId}/snapshot${params}`
}

// Бросает на любой не-2xx: вызывающий обязан показать, что доска не сохранилась.
export async function saveSnapshot(
  pageId: string,
  body: string,
  token?: string
): Promise<void> {
  const resp = await fetch(snapshotUrl(pageId, token), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body,
  })
  if (!resp.ok) {
    throw new Error(`snapshot save failed: ${resp.status}`)
  }
}

// Финальный снапшот при уходе со страницы. fetch на unload браузер отменяет,
// sendBeacon — единственный способ дослать; он же синхронный, поэтому токен
// берётся готовым, без refresh-а. Возвращает false, если браузер отказал
// (обычно превышен beacon-лимит ~64 КБ) — тогда вызывающий шлёт обычным fetch.
export function beaconSnapshot(
  pageId: string,
  body: string,
  token?: string
): boolean {
  if (typeof navigator === 'undefined' || !navigator.sendBeacon) return false
  return navigator.sendBeacon(
    snapshotUrl(pageId, token),
    new Blob([body], { type: 'application/json' })
  )
}

export { BASE_URL }
