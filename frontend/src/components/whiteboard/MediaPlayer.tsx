'use client'

import { useEffect, useRef, useState } from 'react'
import { Slider } from '@base-ui/react/slider'
import { Check, Pause, Play, Settings, X } from 'lucide-react'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import type { MediaPlayerApi } from './useMediaPlayer'

interface Props {
  player: MediaPlayerApi
  /** Закрывать плеер может только препод. */
  canClose: boolean
}

const SPEEDS = [0.5, 0.75, 1, 1.25, 1.5, 2]

// Пороги из спецификации HTMLMediaElement: данных не хватает на продолжение
// (< HAVE_FUTURE_DATA) и при этом сеть качает (NETWORK_LOADING) — это буферизация.
const HAVE_FUTURE_DATA = 3
const NETWORK_LOADING = 2

function formatTime(sec: number): string {
  if (!Number.isFinite(sec)) return '--:--'
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${s < 10 ? '0' : ''}${s}`
}

interface MediaState {
  time: number
  duration: number
  paused: boolean
  buffering: boolean
  error: MediaError | null
}

/**
 * Состояние опрашиваем в rAF, а не собираем из событий (timeupdate и компания).
 *
 * Событийная сборка требовала ref-колбэка на элементе, а тот пересоздаётся
 * каждый рендер — React дёргал его null/элемент по четыре раза в секунду, и
 * вместе с ним `attach` из useMediaPlayer, который на непустом pending
 * откатывает позицию. Опрос снимает проблему в корне: ref стабильный, элемент
 * остаётся единственным источником истины, а лишние рендеры отсекает сравнение
 * полей — на паузе цикл не рендерит ничего.
 */
function useMediaState(ref: React.RefObject<HTMLMediaElement | null>): MediaState {
  const [state, setState] = useState<MediaState>({
    time: 0,
    duration: NaN,
    paused: true,
    buffering: false,
    error: null,
  })

  useEffect(() => {
    let raf = 0
    const tick = () => {
      const el = ref.current
      if (el) {
        const next: MediaState = {
          time: el.currentTime,
          duration: el.duration,
          paused: el.paused,
          buffering: el.readyState < HAVE_FUTURE_DATA && el.networkState === NETWORK_LOADING,
          error: el.error,
        }
        setState((prev) =>
          prev.time === next.time &&
          prev.duration === next.duration &&
          prev.paused === next.paused &&
          prev.buffering === next.buffering &&
          prev.error === next.error
            ? prev
            : next
        )
      }
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [ref])

  return state
}

export function MediaPlayer({ player, canClose }: Props) {
  const {
    media,
    attach,
    needsGesture,
    rate,
    setRate,
    close,
    onLocalPlay,
    onLocalPause,
    onLocalSeeked,
    resume,
  } = player

  const elRef = useRef<HTMLMediaElement | null>(null)
  const state = useMediaState(elRef)
  // Позиция под пальцем во время перетаскивания. Элемент трогаем только на
  // отпускании: иначе каждый пиксель улетал бы пиру отдельным seek-кадром.
  const [scrub, setScrub] = useState<number | null>(null)
  const [playError, setPlayError] = useState<string | null>(null)

  const url = media?.url
  useEffect(() => {
    // Элемент пересоздаётся вместе с треком (key={url}), поэтому переподключаем
    // синхронизацию на смене файла — и только на ней.
    attach(elRef.current)
    return () => attach(null)
  }, [attach, url])

  if (!media) return null

  const isVideo = media.mimeType.startsWith('video/')
  // До loadedmetadata длительности нет. Base UI требует max > min всегда,
  // поэтому в этот промежуток отдаём фиктивную единицу и гасим слайдер.
  const duration = Number.isFinite(state.duration) && state.duration > 0 ? state.duration : 0
  const playing = !state.paused

  const toggle = () => {
    const el = elRef.current
    if (!el) return
    // Смотрим на сам элемент, а не на state: между кадрами rAF он свежее.
    if (!el.paused) {
      el.pause()
      return
    }
    setPlayError(null)
    // Отказ play() показываем, а не глотаем: истёкшая ссылка и неподдержанный
    // формат приходят именно сюда, и молчащая кнопка — худший способ об этом
    // сообщить.
    void el.play().catch((e: DOMException) => setPlayError(e.message || e.name))
  }

  // play/pause/seeked отдаём хуку — он отличит живой клик от эха применённой
  // чужой команды.
  const common = {
    src: media.url,
    onPlay: onLocalPlay,
    onPause: onLocalPause,
    onSeeked: onLocalSeeked,
  }

  return (
    <div
      data-board-ui
      className="absolute bottom-[66px] left-1/2 z-30 -translate-x-1/2 rounded-[13px] border border-border bg-card px-[11px] pb-[13px] pt-[11px] shadow-[0_8px_24px_rgba(0,0,0,0.14)]"
      style={{ width: isVideo ? 480 : 304 }}
    >
      <div className="flex items-start gap-1.5">
        <span className="min-w-0 flex-1 truncate text-[15px] font-bold tracking-[-0.01em] text-foreground">
          {media.name}
        </span>
        {canClose && (
          <button
            onClick={close}
            title="Закрыть"
            className="flex size-[18px] shrink-0 items-center justify-center text-muted-foreground hover:text-foreground"
          >
            <X className="size-[11px]" strokeWidth={1.9} />
          </button>
        )}
      </div>

      {isVideo ? (
        <video
          key={media.url}
          ref={elRef as React.RefObject<HTMLVideoElement>}
          {...common}
          controls
          className="mt-2.5 w-full rounded-[10px]"
        />
      ) : (
        // Без crossOrigin (в отличие от плеера ElevenLabs, откуда взят вид):
        // src — presigned-ссылка прямо в S3, а CORS у бакета не настроен.
        // Медиа-элемент без crossOrigin грузится в обход CORS, с ним — упрётся.
        <audio key={media.url} ref={elRef as React.RefObject<HTMLAudioElement>} {...common} hidden />
      )}

      {state.error || playError ? (
        <p className="mt-3 text-xs leading-[1.4] text-destructive">
          {state.error
            ? `Не удалось загрузить файл (код ${state.error.code}). Возможно, ссылка устарела — откройте материал заново.`
            : `Не удалось воспроизвести: ${playError}`}
        </p>
      ) : (
        !isVideo && (
          <div className="mt-4 flex items-center gap-[11px]">
            <button
              onClick={toggle}
              title={playing ? 'Пауза' : 'Играть'}
              className="lesson-btn relative flex size-[35px] shrink-0 items-center justify-center rounded-[10px] border border-border text-foreground"
            >
              {playing ? (
                <Pause className="size-[13px]" fill="currentColor" strokeWidth={0} />
              ) : (
                <Play className="size-[13px]" fill="currentColor" strokeWidth={0} />
              )}
              {state.buffering && playing && (
                <span className="absolute inset-0 flex items-center justify-center rounded-[inherit] bg-card/70">
                  <span className="size-3.5 animate-spin rounded-full border-2 border-muted border-t-foreground" />
                </span>
              )}
            </button>

            <span className="shrink-0 text-[11px] tabular-nums text-muted-foreground">
              {formatTime(scrub ?? state.time)}
            </span>

            <Slider.Root
              value={scrub ?? state.time}
              min={0}
              // Пока не пришёл loadedmetadata, длительности нет — а Base UI
              // требует max > min всегда. Держим фиктивную единицу и гасим
              // слайдер: показывать в этот момент всё равно нечего.
              max={duration || 1}
              step={0.25}
              disabled={duration === 0}
              onValueChange={(v) => setScrub(v as number)}
              onValueCommitted={(v) => {
                setScrub(null)
                const el = elRef.current
                if (el) el.currentTime = v as number
              }}
              className="group/seek flex-1"
            >
              <Slider.Control className="flex w-full touch-none items-center py-1.5 select-none">
                <Slider.Track className="relative h-[3px] w-full rounded-full bg-primary-light">
                  <Slider.Indicator className="rounded-full bg-primary" />
                  <Slider.Thumb className="size-3 rounded-full bg-foreground opacity-0 transition-opacity group-hover/seek:opacity-100 data-[dragging]:opacity-100" />
                </Slider.Track>
              </Slider.Control>
            </Slider.Root>

            <span className="shrink-0 text-[11px] tabular-nums text-muted-foreground">
              {formatTime(state.duration)}
            </span>

            <DropdownMenu>
              <DropdownMenuTrigger
                title="Скорость воспроизведения"
                className="lesson-btn flex size-6 shrink-0 items-center justify-center rounded-md text-muted-foreground"
              >
                <Settings className="size-3.5" strokeWidth={1.75} />
              </DropdownMenuTrigger>
              {/* Попап уезжает в портал — там нет карточки урока, поэтому
                  светлую схему включаем ему отдельно. */}
              <DropdownMenuContent align="end" className="lesson-light min-w-[120px]">
                {SPEEDS.map((speed) => (
                  <DropdownMenuItem
                    key={speed}
                    onClick={() => setRate(speed)}
                    className="flex items-center justify-between"
                  >
                    <span>{speed === 1 ? 'Обычная' : `${speed}×`}</span>
                    {rate === speed && <Check className="size-4" />}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        )
      )}

      {needsGesture && (
        <button
          onClick={resume}
          className="mt-2.5 w-full rounded-lg bg-primary px-3 py-[7px] text-[13px] font-medium text-primary-foreground"
        >
          Включить звук
        </button>
      )}
    </div>
  )
}
