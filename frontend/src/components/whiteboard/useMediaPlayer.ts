'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { nextMediaState, type MediaPayload, type MediaState } from './mediaSync'

/** Расхождение больше этого — подтягиваем позицию; меньше — не дёргаем элемент,
 *  чтобы не заикался звук. Сеть даёт 100–300 мс, для аудирования незаметно. */
const SEEK_TOLERANCE_SEC = 0.5

/** Всё, что хук требует от плеера. `<audio>`/`<video>` подходят структурно, а
 *  YouTube-ролик подсовывается адаптером (youtubePlayer.ts) — хук про него не знает. */
export interface SyncTarget {
  currentTime: number
  readonly paused: boolean
  play(): Promise<void>
  pause(): void
}

export interface MediaPlayerApi {
  media: MediaState | null
  /** Плеер готов (элемент смонтирован / YT-адаптер создан). null — при размонтировании. */
  attach: (t: SyncTarget | null) => void
  needsGesture: boolean
  open: (p: { url: string; mimeType: string; name: string }) => void
  close: () => void
  onLocalPlay: () => void
  onLocalPause: () => void
  onLocalSeeked: () => void
  receive: (p: MediaPayload) => void
  resume: () => void
}

export function useMediaPlayer(
  sendMedia: (p: MediaPayload) => void
): MediaPlayerApi {
  const [media, setMedia] = useState<MediaState | null>(null)
  const [needsGesture, setNeedsGesture] = useState(false)
  const mediaRef = useRef<SyncTarget | null>(null)
  // Кадр, пришедший до готовности плеера: между `open` и монтированием <audio>
  // (а у YouTube — созданием iframe) проходит рендер, и без буфера позиция
  // подключившегося посреди трека ученика терялась бы.
  const pendingRef = useRef<{ position: number; play: boolean } | null>(null)
  // Пока применяем удалённый кадр, локальные onPlay/onPause/onSeeked молчат —
  // иначе приём порождает отправку и получается эхо-петля между вкладками.
  const applyingRef = useRef(false)
  // Актуальное состояние для receive: он зовётся из WS-колбэка, замыкание
  // которого не пересоздаётся на каждый рендер.
  const stateRef = useRef<MediaState | null>(null)
  useEffect(() => {
    stateRef.current = media
  }, [media])

  const withSuppressed = useCallback((fn: () => void) => {
    applyingRef.current = true
    fn()
    // Снимаем флаг в макрозадаче: play/pause/seeked прилетают асинхронно.
    setTimeout(() => {
      applyingRef.current = false
    }, 0)
  }, [])

  /** Приём кадра из WS. */
  const receive = useCallback(
    (p: MediaPayload) => {
      // Спрашивают состояние: если у нас открыт файл — отвечаем своим кадром.
      if (p.action === 'req') {
        const cur = stateRef.current
        const el = mediaRef.current
        if (cur) {
          sendMedia({
            action: 'open',
            url: cur.url,
            mimeType: cur.mimeType,
            name: cur.name,
            position: el?.currentTime ?? 0,
          })
          if (el && !el.paused) {
            sendMedia({ action: 'play', position: el.currentTime })
          }
        }
        return
      }

      setMedia((prev) => nextMediaState(prev, p))

      const el = mediaRef.current
      if (!el) {
        // Плеера ещё нет — копим кадр до attach. Закрытие копить незачем.
        if (p.action !== 'close') {
          pendingRef.current = {
            position: p.position ?? pendingRef.current?.position ?? 0,
            play: p.action === 'play',
          }
        }
        return
      }

      withSuppressed(() => {
        if (p.position !== undefined) {
          if (Math.abs(el.currentTime - p.position) > SEEK_TOLERANCE_SEC) {
            el.currentTime = p.position
          }
        }
        if (p.action === 'play') {
          void el.play().catch(() => {
            // Автоплей заблокирован: браузер не даёт играть без жеста
            // пользователя. Показываем кнопку «Включить звук».
            setNeedsGesture(true)
          })
        }
        if (p.action === 'pause') el.pause()
        if (p.action === 'close') el.pause()
      })
    },
    [sendMedia, withSuppressed]
  )

  /** Плеер готов: догоняем кадр, пришедший, пока его не было. */
  const attach = useCallback(
    (t: SyncTarget | null) => {
      mediaRef.current = t
      const pend = pendingRef.current
      if (!t || !pend) return
      pendingRef.current = null
      withSuppressed(() => {
        t.currentTime = pend.position
        if (pend.play) {
          void t.play().catch(() => setNeedsGesture(true))
        }
      })
    },
    [withSuppressed]
  )

  const open = useCallback(
    (p: { url: string; mimeType: string; name: string }) => {
      setMedia({ url: p.url, mimeType: p.mimeType, name: p.name })
      setNeedsGesture(false)
      sendMedia({ action: 'open', ...p, position: 0 })
    },
    [sendMedia]
  )

  const close = useCallback(() => {
    setMedia(null)
    sendMedia({ action: 'close' })
  }, [sendMedia])

  const onLocalPlay = useCallback(() => {
    if (applyingRef.current) return
    setNeedsGesture(false)
    sendMedia({ action: 'play', position: mediaRef.current?.currentTime ?? 0 })
  }, [sendMedia])

  const onLocalPause = useCallback(() => {
    if (applyingRef.current) return
    sendMedia({ action: 'pause', position: mediaRef.current?.currentTime ?? 0 })
  }, [sendMedia])

  const onLocalSeeked = useCallback(() => {
    if (applyingRef.current) return
    sendMedia({ action: 'seek', position: mediaRef.current?.currentTime ?? 0 })
  }, [sendMedia])

  /** Клик по «Включить звук»: жест есть, повторяем play. */
  const resume = useCallback(() => {
    setNeedsGesture(false)
    void mediaRef.current?.play()
  }, [])

  return {
    media,
    attach,
    needsGesture,
    open,
    close,
    onLocalPlay,
    onLocalPause,
    onLocalSeeked,
    receive,
    resume,
  }
}
