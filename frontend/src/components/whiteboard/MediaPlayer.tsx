'use client'

import type { MediaPlayerApi } from './useMediaPlayer'

interface Props {
  player: MediaPlayerApi
  /** Закрывать плеер может только препод. */
  canClose: boolean
}

export function MediaPlayer({ player, canClose }: Props) {
  const {
    media,
    mediaRef,
    needsGesture,
    close,
    onLocalPlay,
    onLocalPause,
    onLocalSeeked,
    resume,
  } = player
  if (!media) return null

  const isVideo = media.mimeType.startsWith('video/')
  // ref-колбэком, а не объектом: RefObject<HTMLMediaElement> не присваивается
  // ref у <audio>/<video> (там HTMLAudioElement/HTMLVideoElement, инвариантно).
  const common = {
    ref: (el: HTMLMediaElement | null) => {
      mediaRef.current = el
    },
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
          <button
            onClick={close}
            className="text-sm text-gray-500 hover:text-gray-900"
          >
            ✕
          </button>
        )}
      </div>

      {isVideo ? (
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
