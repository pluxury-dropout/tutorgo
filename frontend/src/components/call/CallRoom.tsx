'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import dynamic from 'next/dynamic'
import {
  LiveKitRoom,
  RoomAudioRenderer,
  useConnectionState,
  useRoomContext,
} from '@livekit/components-react'
import { ConnectionState, RoomEvent } from 'livekit-client'
import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import '@livekit/components-styles'
import { toast } from 'sonner'

import { useTheme } from 'next-themes'
import { CallParticipants } from './CallParticipants'
import { CallStage } from './CallStage'
import { CallToolbar } from './CallToolbar'
import { HomeworkEditDialog } from '@/components/homework/HomeworkEditDialog'
import { HomeworkViewDialog } from '@/components/homework/HomeworkViewDialog'
import { CallChat } from './CallChat'
import { useCallChat } from './useCallChat'
import { themeTokens } from './callTheme'

// Excalidraw трогает window при инициализации — только клиент, без SSR.
const ExcalidrawCanvas = dynamic(
  () =>
    import('@/components/whiteboard/ExcalidrawCanvas').then(
      (m) => m.ExcalidrawCanvas
    ),
  { ssr: false }
)
import { whiteboardApi } from '@/lib/api/whiteboard'
import { useBoardDisplayName } from '@/lib/hooks/useBoardDisplayName'
import type { BoardWithPages } from '@/types/api'

type Mode = 'call' | 'board'

interface DataMessage {
  type: 'board-open' | 'board-close'
  board_token?: string
}

// ─── Inner component (must live inside <LiveKitRoom>) ───────────────────────

interface CallRoomInnerProps {
  courseId?: string
  role: 'tutor' | 'guest'
  inviteUrl?: string
  /** Пробный урок: доска берётся из общей trial-доски препода, курса нет. */
  trial?: boolean
}

