'use client'

import { useCallback, useRef } from 'react'
import type { MediaPayload } from './mediaSync'
import type { SyncTarget } from './useMediaPlayer'

/** Расхождение больше этого — подтягиваем позицию; меньше — не дёргаем плеер,
 *  чтобы не заикалось видео. Сеть даёт 100–300 мс, для урока незаметно. */
const SEEK_TOLERANCE_SEC = 0.5

interface Entry {
  target: SyncTarget
  /** Автоплей зарезан браузером — рамка покажет «Включить звук». */
  onBlocked: () => void
}

export interface YouTubeSyncApi {
  /** Плеер ролика готов (или размонтирован — тогда target null). */
  attach: (id: string, target: SyncTarget | null, onBlocked?: () => void) => void
  onLocalPlay: (id: string) => void
  onLocalPause: (id: string) => void
  onLocalSeeked: (id: string) => void
  /** Клик по «Включить звук»: жест есть, повторяем play. */
  resume: (id: string) => void
  receive: (p: MediaPayload) => void
}

/** Синхронизация воспроизведения YouTube-роликов, лежащих на доске.
 *
 *  Сам ролик — embeddable-элемент сцены, он приезжает обычным синком элементов.
 *  Здесь ездит только позиция и play/pause, привязанные к id элемента: роликов
 *  на доске может быть несколько.
 *
 *  ponytail: applying — один флаг на все ролики, а не на каждый. Приём кадра
 *  живёт доли секунды, одновременные кадры по разным роликам развести можно,
 *  но пока незачем — максимум пропадёт один эхо-кадр.
 */
export function useYouTubeSync(
  sendMedia: (p: MediaPayload) => void
): YouTubeSyncApi {
  const targets = useRef(new Map<string, Entry>())
  // Кадр, пришедший раньше плеера: элемент едет по WS отдельно от кадра, и без
  // буфера позиция ролика, включённого до входа ученика, терялась бы.
  const pending = useRef(new Map<string, { position: number; play: boolean }>())
  // Пока применяем удалённый кадр, локальные onPlay/onPause/onSeek молчат —
  // иначе приём порождает отправку и получается эхо-петля между вкладками.
  const applying = useRef(false)

  const withSuppressed = useCallback((fn: () => void) => {
    applying.current = true
    fn()
    // Снимаем флаг в макрозадаче: play/pause/seek прилетают асинхронно.
    setTimeout(() => {
      applying.current = false
    }, 0)
  }, [])

  const attach = useCallback(
    (id: string, target: SyncTarget | null, onBlocked: () => void = () => {}) => {
      if (!target) {
        targets.current.delete(id)
        return
      }
      targets.current.set(id, { target, onBlocked })
      const pend = pending.current.get(id)
      if (!pend) return
      pending.current.delete(id)
      withSuppressed(() => {
        target.currentTime = pend.position
        if (pend.play) void target.play().catch(onBlocked)
      })
    },
    [withSuppressed]
  )

  const receive = useCallback(
    (p: MediaPayload) => {
      // Спрашивают состояние (кто-то подключился): отвечаем за каждый свой ролик.
      if (p.action === 'req') {
        for (const [id, e] of targets.current) {
          sendMedia({
            action: e.target.paused ? 'pause' : 'play',
            id,
            position: e.target.currentTime,
          })
        }
        return
      }
      if (!p.id) return

      const e = targets.current.get(p.id)
      if (!e) {
        pending.current.set(p.id, {
          position: p.position ?? pending.current.get(p.id)?.position ?? 0,
          play: p.action === 'play',
        })
        return
      }

      withSuppressed(() => {
        if (
          p.position !== undefined &&
          Math.abs(e.target.currentTime - p.position) > SEEK_TOLERANCE_SEC
        ) {
          e.target.currentTime = p.position
        }
        if (p.action === 'play') void e.target.play().catch(e.onBlocked)
        if (p.action === 'pause') e.target.pause()
      })
    },
    [sendMedia, withSuppressed]
  )

  const emit = useCallback(
    (action: MediaPayload['action'], id: string) => {
      if (applying.current) return
      sendMedia({
        action,
        id,
        position: targets.current.get(id)?.target.currentTime ?? 0,
      })
    },
    [sendMedia]
  )

  const onLocalPlay = useCallback((id: string) => emit('play', id), [emit])
  const onLocalPause = useCallback((id: string) => emit('pause', id), [emit])
  const onLocalSeeked = useCallback((id: string) => emit('seek', id), [emit])

  const resume = useCallback((id: string) => {
    void targets.current.get(id)?.target.play()
  }, [])

  return { attach, onLocalPlay, onLocalPause, onLocalSeeked, resume, receive }
}
