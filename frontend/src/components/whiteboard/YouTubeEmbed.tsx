'use client'

import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { createYouTubePlayer } from './youtubePlayer'
import type { YouTubeSyncApi } from './useYouTubeSync'

interface Props {
  /** id embeddable-элемента: ключ синхронизации. */
  id: string
  videoId: string
  sync: YouTubeSyncApi
}

/** Содержимое embeddable-элемента: сам ролик. Геометрию (позицию, размер,
 *  перетаскивание) держит Excalidraw — здесь только плеер и его синхронизация. */
export function YouTubeEmbed({ id, videoId, sync }: Props) {
  const boxRef = useRef<HTMLDivElement | null>(null)
  const [failed, setFailed] = useState(false)
  const [needsGesture, setNeedsGesture] = useState(false)
  // Колбэки берём из ref: пересоздавать iframe из-за нового замыкания недопустимо —
  // ролик начался бы заново.
  const syncRef = useRef(sync)
  useEffect(() => {
    syncRef.current = sync
  }, [sync])

  useEffect(() => {
    const box = boxRef.current
    if (!box) return
    let destroy: (() => void) | null = null
    let dead = false

    void createYouTubePlayer(box, videoId, {
      onPlay: () => {
        setNeedsGesture(false)
        syncRef.current.onLocalPlay(id)
      },
      onPause: () => syncRef.current.onLocalPause(id),
      onSeek: () => syncRef.current.onLocalSeeked(id),
      onError: () => setFailed(true),
    })
      .then(({ target, destroy: d }) => {
        // Пока грузился iframe API, элемент могли удалить — тогда сносим сразу.
        if (dead) {
          d()
          return
        }
        destroy = d
        syncRef.current.attach(id, target, () => setNeedsGesture(true))
      })
      .catch(() => setFailed(true))

    return () => {
      dead = true
      syncRef.current.attach(id, null)
      destroy?.()
    }
  }, [id, videoId])

  useEffect(() => {
    if (failed) toast.error('Это видео нельзя встроить — попробуйте другое')
  }, [failed])

  return (
    <div className="relative h-full w-full overflow-hidden bg-black">
      {/* YT API подменяет этот div на iframe, перенося class — отсюда h/w-full. */}
      <div ref={boxRef} className="h-full w-full" />
      {needsGesture && (
        <button
          onClick={() => {
            setNeedsGesture(false)
            syncRef.current.resume(id)
          }}
          className="absolute bottom-2 left-1/2 -translate-x-1/2 rounded-lg bg-white px-3 py-1.5 text-sm font-medium shadow"
        >
          Включить звук
        </button>
      )}
    </div>
  )
}
