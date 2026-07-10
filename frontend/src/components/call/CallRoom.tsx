'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import dynamic from 'next/dynamic'
import {
  LiveKitRoom,
  RoomAudioRenderer,
  useRoomContext,
} from '@livekit/components-react'
import { RoomEvent } from 'livekit-client'
import '@livekit/components-styles'
import { toast } from 'sonner'

import { useTheme } from 'next-themes'
import { PipCameras } from './PipCameras'
import { CallStage } from './CallStage'
import { CallToolbar } from './CallToolbar'
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
}

function CallRoomInner({ courseId, role, inviteUrl }: CallRoomInnerProps) {
  const room = useRoomContext()
  const { resolvedTheme } = useTheme()
  const theme = themeTokens(resolvedTheme === 'dark' ? 'dark' : 'light')
  const [mode, setMode] = useState<Mode>('call')
  const [boardLoading, setBoardLoading] = useState(false)
  const [activePageId, setActivePageId] = useState<string | null>(null)
  const [chatOpen, setChatOpen] = useState(false)
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
  const inviteTokenRef = useRef<string | null>(null)

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
        setActivePageId(null)
      }
    }

    room.on(RoomEvent.DataReceived, onData)
    return () => { room.off(RoomEvent.DataReceived, onData) }
  }, [room, role])

  // Tutor: toggle board open/close
  const handleToggle = useCallback(async () => {
    if (!courseId) {
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
        setActivePageId(null)
      }
      return
    }

    setBoardLoading(true)
    try {
      const board = tutorBoard ?? await whiteboardApi.getBoardByCourse(courseId)
      if (!tutorBoard) setTutorBoard(board)

      let inviteToken = inviteTokenRef.current
      if (!inviteToken) {
        const invite = await whiteboardApi.createInvite(board.id)
        inviteToken = invite.id
        inviteTokenRef.current = inviteToken
      }

      const openMsg: DataMessage = { type: 'board-open', board_token: inviteToken }
      await room.localParticipant.publishData(
        new TextEncoder().encode(JSON.stringify(openMsg)),
        { reliable: true },
      )
      await room.localParticipant.setMetadata(
        JSON.stringify({ boardOpen: true, boardToken: inviteToken }),
      )
      setMode('board')
    } catch (err) {
      console.error('[CallRoom] handleToggle error:', err)
      toast.error('Не удалось открыть доску')
    } finally {
      setBoardLoading(false)
    }
  }, [courseId, mode, tutorBoard, room])

  // Resolve which board + page to render
  const activeBoard = role === 'tutor' ? tutorBoard : guestBoard
  const activeBoardToken = role === 'guest' ? guestBoardToken ?? undefined : undefined
  const resolvedPageId = activePageId ?? activeBoard?.pages[0]?.id ?? null
  const currentPage = activeBoard?.pages.find((p) => p.id === resolvedPageId) ?? null

  return (
    <div style={{ position: 'relative', width: '100%', height: '100%', overflow: 'hidden' }}>
      {/* Область сцены/доски ужимается при открытом чате */}
      <div
        style={{
          position: 'absolute', top: 0, left: 0, bottom: 0,
          right: chatOpen ? 280 : 0, transition: 'right .25s ease',
        }}
      >
        {mode === 'call' && <CallStage />}

        {mode === 'board' && activeBoard && resolvedPageId && (
          <ExcalidrawCanvas
            page={currentPage}
            boardId={activeBoard.id}
            pages={activeBoard.pages}
            activePageId={resolvedPageId}
            onSelectPage={setActivePageId}
            token={activeBoardToken}
            courseId={role === 'tutor' ? courseId ?? undefined : undefined}
            isGuest={role === 'guest'}
          />
        )}

        {mode === 'board' && <PipCameras chatOpen={chatOpen} />}
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
        onToggleBoard={handleToggle}
        onToggleChat={() => setChatOpen((v) => !v)}
        onLeave={() => room.disconnect()}
      />
    </div>
  )
}

// ─── Public component ────────────────────────────────────────────────────────

export interface CallRoomProps {
  courseId?: string
  serverUrl: string
  token: string
  role: 'tutor' | 'guest'
  enableMedia?: boolean
  inviteUrl?: string
  onDisconnected: () => void
}

export function CallRoom({
  courseId,
  serverUrl,
  token,
  role,
  enableMedia = false,
  inviteUrl,
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
      <CallRoomInner courseId={courseId} role={role} inviteUrl={inviteUrl} />
    </LiveKitRoom>
  )
}
