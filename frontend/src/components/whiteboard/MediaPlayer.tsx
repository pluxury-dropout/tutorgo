'use client'

import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { YOUTUBE_MIME } from './mediaSync'
import { createYouTubePlayer } from './youtubePlayer'
import type { MediaPlayerApi } from './useMediaPlayer'

interface Props {
  player: MediaPlayerApi
  /** Закрывать плеер может только препод. */
  canClose: boolean
}

export function MediaPlayer({ player, canClose }: Props) {
  const {
    media,
    attach,
    needsGesture,
    close,
    onLocalPlay,
    onLocalPause,
    onLocalSeeked,
    resume,
  } = player
  if (!media) return null

  const isYouTube = media.mimeType === YOUTUBE_MIME
  const isVideo = media.mimeType.startsWith('video/')

  // ref-колбэком, а не объектом: RefObject<SyncTarget> не присваивается ref у
  // <audio>/<video> — там инвариантный HTMLAudioElement/HTMLVideoElement.
  const common = {
    ref: (el: HTMLMediaElement | null) => attach(el),
    src: media.url,
    controls: true,
    onPlay: onLocalPlay,
    onPause: onLocalPause,
    onSeeked: onLocalSeeked,
  }

  return (
    <div
      data-board-ui
      className="absolute bottom-4 left-1/2 -translate-x-1/2 z-20 flex flex-col gap-2 rounded-xl bg-white p-3 shadow-lg"
      style={{ width: isVideo ? 480 : 360 }}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-sm font-medium">{media.name}</span>
        {canClose && (
          <button onClick={close} className="text-sm text-gray-500 hover:text-gray-900">
            ✕
          </button>
        )}
      </div>

      {/* key: смена ролика должна пересоздать плеер, а не переиспользовать старый. */}
      {isYouTube ? (
        <YouTubeFrame key={media.url} videoId={media.url} player={player} />
      ) : isVideo ? (
        <video {...common} className="w-full rounded-lg" />
      ) : (
        <audio {...common} className="w-full" />
      )}

      {needsGesture && (
        <button
          onClick={resume}
          className="rounded-lg bg-black px-3 py-1.5 text-sm font-medium text-white"
        >
          Включить звук
        </button>
      )}
    </div>
  )
}

function YouTubeFrame({ videoId, player }: { videoId: string; player: MediaPlayerApi }) {
  const boxRef = useRef<HTMLDivElement | null>(null)
  const [failed, setFailed] = useState(false)
  // Колбэки берём из ref: пересоздавать iframe из-за нового замыкания недопустимо —
  // ролик начался бы заново.
  const apiRef = useRef(player)
  useEffect(() => {
    apiRef.current = player
  }, [player])

  useEffect(() => {
    const box = boxRef.current
    if (!box) return
    let destroy: (() => void) | null = null
    let dead = false

    void createYouTubePlayer(box, videoId, {
      onPlay: () => apiRef.current.onLocalPlay(),
      onPause: () => apiRef.current.onLocalPause(),
      onSeek: () => apiRef.current.onLocalSeeked(),
      onError: () => setFailed(true),
    })
      .then(({ target, destroy: d }) => {
        // Пока грузился iframe API, плеер могли закрыть — тогда сносим сразу.
        if (dead) {
          d()
          return
        }
        destroy = d
        apiRef.current.attach(target)
      })
      .catch(() => setFailed(true))

    return () => {
      dead = true
      apiRef.current.attach(null)
      destroy?.()
    }
  }, [videoId])

  useEffect(() => {
    if (failed) toast.error('Это видео нельзя встроить — попробуйте другое')
  }, [failed])

  return (
    <div className="aspect-video w-full overflow-hidden rounded-lg bg-black">
      <div ref={boxRef} className="h-full w-full" />
    </div>
  )
}
