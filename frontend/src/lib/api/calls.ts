import { api } from './client'
import axios from 'axios'

export interface RoomTokenResponse {
  token:      string
  room_name:  string
  server_url: string
}

export const callsApi = {
  getRoomToken: (lessonId: string) =>
    api.post<RoomTokenResponse>(`/lessons/${lessonId}/room-token`).then((r) => r.data),

  getGuestToken: (lessonId: string) => {
    const baseURL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'
    return axios
      .get<RoomTokenResponse>(`${baseURL}/public/lessons/${lessonId}/guest-token`)
      .then((r) => r.data)
  },
}
