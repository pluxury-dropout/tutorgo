'use client'

import type { SyncTarget } from './useMediaPlayer'

// Минимальные типы IFrame API — вместо @types/youtube ради шести методов.
interface YTPlayer {
  playVideo(): void
  pauseVideo(): void
  seekTo(seconds: number, allowSeekAhead: boolean): void
  getCurrentTime(): number
  getPlayerState(): number
  destroy(): void
}
interface YTNamespace {
  Player: new (
    el: HTMLElement,
    opts: {
      videoId: string
      playerVars?: Record<string, number | string>
      events?: {
        onReady?: () => void
        onStateChange?: (e: { data: number }) => void
        onError?: () => void
      }
    }
  ) => YTPlayer
}
declare global {
  interface Window {
    YT?: YTNamespace
    onYouTubeIframeAPIReady?: () => void
  }
}

const STATE_PLAYING = 1
const STATE_PAUSED = 2
const STATE_BUFFERING = 3

/** Проверяем позицию чаще, чем допуск на перемотку: иначе seek заметим с опозданием. */
const POLL_MS = 500
/** Ползунок сдвинули, если позиция прыгнула сильнее, чем могла натикать сама.
 *  Порог с запасом: буферизация и лаги дают рывки в пару десятых. */
const SEEK_JUMP_SEC = 1.5

let apiPromise: Promise<YTNamespace> | null = null

/** Скрипт грузим один раз на вкладку и запоминаем промис — второй <script> сотрёт
 *  window.onYouTubeIframeAPIReady, и первый ожидающий не дождётся колбэка. */
function loadApi(): Promise<YTNamespace> {
  if (apiPromise) return apiPromise
  apiPromise = new Promise<YTNamespace>((resolve, reject) => {
    if (window.YT?.Player) {
      resolve(window.YT)
      return
    }
    const script = document.createElement('script')
    script.src = 'https://www.youtube.com/iframe_api'
    script.onerror = () => reject(new Error('youtube api load failed'))
    window.onYouTubeIframeAPIReady = () => {
      if (window.YT) resolve(window.YT)
      else reject(new Error('youtube api ready without YT'))
    }
    document.head.appendChild(script)
  })
  return apiPromise
}

interface Handlers {
  onPlay: () => void
  onPause: () => void
  onSeek: () => void
  onError: () => void
}

/** Создаёт плеер в контейнере и отдаёт его под видом SyncTarget. */
export async function createYouTubePlayer(
  container: HTMLElement,
  videoId: string,
  h: Handlers
): Promise<{ target: SyncTarget; destroy: () => void }> {
  const YT = await loadApi()

  const player = await new Promise<YTPlayer>((resolve) => {
    const p = new YT.Player(container, {
      videoId,
      // rel=0 — не подсовывать чужие ролики в конце; playsinline — не рвать урок
      // полноэкранным плеером на iOS.
      playerVars: { rel: 0, playsinline: 1, modestbranding: 1 },
      events: {
        onReady: () => resolve(p),
        onStateChange: (e) => {
          if (e.data === STATE_PLAYING) h.onPlay()
          if (e.data === STATE_PAUSED) h.onPause()
        },
        onError: h.onError,
      },
    })
  })

  // У YouTube нет события seeked: ловим перемотку опросом. Ожидаемая позиция
  // тикает вместе с реальным временем, пока играем; разошлись сильнее допуска —
  // значит ползунок двигали (или реклама съехала), и это надо разослать.
  let expected = 0
  let lastCheck = Date.now()
  const timer = window.setInterval(() => {
    const now = Date.now()
    const elapsed = (now - lastCheck) / 1000
    lastCheck = now
    const actual = player.getCurrentTime()
    const playing = player.getPlayerState() === STATE_PLAYING
    if (playing) expected += elapsed
    if (Math.abs(actual - expected) > SEEK_JUMP_SEC) {
      expected = actual
      h.onSeek()
    } else if (playing) {
      // Мелкий дрейф не выдаём за перемотку, но и не копим — берём факт за истину.
      expected = actual
    }
  }, POLL_MS)

  const target: SyncTarget = {
    get currentTime() {
      return player.getCurrentTime()
    },
    set currentTime(sec: number) {
      expected = sec
      player.seekTo(sec, true)
    },
    get paused() {
      const s = player.getPlayerState()
      return s !== STATE_PLAYING && s !== STATE_BUFFERING
    },
    play() {
      player.playVideo()
      // playVideo() ничего не возвращает и молчит, когда браузер режет автоплей.
      // Ждём и смотрим, тронулся ли плеер: не тронулся — значит нужен жест,
      // и хук покажет кнопку «Включить звук».
      return new Promise<void>((resolve, reject) => {
        window.setTimeout(() => {
          const s = player.getPlayerState()
          if (s === STATE_PLAYING || s === STATE_BUFFERING) resolve()
          else reject(new Error('autoplay blocked'))
        }, 1000)
      })
    },
    pause() {
      player.pauseVideo()
    },
  }

  return {
    target,
    destroy: () => {
      window.clearInterval(timer)
      player.destroy()
    },
  }
}