function CallRoomInner({ courseId, role, inviteUrl, trial }: CallRoomInnerProps) {
  const room = useRoomContext()
  const { resolvedTheme } = useTheme()
  const theme = themeTokens(resolvedTheme === 'dark' ? 'dark' : 'light')

  const identity = useBoardDisplayName(role)
  const [mode, setMode] = useState<Mode>('call')
  const [chatOpen, setChatOpen] = useState(false)
  const [homeworkOpen, setHomeworkOpen] = useState(false)
  // api доски нужен панели участников (follow за коллаборатором).
  const [boardApi, setBoardApi] = useState<ExcalidrawImperativeAPI | null>(null)
  const { messages, send, unread } = useCallChat({ chatOpen })

  // Enable camera+mic once per call. Must live here (not in CallStage): CallStage
  // unmounts/remounts on every board toggle, and re-running enableCameraAndMicrophone
  // races with itself (StrictMode double-invoke + overlapping toggles) → duplicate
  // camera publications on the same participant. The ref guards the StrictMode replay.
  const mediaStartedRef = useRef(false)
  useEffect(() => {
    if (mediaStartedRef.current) return
    mediaStartedRef.current = true
    room.localParticipant.enableCameraAndMicrophone().catch(() => {})
  }, [room])

  // Tutor state
  const [tutorBoard, setTutorBoard] = useState<BoardWithPages | null>(null)
  const [boardFailed, setBoardFailed] = useState(false)

  // Guest state
  const [guestBoardToken, setGuestBoardToken] = useState<string | null>(null)
  const [guestBoard, setGuestBoard] = useState<BoardWithPages | null>(null)

  // Guest: handle board-open sent before this participant joined (participant metadata)
  useEffect(() => {
    if (role !== 'guest') return
    for (const participant of room.remoteParticipants.values()) {
      let meta: { boardOpen?: boolean; boardToken?: string } | null = null
      try { meta = participant.metadata ? JSON.parse(participant.metadata) : null } catch {}
      if (!meta?.boardOpen || !meta.boardToken) continue
      const boardToken = meta.boardToken
      whiteboardApi.joinByInvite(boardToken).then((data) => {
        setGuestBoardToken(boardToken)
        setGuestBoard(data)
        setMode('board')
      }).catch(() => {
        toast.error('Не удалось открыть доску')
      })
      break
    }
  }, [room, role])

  // Both: listen for DataChannel events
  useEffect(() => {
    function onData(payload: Uint8Array) {
      let msg: DataMessage
      try { msg = JSON.parse(new TextDecoder().decode(payload)) } catch { return }

      if (msg.type === 'board-open' && msg.board_token) {
        const boardToken = msg.board_token
        if (role === 'guest') {
          whiteboardApi.joinByInvite(boardToken).then((data) => {
            setGuestBoardToken(boardToken)
            setGuestBoard(data)
            setMode('board')
          }).catch(() => toast.error('Не удалось открыть доску'))
        }
      } else if (msg.type === 'board-close') {
        setMode('call')
        setGuestBoardToken(null)
        setGuestBoard(null)
      }
    }

    room.on(RoomEvent.DataReceived, onData)
    return () => { room.off(RoomEvent.DataReceived, onData) }
  }, [room, role])

  // Раньше courseId играл роль флага «доска есть». С пробной доской источников два.
  const hasBoard = Boolean(courseId) || Boolean(trial)

  // Доска + invite: один общий промис на автооткрытие и на кнопку тулбара.
  // Это чистый HTTP — коннекта LiveKit он не ждёт (см. эффект автооткрытия).
  const boardRef = useRef<Promise<{ board: BoardWithPages; inviteToken: string }> | null>(null)
  const ensureBoard = useCallback(() => {
    if (!boardRef.current) {
      boardRef.current = (async () => {
        const board = trial
          ? await whiteboardApi.getTrialBoard()
          : await whiteboardApi.getBoardByCourse(courseId as string)
        const invite = await whiteboardApi.createInvite(board.id)
        return { board, inviteToken: invite.id }
      })().catch((err) => {
        boardRef.current = null // дать повторить по кнопке
        throw err
      })
    }
    return boardRef.current
  }, [courseId, trial])

  // Сообщить гостю, что доска открыта. Требует Connected: publishData на
  // неподключённой комнате бросает.
  const announceOpen = useCallback(async (inviteToken: string) => {
    const openMsg: DataMessage = { type: 'board-open', board_token: inviteToken }
    await room.localParticipant.publishData(
      new TextEncoder().encode(JSON.stringify(openMsg)),
      { reliable: true },
    )
    await room.localParticipant.setMetadata(
      JSON.stringify({ boardOpen: true, boardToken: inviteToken }),
    )
  }, [room])

  // Tutor: toggle board open/close
  const handleToggle = useCallback(async () => {
    if (!hasBoard) {
      toast.error('Доска недоступна: урок не привязан к курсу')
      return
    }

    if (mode === 'board') {
      const closeMsg: DataMessage = { type: 'board-close' }
      try {
        await room.localParticipant.publishData(
          new TextEncoder().encode(JSON.stringify(closeMsg)),
          { reliable: true },
        )
        await room.localParticipant.setMetadata(JSON.stringify({ boardOpen: false }))
      } catch {
        toast.error('Не удалось закрыть доску')
      } finally {
        setMode('call')
      }
      return
    }

    try {
      const { board, inviteToken } = await ensureBoard()
      setTutorBoard(board)
      setBoardFailed(false)
      await announceOpen(inviteToken)
      setMode('board')
    } catch (err) {
      console.error('[CallRoom] handleToggle error:', err)
      toast.error('Не удалось открыть доску')
    }
  }, [hasBoard, mode, room, ensureBoard, announceOpen])

  // Tutor: доска — основной режим урока, тянем её сразу на маунте, параллельно
  // с коннектом LiveKit, и показываем как только пришла — ждать WebRTC незачем.
  // Урок без курса — доски нет, молча остаёмся в сетке камер.
  useEffect(() => {
    if (role !== 'tutor' || !hasBoard) return
    let cancelled = false
    ensureBoard()
      .then(({ board }) => {
        if (cancelled) return
        setTutorBoard(board)
        setMode('board')
      })
      .catch(() => {
        if (cancelled) return
        setBoardFailed(true)
        toast.error('Не удалось открыть доску')
      })
    return () => { cancelled = true }
  }, [role, hasBoard, ensureBoard])

  // Гостю о доске сообщаем по факту коннекта — отдельно от её загрузки.
  const connectionState = useConnectionState()
  const announcedRef = useRef(false)
  useEffect(() => {
    if (role !== 'tutor' || mode !== 'board' || announcedRef.current) return
    if (connectionState !== ConnectionState.Connected) return
    announcedRef.current = true
    ensureBoard()
      .then(({ inviteToken }) => announceOpen(inviteToken))
      .catch(() => {})
  }, [role, mode, connectionState, ensureBoard, announceOpen])

  // Чанк Excalidraw ~тяжёлый: греем его сразу, пока идёт коннект, чтобы
  // dynamic() отрисовался мгновенно. Гостю тоже — доску ему откроет препод.
  useEffect(() => {
    if (hasBoard || role === 'guest') void import('@/components/whiteboard/ExcalidrawCanvas')
  }, [hasBoard, role])

  // Resolve which board + page to render
  const activeBoard = role === 'tutor' ? tutorBoard : guestBoard
  const activeBoardToken = role === 'guest' ? guestBoardToken ?? undefined : undefined
  const currentPage = activeBoard?.pages[0] ?? null

  // Доска у препода вот-вот откроется — не мигаем сеткой камер по дороге.
  const boardPending = role === 'tutor' && hasBoard && !tutorBoard && !boardFailed

  return (
    <div style={{ position: 'relative', width: '100%', height: '100%', overflow: 'hidden' }}>
      {/* Область сцены/доски ужимается при открытом чате */}
      <div
        style={{
          position: 'absolute', top: 0, left: 0, bottom: 0,
          right: chatOpen ? 280 : 0, transition: 'right .25s ease',
        }}
      >
        {mode === 'call' && (boardPending ? <BoardPlaceholder /> : <CallStage />)}

        {mode === 'board' && activeBoard && currentPage && (
          <ExcalidrawCanvas
            page={currentPage}
            boardId={activeBoard.id}
            token={activeBoardToken}
            courseId={role === 'tutor' ? courseId ?? undefined : undefined}
            isGuest={role === 'guest'}
            identity={identity}
            onApi={setBoardApi}
            hideUserList
          />
        )}

        {mode === 'board' && (
          <CallParticipants excalidrawApi={boardApi} identity={identity} />
        )}
      </div>

      {chatOpen && (
        <CallChat
          theme={theme}
          messages={messages}
          onSend={send}
          onClose={() => setChatOpen(false)}
        />
      )}

      <CallToolbar
        role={role}
        inviteUrl={inviteUrl}
        boardActive={mode === 'board'}
        chatActive={chatOpen}
        chatUnread={unread}
        showHomework={!trial}
        onToggleBoard={handleToggle}
        onToggleChat={() => setChatOpen((v) => !v)}
        onHomework={() => setHomeworkOpen(true)}
        onLeave={() => room.disconnect()}
      />

      {role === 'tutor' && courseId && (
        <HomeworkEditDialog open={homeworkOpen} onClose={() => setHomeworkOpen(false)} courseId={courseId} />
      )}
      {role === 'guest' && !trial && (
        <HomeworkViewDialog open={homeworkOpen} onClose={() => setHomeworkOpen(false)} />
      )}
    </div>
  )
}

