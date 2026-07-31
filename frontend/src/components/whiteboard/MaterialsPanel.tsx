'use client'

import { useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronLeft, Folder, Library, Music, X } from 'lucide-react'
import { toast } from 'sonner'
import { materialsApi } from '@/lib/api/materials'
import { isPlayable } from './mediaSync'
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

  const handleUpload = async (files: File[]) => {
    // accept у input — только подсказка диалогу, обойти его можно перетаскиванием;
    // фильтруем по-настоящему здесь.
    files
      .filter((f) => !isPlayable(f.type))
      .forEach((f) => toast.error(`${f.name}: только аудио и видео`))
    // Отсекаем до аплоада: иначе пользователь ждёт заливку 100 МБ ради 413.
    files
      .filter((f) => isPlayable(f.type) && f.size > MAX_ASSET_BYTES)
      .forEach((f) =>
        toast.error(`${f.name}: ${formatMb(f.size)} МБ, максимум ${formatMb(MAX_ASSET_BYTES)} МБ`)
      )
    const queue = files.filter((f) => isPlayable(f.type) && f.size <= MAX_ASSET_BYTES)
    if (queue.length === 0) return

    setBusy(true)
    // ponytail: заливаем по одному. Promise.all быстрее, но держит все файлы в
    // памяти разом и упирается в лимит соединений; параллелить — когда станет узким.
    const toastId = queue.length > 1 ? toast.loading(`Загрузка 0 из ${queue.length}…`) : undefined
    let done = 0
    const failed: string[] = []
    for (const file of queue) {
      try {
        await materialsApi.upload(file, parentId)
        done++
      } catch {
        failed.push(file.name)
      }
      if (toastId) toast.loading(`Загрузка ${done} из ${queue.length}…`, { id: toastId })
    }
    if (toastId) toast.dismiss(toastId)
    if (failed.length > 0) {
      toast.error(
        failed.length === queue.length
          ? 'Не удалось загрузить файлы'
          : `Не загрузились: ${failed.join(', ')}`
      )
    }
    await reload()
    setBusy(false)
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

  const uploadButton = (
    <button
      onClick={() => fileInputRef.current?.click()}
      disabled={busy}
      className="h-9 rounded-[10px] bg-primary px-4 text-[13px] font-medium text-primary-foreground hover:opacity-90 disabled:opacity-50"
    >
      Загрузить
    </button>
  )

  const current = path.length > 0 ? path[path.length - 1] : null

  return (
    <div
      data-board-ui
      className="absolute right-3.5 top-[62px] z-20 max-h-[480px] w-[280px] overflow-auto rounded-2xl border border-border bg-card p-5 shadow-[0_12px_32px_rgba(0,0,0,0.18)]"
    >
      {current ? (
        <div className="flex items-center gap-2">
          <button
            onClick={() => setPath((p) => p.slice(0, -1))}
            title="Назад"
            className="flex size-[26px] items-center justify-center text-muted-foreground hover:text-foreground"
          >
            <ChevronLeft className="size-4" />
          </button>
          <span className="truncate text-base font-semibold text-foreground">{current.name}</span>
          <button
            onClick={onClose}
            title="Закрыть"
            className="ml-auto flex size-6 shrink-0 items-center justify-center text-muted-foreground hover:text-foreground"
          >
            <X className="size-4" />
          </button>
        </div>
      ) : (
        <>
          <div className="flex items-center justify-between">
            <span className="text-[17px] font-semibold text-foreground">Материалы</span>
            <button
              onClick={onClose}
              title="Закрыть"
              className="flex size-6 items-center justify-center text-muted-foreground hover:text-foreground"
            >
              <X className="size-4" />
            </button>
          </div>
          <p className="mt-2.5 text-[13px] leading-[1.4] text-muted-foreground">
            Аудио для аудирования: файлы и папки, которые вы открываете во время урока.
          </p>
          <div className="my-3.5 text-[13px] text-muted-foreground">Все</div>
        </>
      )}

      <div className={`flex flex-col gap-2 ${current ? 'mt-4' : ''} mb-4`}>
        {!current && (
          <button
            onClick={handleCreateFolder}
            disabled={busy}
            className="h-9 rounded-[10px] border border-border bg-card text-[13px] font-medium text-foreground hover:bg-muted disabled:opacity-50"
          >
            Новая папка
          </button>
        )}
        {items.length > 0 && uploadButton}
        <input
          ref={fileInputRef}
          type="file"
          multiple
          accept="audio/*,video/*"
          className="hidden"
          onChange={(e) => {
            const files = Array.from(e.target.files ?? [])
            // Сбрасываем value: иначе повторный выбор того же файла не даст change.
            e.target.value = ''
            if (files.length > 0) void handleUpload(files)
          }}
        />
      </div>

      {isLoading ? (
        <p className="py-4 text-center text-xs text-muted-foreground">Загрузка…</p>
      ) : items.length === 0 ? (
        // Пустая папка — единственное место, где загрузка объясняется словами.
        <div className="flex flex-col items-center px-2.5 pb-3 pt-4 text-center">
          <div className="mb-3.5 flex size-14 items-center justify-center rounded-full bg-muted">
            <Library className="size-6 text-muted-foreground" strokeWidth={1.5} />
          </div>
          <p className="mb-1 text-sm font-medium text-foreground">Нет файлов</p>
          <p className="mb-4 text-xs text-muted-foreground">Добавьте аудио в эту папку</p>
          {uploadButton}
        </div>
      ) : (
        <ul className="flex flex-col">
          {items.map((m) => (
            <li key={m.id} className="group flex items-center rounded-[10px] hover:bg-muted">
              <button
                onClick={() => void handleClick(m)}
                disabled={busy}
                className="flex min-w-0 flex-1 items-center gap-2.5 px-2 py-2.5 text-left text-sm text-foreground disabled:opacity-50"
                title={m.name}
              >
                {m.kind === 'folder' ? (
                  <Folder
                    className="size-[18px] shrink-0"
                    strokeWidth={1}
                    style={{ fill: '#F0C36B', stroke: '#C99A3E' }}
                  />
                ) : (
                  <Music className="size-[17px] shrink-0 text-muted-foreground" strokeWidth={1.75} />
                )}
                <span className="truncate">{m.name}</span>
                {m.kind === 'file' && (
                  <span className="ml-auto shrink-0 text-xs text-muted-foreground">
                    {formatMb(m.size_bytes)} МБ
                  </span>
                )}
              </button>
              <button
                onClick={() => void handleDelete(m)}
                disabled={busy}
                className="px-2 text-transparent hover:text-destructive disabled:opacity-50 group-hover:text-muted-foreground"
                title="Удалить"
              >
                <X className="size-3.5" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
