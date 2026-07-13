import { api } from './client'
import type { Material } from '@/types/api'

export const materialsApi = {
  /** parentId не задан → корень дерева. */
  list: (parentId?: string) =>
    api
      .get<Material[]>('/materials', { params: parentId ? { parent_id: parentId } : {} })
      .then((r) => r.data),

  createFolder: (name: string, parentId?: string) =>
    api
      .post<Material>('/materials/folder', { name, parent_id: parentId ?? null })
      .then((r) => r.data),

  upload: (file: File, parentId?: string) => {
    const form = new FormData()
    form.append('file', file)
    if (parentId) form.append('parent_id', parentId)
    return api
      .post<Material>('/materials', form, {
        // false → axios удаляет заголовок, и браузер сам выставит
        // multipart/form-data c boundary (см. whiteboardApi.uploadAsset).
        headers: { 'Content-Type': false },
      })
      .then((r) => r.data)
  },

  remove: (id: string) => api.delete(`/materials/${id}`).then(() => undefined),

  /** Presigned-ссылка на файл (TTL 4 часа — на весь урок). */
  getUrl: (id: string) =>
    api.get<{ url: string }>(`/materials/${id}/url`).then((r) => r.data.url),
}
