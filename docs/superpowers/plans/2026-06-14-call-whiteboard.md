# Call + Whiteboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Embed a collaborative tldraw whiteboard into the LiveKit video call — tutor clicks "Открыть доску", both participants switch to PiP camera mode with the shared board, signalled via LiveKit DataChannels.

**Architecture:** New `src/components/call/` folder with 4 components (`VideoGrid`, `PipCameras`, `BoardToggleButton`, `CallRoom`). Both tutor page (`/lessons/[id]/call`) and student page (`/join/[lessonId]`) replace `<VideoConference>` with `<CallRoom role="tutor"|"guest">`. Board access uses the existing board invite token mechanism (`BoardInvite.id`), sent via LiveKit DataChannel `publishData`.

**Tech Stack:** `@livekit/components-react` 2.9 (`useTracks`, `GridLayout`, `ParticipantTile`, `ControlBar`, `RoomAudioRenderer`, `useRoomContext`), `livekit-client` 2.x (`RoomEvent.DataReceived`, `publishData`, `setMetadata`), `@tldraw/tldraw` (existing `TldrawCanvas`), existing `lessonsApi`, `whiteboardApi`.

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `src/components/call/VideoGrid.tsx` | Full-screen camera grid for call mode |
| Create | `src/components/call/PipCameras.tsx` | Fixed-position PiP overlays for board mode |
| Create | `src/components/call/BoardToggleButton.tsx` | Open/close board button (tutor only) |
| Create | `src/components/call/CallRoom.tsx` | Orchestrator: mode state, DataChannel, board token |
| Modify | `src/app/(call)/lessons/[id]/call/page.tsx` | Replace `VideoConference` with `CallRoom` |
| Modify | `src/app/join/[lessonId]/page.tsx` | Replace `VideoConference` with `CallRoom` |

---

## Task 1: VideoGrid component

**Files:**
- Create: `src/components/call/VideoGrid.tsx`

- [ ] **Step 1: Create the component**

```tsx
'use client'

import { useEffect } from 'react'
import {
  GridLayout,
  ParticipantTile,
  ControlBar,
  RoomAudioRenderer,
  useTracks,
  useRoomContext,
} from '@livekit/components-react'
import { Track } from 'livekit-client'

export function VideoGrid() {
  const room = useRoomContext()
  const tracks = useTracks(
    [
      { source: Track.Source.Camera, withPlaceholder: true },
      { source: Track.Source.ScreenShare, withPlaceholder: false },
    ],
    { onlySubscribed: false },
  )

  useEffect(() => {
    room.localParticipant.enableCameraAndMicrophone().catch(() => {})
  }, [room])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <GridLayout tracks={tracks} style={{ flex: 1 }}>
        <ParticipantTile />
      </GridLayout>
      <ControlBar />
      <RoomAudioRenderer />
    </div>
  )
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/dragonbrn/tutorgo/frontend && npx tsc --noEmit 2>&1 | head -30
```

Expected: no errors related to `VideoGrid.tsx` (other pre-existing errors are ok).

- [ ] **Step 3: Commit**

```bash
git add src/components/call/VideoGrid.tsx
git commit -m "feat: add VideoGrid component for call mode"
```

---

## Task 2: PipCameras component

**Files:**
- Create: `src/components/call/PipCameras.tsx`

- [ ] **Step 1: Create the component**

```tsx
'use client'

import { ParticipantTile, useTracks } from '@livekit/components-react'
import { Track } from 'livekit-client'

export function PipCameras() {
  const tracks = useTracks(
    [{ source: Track.Source.Camera, withPlaceholder: true }],
    { onlySubscribed: false },
  )

  return (
    <div
      style={{
        position: 'absolute',
        top: 12,
        right: 12,
        zIndex: 50,
        display: 'flex',
        flexDirection: 'column',
        gap: 8,
        pointerEvents: 'auto',
      }}
    >
      {tracks.map((track) => (
        <ParticipantTile
          key={`${track.participant.identity}-${track.source}`}
          trackRef={track}
          style={{
            width: 120,
            height: 80,
            borderRadius: 8,
            overflow: 'hidden',
            flexShrink: 0,
          }}
        />
      ))}
    </div>
  )
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/dragonbrn/tutorgo/frontend && npx tsc --noEmit 2>&1 | head -30
```

