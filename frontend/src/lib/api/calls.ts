import { api } from './client'

export interface RoomTokenResponse {
  token:      string
  room_name:  string
  server_url: string
}

export interface RoomStatusResponse {
  status: 'waiting' | 'active' | 'ended'
}

export const callsApi = {
  getRoomToken: (lessonId: string) =>
    api.post<RoomTokenResponse>(`/lessons/${lessonId}/room-token`).then((r) => r.data),

  startRoom: (lessonId: string) =>
    api.post(`/lessons/${lessonId}/start-room`).then((r) => r.data),

  endRoom: (lessonId: string) =>
    api.post(`/lessons/${lessonId}/end-room`).then((r) => r.data),

  getGuestToken: (lessonId: string) =>
    fetch(`/api/guest-token/${lessonId}`)
      .then((r) => { if (!r.ok) throw new Error(); return r.json() as Promise<RoomTokenResponse> }),

  getRoomStatus: (lessonId: string) =>
    fetch(`/api/room-status/${lessonId}`)
      .then((r) => { if (!r.ok) throw new Error(); return r.json() as Promise<RoomStatusResponse> }),
}
