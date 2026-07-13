'use client'

import { useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { materialsApi } from '@/lib/api/materials'
import type { Material } from '@/types/api'

// Держать в синхроне с лимитом бэкенда (router.go: multipart 50 МБ).
const MAX_ASSET_BYTES = 50 * 1024 * 1024

const formatMb = (bytes: number) => (bytes / 1024 / 1024).toFixed(1)

/** Ошибки из axios-клиента нормализованы в ApiError ({ message, status }). */
function statusOf(err: unknown): number {
  return typeof err === 'object' && err !== null && 'status' in err
    ? Number((err as { status: unknown }).status)
    : 0
}

interface Props {
  onClose: () => void
  /** Файл выбран: панель уже получила presigned-ссылку. Что с ней делать —
   *  решает родитель (плеер, доска, скачивание) — панель про это не знает. */
  onPick: (material: Material, url: string) => void
}

export function MaterialsPanel({ onClose, onPick }: Props) {
  // Путь до текущей папки; пустой массив — корень.
  const [path, setPath] = useState<Array<{ id: string; name: string }>>([])
  const [busy, setBusy] = useState(false)
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const qc = useQueryClient()

  const parentId = path.length > 0 ? path[path.length - 1].id : undefined

  const { data: items = [], isLoading } = useQuery({
    queryKey: ['materials', parentId ?? 'root'],
    queryFn: () => materialsApi.list(parentId),
  })

  const reload = () => qc.invalidateQueries({ queryKey: ['materials', parentId ?? 'root'] })

  const handleCreateFolder = async () => {
    // Визуал переделывается отдельно — здесь достаточно нативного диалога.
    const name = window.prompt('Название папки')?.trim()
    if (!name) return
    setBusy(true)
    try {
      await materialsApi.createFolder(name, parentId)
      await reload()
    } catch {
      toast.error('Не удалось создать папку')
    } finally {
      setBusy(false)
    }
  }

  const handleUpload = async (file: File) => {
    // Отсекаем до аплоада: иначе пользователь ждёт заливку 100 МБ ради 413.
    if (file.size > MAX_ASSET_BYTES) {
      toast.error(
        `Файл ${formatMb(file.size)} МБ, максимум ${formatMb(MAX_ASSET_BYTES)} МБ`
      )
      return
    }
    setBusy(true)
    try {
      await materialsApi.upload(file, parentId)
      await reload()
    } catch {
      toast.error('Не удалось загрузить файл')
    } finally {
      setBusy(false)
    }
  }

  const handleDelete = async (m: Material) => {
    if (!window.confirm(`Удалить «${m.name}»?`)) return
    setBusy(true)
    try {
      await materialsApi.remove(m.id)
      await reload()
    } catch (err) {
      // 409 — единственный ожидаемый отказ: рекурсивного удаления нет.
      toast.error(statusOf(err) === 409 ? 'Папка не пуста' : 'Не удалось удалить')
    } finally {
      setBusy(false)
    }
  }

  const handleClick = async (m: Material) => {
    if (m.kind === 'folder') {
      setPath((p) => [...p, { id: m.id, name: m.name }])
      return
    }
    setBusy(true)
    try {
      const url = await materialsApi.getUrl(m.id)
      onPick(m, url)
    } catch {
      toast.error('Не удалось открыть материал')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div
      data-board-ui
      className="absolute right-2 top-14 z-20 w-72 rounded-xl bg-white p-3 shadow-lg"
    >
      <div className="mb-2 flex items-center justify-between">
        <span className="text-sm font-semibold text-gray-900">Материалы</span>
        <button
          onClick={onClose}
          className="px-1 text-sm text-gray-500 hover:text-gray-900"
          title="Закрыть"
        >
          ✕
        </button>
      </div>

      {/* Хлебные крошки: клик поднимает на нужный уровень. */}
      <div className="mb-2 flex flex-wrap items-center gap-1 text-xs text-gray-500">
        <button onClick={() => setPath([])} className="hover:text-gray-900">
          Все
        </button>
        {path.map((f, i) => (
          <span key={f.id} className="flex items-center gap-1">
            <span>/</span>
            <button
              onClick={() => setPath((p) => p.slice(0, i + 1))}
              className="max-w-[7rem] truncate hover:text-gray-900"
            >
              {f.name}
            </button>
          </span>
        ))}
      </div>

      <div className="mb-2 flex gap-2">
        <button
          onClick={handleCreateFolder}
          disabled={busy}
          className="flex-1 rounded-lg border border-gray-200 px-2 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
        >
          Новая папка
        </button>
        <button
          onClick={() => fileInputRef.current?.click()}
          disabled={busy}
          className="flex-1 rounded-lg bg-gray-900 px-2 py-1.5 text-xs font-medium text-white hover:bg-black disabled:opacity-50"
        >
          Загрузить
        </button>
        <input
          ref={fileInputRef}
          type="file"
          className="hidden"
          onChange={(e) => {
            const file = e.target.files?.[0]
            // Сбрасываем value: иначе повторный выбор того же файла не даст change.
            e.target.value = ''
            if (file) void handleUpload(file)
          }}
        />
      </div>

      <div className="max-h-72 overflow-y-auto">
        {isLoading ? (
          <p className="py-4 text-center text-xs text-gray-400">Загрузка…</p>
        ) : items.length === 0 ? (
          <p className="py-4 text-center text-xs text-gray-400">Пусто</p>
        ) : (
          <ul className="flex flex-col">
            {items.map((m) => (
              <li key={m.id} className="group flex items-center gap-1 rounded-lg hover:bg-gray-50">
                <button
                  onClick={() => void handleClick(m)}
                  disabled={busy}
                  className="flex min-w-0 flex-1 items-center gap-2 px-2 py-1.5 text-left text-sm text-gray-800 disabled:opacity-50"
                  title={m.name}
                >
                  <span className="shrink-0 text-gray-400">
                    {m.kind === 'folder' ? '📁' : '📄'}
                  </span>
                  <span className="truncate">{m.name}</span>
                </button>
                <button
                  onClick={() => void handleDelete(m)}
                  disabled={busy}
                  className="px-2 text-xs text-gray-300 hover:text-red-600 disabled:opacity-50 group-hover:text-gray-500"
                  title="Удалить"
                >
                  ✕
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