Expected: no new errors.

- [ ] **Step 3: Commit**

```bash
git add src/components/call/PipCameras.tsx
git commit -m "feat: add PipCameras component for board mode overlay"
```

---

## Task 3: BoardToggleButton component

**Files:**
- Create: `src/components/call/BoardToggleButton.tsx`

- [ ] **Step 1: Create the component**

```tsx
'use client'

import { LayoutGrid, LayoutPanelTop } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface BoardToggleButtonProps {
  mode: 'call' | 'board'
  loading: boolean
  onToggle: () => void
}

export function BoardToggleButton({ mode, loading, onToggle }: BoardToggleButtonProps) {
  return (
    <Button
      variant="secondary"
      size="sm"
      onClick={onToggle}
      disabled={loading}
      style={{
        position: 'absolute',
        top: 12,
        left: 12,
        zIndex: 50,
      }}
    >
      {mode === 'call' ? (
        <>
          <LayoutPanelTop className="h-4 w-4 mr-2" />
          Открыть доску
        </>
      ) : (
        <>
          <LayoutGrid className="h-4 w-4 mr-2" />
          Закрыть доску
        </>
      )}
    </Button>
  )
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/dragonbrn/tutorgo/frontend && npx tsc --noEmit 2>&1 | head -30
```

Expected: no new errors.

- [ ] **Step 3: Commit**

```bash
git add src/components/call/BoardToggleButton.tsx
git commit -m "feat: add BoardToggleButton component"
```

---

## Task 4: CallRoom component

This is the main orchestrator. Split into two functions in one file: `CallRoom` (wraps `<LiveKitRoom>`) and `CallRoomInner` (uses LiveKit hooks that require being inside a room provider).

**Files:**
- Create: `src/components/call/CallRoom.tsx`

**Key types from the codebase:**
- `BoardInvite.id` (string) — this IS the invite token used for both WS auth and board fetch
- `Lesson.course_id` (string) — from `lessonsApi.get(lessonId)`
- `BoardWithPages` — returned by `whiteboardApi.getBoardByCourse` and `whiteboardApi.joinByInvite`

- [ ] **Step 1: Create CallRoom.tsx**

```tsx
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
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/dragonbrn/tutorgo/frontend && npx tsc --noEmit 2>&1 | head -40
```

Expected: no new errors from `CallRoom.tsx` or its imports.

- [ ] **Step 3: Commit**

```bash
git add src/components/call/CallRoom.tsx
git commit -m "feat: add CallRoom orchestrator with DataChannel board signalling"
```

---

## Task 5: Update tutor call page

The existing page renders `<VideoConference>` inside `<LiveKitRoom>`. Replace that block with `<CallRoom>`. The copy link button moves to `bottom: 80px` to avoid overlapping PiP cameras.

**Files:**
- Modify: `src/app/(call)/lessons/[id]/call/page.tsx`

- [ ] **Step 1: Replace the in-room render block**

Current block (lines 100–126):
```tsx
return (
  <div style={{ height: '100dvh', position: 'relative' }}>
    <LiveKitRoom
      key={room.token}
      serverUrl={room.server_url}
      token={room.token}
      onDisconnected={handleDisconnected}
      data-lk-theme="default"
      style={{ height: '100%' }}
    >
      <VideoConferenceBoundary>
        <VideoConference />
      </VideoConferenceBoundary>
    </LiveKitRoom>

    <Button
      variant="secondary"
      size="icon"
      onClick={handleCopyLink}
      title="Скопировать ссылку для ученика"
      style={{ position: 'absolute', top: '12px', right: '12px', zIndex: 50 }}
    >
      {copied ? <Check className="h-4 w-4" /> : <Link className="h-4 w-4" />}
    </Button>
  </div>
)
```

Replace with:
```tsx
return (
  <div style={{ height: '100dvh', position: 'relative' }}>
    <CallRoom
      lessonId={id}
      serverUrl={room.server_url}
      token={room.token}
      role="tutor"
      onDisconnected={handleDisconnected}
    />
    <Button
      variant="secondary"
      size="icon"
      onClick={handleCopyLink}
      title="Скопировать ссылку для ученика"
      style={{ position: 'absolute', bottom: '80px', right: '12px', zIndex: 60 }}
    >
      {copied ? <Check className="h-4 w-4" /> : <Link className="h-4 w-4" />}
    </Button>
  </div>
)
```

