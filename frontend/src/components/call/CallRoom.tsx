'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import {
  LiveKitRoom,
  RoomAudioRenderer,
  useRoomContext,
} from '@livekit/components-react'
import { RoomEvent } from 'livekit-client'
import '@livekit/components-styles'
import { toast } from 'sonner'

import { VideoGrid } from './VideoGrid'
import { PipCameras } from './PipCameras'
import { BoardToggleButton } from './BoardToggleButton'
import { TldrawCanvas } from '@/components/whiteboard/TldrawCanvas'
import { lessonsApi } from '@/lib/api/lessons'
import { whiteboardApi } from '@/lib/api/whiteboard'
import type { BoardWithPages } from '@/types/api'

type Mode = 'call' | 'board'

interface DataMessage {
  type: 'board-open' | 'board-close'
  board_token?: string
}

// ─── Inner component (must live inside <LiveKitRoom>) ───────────────────────

interface CallRoomInnerProps {
  lessonId: string
  role: 'tutor' | 'guest'
}

function CallRoomInner({ lessonId, role }: CallRoomInnerProps) {
  const room = useRoomContext()
  const [mode, setMode] = useState<Mode>('call')
  const [boardLoading, setBoardLoading] = useState(false)
  const [activePageId, setActivePageId] = useState<string | null>(null)

  // Tutor state
  const [courseId, setCourseId] = useState<string | null>(null)
  const [tutorBoard, setTutorBoard] = useState<BoardWithPages | null>(null)
  const inviteTokenRef = useRef<string | null>(null)

  // Guest state
  const [guestBoardToken, setGuestBoardToken] = useState<string | null>(null)
  const [guestBoard, setGuestBoard] = useState<BoardWithPages | null>(null)

  // Tutor: resolve courseId once on mount
  useEffect(() => {
    if (role !== 'tutor') return
    lessonsApi.get(lessonId).then((lesson) => {
      if (lesson.course_id) setCourseId(lesson.course_id)
    }).catch(() => {})
  }, [lessonId, role])

  // Guest: handle board-open sent before this participant joined (room metadata)
  useEffect(() => {
    if (role !== 'guest') return
    let meta: { boardOpen?: boolean; boardToken?: string } | null = null
    try { meta = room.metadata ? JSON.parse(room.metadata) : null } catch {}
    if (!meta?.boardOpen || !meta.boardToken) return
    whiteboardApi.joinByInvite(meta.boardToken).then((data) => {
      setGuestBoardToken(meta!.boardToken!)
      setGuestBoard(data)
      setMode('board')
    }).catch(() => {})
  }, [room.metadata, role])

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

  // Tutor: toggle board open/close
  const handleToggle = useCallback(async () => {
    if (!courseId) return

    if (mode === 'board') {
      const closeMsg: DataMessage = { type: 'board-close' }
      room.localParticipant.publishData(
        new TextEncoder().encode(JSON.stringify(closeMsg)),
        { reliable: true },
      )
      await room.localParticipant.setMetadata(JSON.stringify({ boardOpen: false }))
      setMode('call')
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
      room.localParticipant.publishData(
        new TextEncoder().encode(JSON.stringify(openMsg)),
        { reliable: true },
      )
      await room.localParticipant.setMetadata(
        JSON.stringify({ boardOpen: true, boardToken: inviteToken }),
      )
      setMode('board')
    } catch {
      toast.error('Не удалось открыть доску')
    } finally {
      setBoardLoading(false)
    }
  }, [courseId, mode, tutorBoard, room])

  // Resolve which board + page to render
  const activeBoard = role === 'tutor' ? tutorBoard : guestBoard
  const activeBoardToken = role === 'guest' ? guestBoardToken ?? undefined : undefined
  const resolvedPageId = activePageId ?? activeBoard?.pages[0]?.id ?? ''
  const currentPage = activeBoard?.pages.find((p) => p.id === resolvedPageId) ?? null

  return (
    <div style={{ position: 'relative', width: '100%', height: '100%' }}>
      {mode === 'call' && <VideoGrid />}

      {mode === 'board' && activeBoard && (
        <TldrawCanvas
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

      {mode === 'board' && <PipCameras />}

      {role === 'tutor' && !!courseId && (
        <BoardToggleButton
          mode={mode}
          loading={boardLoading}
          onToggle={handleToggle}
        />
      )}
    </div>
  )
}

// ─── Public component ────────────────────────────────────────────────────────

export interface CallRoomProps {
  lessonId: string
  serverUrl: string
  token: string
  role: 'tutor' | 'guest'
  enableMedia?: boolean
  onDisconnected: () => void
}

export function CallRoom({
  lessonId,
  serverUrl,
  token,
  role,
  enableMedia = false,
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
      <CallRoomInner lessonId={lessonId} role={role} />
    </LiveKitRoom>
  )
}