// Пустой холст цвета доски, пока она грузится: подмена сетки камер на пару
// сотен миллисекунд смотрится спокойнее, чем два переключения экрана.
function BoardPlaceholder() {
  return <div style={{ width: '100%', height: '100%', background: '#fff' }} />
}

// ─── Public component ────────────────────────────────────────────────────────

export interface CallRoomProps {
  courseId?: string
  serverUrl: string
  token: string
  role: 'tutor' | 'guest'
  enableMedia?: boolean
  inviteUrl?: string
  /** Пробный урок: доска берётся из общей trial-доски препода, курса нет. */
  trial?: boolean
  onDisconnected: () => void
}

export function CallRoom({
  courseId,
  serverUrl,
  token,
  role,
  enableMedia = false,
  inviteUrl,
  trial,
  onDisconnected,
}: CallRoomProps) {
  return (
    <LiveKitRoom
      key={token}
      serverUrl={serverUrl}
      token={token}
      video={enableMedia}
      audio={enableMedia}
      onDisconnected={onDisconnected}
      data-lk-theme="default"
      style={{ height: '100%' }}
    >
      <RoomAudioRenderer />
      <CallRoomInner courseId={courseId} role={role} inviteUrl={inviteUrl} trial={trial} />
    </LiveKitRoom>
  )
}