- [ ] **Step 2: Update imports**

Remove `LiveKitRoom, VideoConference` from `@livekit/components-react` import (they're no longer used).
Add import for `CallRoom`:
```tsx
import { CallRoom } from '@/components/call/CallRoom'
```

Remove the `VideoConferenceBoundary` class (it's no longer needed).

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /home/dragonbrn/tutorgo/frontend && npx tsc --noEmit 2>&1 | head -40
```

Expected: no errors in `call/page.tsx`.

- [ ] **Step 4: Commit**

```bash
git add src/app/\(call\)/lessons/\[id\]/call/page.tsx
git commit -m "feat: replace VideoConference with CallRoom on tutor call page"
```

---

## Task 6: Update student join page

The student page (`/join/[lessonId]`) renders `<LiveKitRoom>` + `<VideoConference>` when `stage === 'in-room'`. Replace that block with `<CallRoom role="guest" enableMedia>`.

**Files:**
- Modify: `src/app/join/[lessonId]/page.tsx`

- [ ] **Step 1: Replace the in-room render block**

Current block (lines 78–94):
```tsx
if (stage === 'in-room' && room) {
  return (
    <div style={{ height: '100dvh' }}>
      <LiveKitRoom
        serverUrl={room.server_url}
        token={room.token}
        video={true}
        audio={true}
        onDisconnected={handleDisconnected}
        data-lk-theme="default"
        style={{ height: '100%' }}
      >
        <VideoConference />
      </LiveKitRoom>
    </div>
  )
}
```

Replace with:
```tsx
if (stage === 'in-room' && room) {
  return (
    <div style={{ height: '100dvh' }}>
      <CallRoom
        lessonId={lessonId}
        serverUrl={room.server_url}
        token={room.token}
        role="guest"
        enableMedia
        onDisconnected={handleDisconnected}
      />
    </div>
  )
}
```

- [ ] **Step 2: Update imports**

Remove `LiveKitRoom, VideoConference` from `@livekit/components-react` (no longer used directly).
Add:
```tsx
import { CallRoom } from '@/components/call/CallRoom'
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /home/dragonbrn/tutorgo/frontend && npx tsc --noEmit 2>&1 | head -40
```

Expected: no errors in `join/[lessonId]/page.tsx`.

- [ ] **Step 4: Commit**

```bash
git add src/app/join/\[lessonId\]/page.tsx
git commit -m "feat: replace VideoConference with CallRoom on student join page"
```

---

## Task 7: Manual testing

- [ ] **Step 1: Start the dev server**

```bash
cd /home/dragonbrn/tutorgo/frontend && npm run dev
```

- [ ] **Step 2: Test tutor call — call mode**

1. Open a lesson and click "Начать урок"
2. Verify cameras appear in a grid (VideoGrid)
3. Verify mic/camera controls appear (ControlBar)
4. Verify "Открыть доску" button is visible in the top-left

- [ ] **Step 3: Test tutor call — board mode**

1. Click "Открыть доску"
2. Verify tldraw board opens full-screen
3. Verify camera PiP windows appear top-right (max 120×80px each)
4. Verify "Закрыть доску" button is visible top-left
5. Click "Закрыть доску" → verify grid returns, PiP disappears

- [ ] **Step 4: Test student join — board mode triggered by tutor**

1. Open two browser tabs: tutor in `/lessons/:id/call`, student in `/join/:lessonId`
2. Both connect to the call
3. Tutor clicks "Открыть доску"
4. Verify student's UI automatically switches to board mode with PiP cameras
5. Both draw on the board — verify changes sync in real time
6. Tutor clicks "Закрыть доску" — verify student returns to call mode

- [ ] **Step 5: Test late-join board mode**

1. Tutor opens the board
2. Open a new student tab and join the call
3. Verify the new student immediately sees the board (via room metadata)

- [ ] **Step 6: Test error handling when board unavailable**

1. Temporarily modify `whiteboardApi.getBoardByCourse` to throw in the browser console (or use a courseId that has no board in DB)
2. Click "Открыть доску"
3. Verify a toast "Не удалось открыть доску" appears and mode stays `call`
4. Revert any temporary change
