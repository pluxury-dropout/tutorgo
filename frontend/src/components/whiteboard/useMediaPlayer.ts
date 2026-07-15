'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import {
  isEchoOfRemote,
  isSeekEcho,
  nextMediaState,
  type MediaPayload,
  type MediaState,
} from './mediaSync'

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
  // Согласованное состояние плеера: играет или нет. Ставится и при приёме чужого
  // кадра, и при собственном действии пользователя. По нему отличаем эхо
  // применённой команды от живого клика — см. isEchoOfRemote. Таймер тут не
  // годится: события play/pause у <audio> прилетают асинхронно и гонятся с любым
  // setTimeout, поэтому при быстрых pause/play подавление промахивается и стороны
  // зацикливают команду друг на друга.
  const agreedRef = useRef<boolean | undefined>(undefined)
  // Позиция, выставленная нами по удалённому кадру. Программный сеттер currentTime
  // тоже стреляет 'seeked'; без метки этот отзвук уехал бы обратно и закольцевал
  // перемотку. См. isSeekEcho.
  const expectedSeekRef = useRef<number | null>(null)
  // Актуальное состояние для receive: он зовётся из WS-колбэка, замыкание
  // которого не пересоздаётся на каждый рендер.
  const stateRef = useRef<MediaState | null>(null)
  useEffect(() => {
    stateRef.current = media
  }, [media])

  /** Подтянуть позицию из удалённого кадра, запомнив её, чтобы свой же 'seeked'
   *  не улетел обратно. */
  const applyPosition = (el: SyncTarget, position: number | undefined) => {
    if (position === undefined) return
    if (Math.abs(el.currentTime - position) > SEEK_TOLERANCE_SEC) {
      expectedSeekRef.current = position
      el.currentTime = position
    }
  }

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

      // Согласуем состояние ДО применения: play/pause у элемента выстрелят
      // асинхронно, и localToggle сверится с уже обновлённым agreed.
      if (p.action === 'play') agreedRef.current = true
      if (p.action === 'pause' || p.action === 'close') agreedRef.current = false

      applyPosition(el, p.position)
      if (p.action === 'play') {
        void el.play().catch(() => {
          // Автоплей заблокирован: браузер не даёт играть без жеста
          // пользователя. Показываем кнопку «Включить звук».
          setNeedsGesture(true)
        })
      }
      if (p.action === 'pause' || p.action === 'close') el.pause()
    },
    [sendMedia]
  )

  /** Плеер готов: догоняем кадр, пришедший, пока его не было. */
  const attach = useCallback((t: SyncTarget | null) => {
    mediaRef.current = t
    const pend = pendingRef.current
    if (!t || !pend) return
    pendingRef.current = null
    agreedRef.current = pend.play
    expectedSeekRef.current = pend.position
    t.currentTime = pend.position
    if (pend.play) {
      void t.play().catch(() => setNeedsGesture(true))
    }
  }, [])

  const open = useCallback(
    (p: { url: string; mimeType: string; name: string }) => {
      setMedia({ url: p.url, mimeType: p.mimeType, name: p.name })
      setNeedsGesture(false)
      agreedRef.current = false
      sendMedia({ action: 'open', ...p, position: 0 })
    },
    [sendMedia]
  )

  const close = useCallback(() => {
    agreedRef.current = false
    setMedia(null)
    sendMedia({ action: 'close' })
  }, [sendMedia])

  /** Плеер сообщил play/pause. Своё это или отзвук применённой чужой команды —
   *  решает согласованное состояние; попутно оно же ловит перебуферизацию,
   *  которая даёт лишний PLAYING на ровном месте. */
  const localToggle = useCallback(
    (action: 'play' | 'pause') => {
      if (isEchoOfRemote(agreedRef.current, action)) return
      agreedRef.current = action === 'play'
      if (action === 'play') setNeedsGesture(false)
      sendMedia({ action, position: mediaRef.current?.currentTime ?? 0 })
    },
    [sendMedia]
  )

  const onLocalPlay = useCallback(() => localToggle('play'), [localToggle])
  const onLocalPause = useCallback(() => localToggle('pause'), [localToggle])

  const onLocalSeeked = useCallback(() => {
    const el = mediaRef.current
    if (el && isSeekEcho(expectedSeekRef.current, el.currentTime)) {
      expectedSeekRef.current = null
      return
    }
    sendMedia({ action: 'seek', position: el?.currentTime ?? 0 })
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
