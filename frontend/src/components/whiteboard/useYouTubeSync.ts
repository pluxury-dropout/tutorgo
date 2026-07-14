'use client'

import { useCallback, useRef } from 'react'
import { isEchoOfRemote, type MediaPayload } from './mediaSync'
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
 *  Кадр рассылает только тот, у кого нажал живой человек. Принявший применяет
 *  команду и молчит: иначе его подтверждение уезжает обратно с отставшей
 *  позицией, отправитель откатывается на неё — и стороны бесконечно тянут друг
 *  друга назад.
 *
 *  ponytail: остаточное расхождение (буферизация у сторон разная) не правим —
 *  оно постоянное и не копится. Нужно точнее — синхронизация часов по серверной
 *  метке времени и старт по общей отметке в будущем.
 */
export function useYouTubeSync(
  sendMedia: (p: MediaPayload) => void
): YouTubeSyncApi {
  const targets = useRef(new Map<string, Entry>())
  // Кадр, пришедший раньше плеера: элемент едет по WS отдельно от кадра, и без
  // буфера позиция ролика, включённого до входа ученика, терялась бы.
  const pending = useRef(new Map<string, { position: number; play: boolean }>())
  // Согласованное состояние ролика: играет или нет. Ставится и при приёме чужого
  // кадра, и при собственном действии пользователя. По нему отличаем эхо
  // применённой команды от живого клика — см. isEchoOfRemote.
  const agreed = useRef(new Map<string, boolean>())

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
      agreed.current.set(id, pend.play)
      target.currentTime = pend.position
      if (pend.play) void target.play().catch(onBlocked)
    },
    []
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

      if (p.action === 'play') agreed.current.set(p.id, true)
      if (p.action === 'pause') agreed.current.set(p.id, false)

      // Перемотку глушить отдельно не надо: сеттер currentTime переставляет
      // ожидаемую позицию внутри плеера, и опрос не примет её за движение ползунка.
      if (
        p.position !== undefined &&
        Math.abs(e.target.currentTime - p.position) > SEEK_TOLERANCE_SEC
      ) {
        e.target.currentTime = p.position
      }
      if (p.action === 'play') void e.target.play().catch(e.onBlocked)
      if (p.action === 'pause') e.target.pause()
    },
    [sendMedia]
  )

  const emit = useCallback(
    (action: MediaPayload['action'], id: string) => {
      sendMedia({
        action,
        id,
        position: targets.current.get(id)?.target.currentTime ?? 0,
      })
    },
    [sendMedia]
  )

  /** Плеер сообщил play/pause. Своё это или отзвук применённой чужой команды —
   *  решает согласованное состояние; попутно оно же ловит перебуферизацию,
   *  которая даёт лишний PLAYING на ровном месте. */
  const localToggle = useCallback(
    (id: string, action: 'play' | 'pause') => {
      if (isEchoOfRemote(agreed.current.get(id), action)) return
      agreed.current.set(id, action === 'play')
      emit(action, id)
    },
    [emit]
  )

  const onLocalPlay = useCallback(
    (id: string) => localToggle(id, 'play'),
    [localToggle]
  )
  const onLocalPause = useCallback(
    (id: string) => localToggle(id, 'pause'),
    [localToggle]
  )
  const onLocalSeeked = useCallback((id: string) => emit('seek', id), [emit])

  const resume = useCallback((id: string) => {
    void targets.current.get(id)?.target.play()
  }, [])

  return { attach, onLocalPlay, onLocalPause, onLocalSeeked, resume, receive }
}
