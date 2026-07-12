import { api } from './client'

export interface RoomTokenResponse {
  token:      string
  room_name:  string
  server_url: string
}

export interface RoomStatusResponse {
  status: 'waiting' | 'active' | 'ended'
}

export interface QuickRoomResponse {
  room_id:    string
  token:      string
  server_url: string
}

export const callsApi = {
  getRoomToken: (lessonId: string) =>
    api.post<RoomTokenResponse>(`/lessons/${lessonId}/room-token`).then((r) => r.data),

  startRoom: (lessonId: string) =>
    api.post(`/lessons/${lessonId}/start-room`).then((r) => r.data),

  endRoom: (lessonId: string) =>
    api.post(`/lessons/${lessonId}/end-room`).then((r) => r.data),

  getRoomStatus: (lessonId: string) =>
    fetch(`/api/room-status/${lessonId}`)
      .then((r) => { if (!r.ok) throw new Error(); return r.json() as Promise<RoomStatusResponse> }),

  startQuickRoom: () =>
    api.post<QuickRoomResponse>('/calls/quick').then((r) => r.data),

  endQuickRoom: (roomId: string) =>
    api.post(`/calls/quick/${roomId}/end`).then((r) => r.data),

  getQuickRoomStatus: (roomId: string) =>
    fetch(`/api/quick-status/${roomId}`)
      .then((r) => { if (!r.ok) throw new Error(); return r.json() as Promise<RoomStatusResponse> }),

  getQuickGuestToken: (roomId: string) =>
    fetch(`/api/quick-guest-token/${roomId}`)
      .then((r) => { if (!r.ok) throw new Error(); return r.json() as Promise<RoomTokenResponse> }),
}
