# Call + Whiteboard Feature Design

**Date:** 2026-06-14
**Status:** Approved

## Overview

Allow tutor and student to open a collaborative whiteboard during a LiveKit video call without leaving the call. When the tutor clicks "Открыть доску", both participants' cameras shrink to Picture-in-Picture overlays in the top-right corner and the tldraw board expands to fill the screen. The tutor triggers this transition for the student via LiveKit DataChannels — no additional backend infrastructure required.

## User Flow

1. Tutor opens `/lessons/[id]/call` and starts a lesson (existing flow).
2. Student joins via `/join/[lessonId]` (existing flow).
3. Both see a standard video call (cameras filling screen).
4. Tutor clicks **"Открыть доску"** → cameras shrink to PiP (top-right) → tldraw board opens. Student's UI mirrors this transition automatically.
5. Both can draw on the shared board simultaneously.
6. Tutor clicks **"Закрыть доску"** → PiP cameras expand back to full call. Student follows.

## Architecture

### Component Tree

```
CallRoom (new, role="tutor"|"guest")
└─ LiveKitRoom
   ├─ [mode=call]  VideoGrid      (new) — full-screen camera grid
   ├─ [mode=board] TldrawCanvas   (existing) — full-screen board
   ├─ [mode=board] PipCameras     (new) — floating camera overlay
   └─ BoardToggleButton           (new, tutor-only)
```

### Modified Pages

| File | Change |
|------|--------|
| `app/(call)/lessons/[id]/call/page.tsx` | Replace `<VideoConference>` with `<CallRoom role="tutor">` |
| `app/join/[lessonId]/page.tsx` | Replace `<VideoConference>` with `<CallRoom role="guest">` |

### New Files

```
src/components/call/
  CallRoom.tsx          — orchestrator: mode state, DataChannel, board token
  VideoGrid.tsx         — full-screen LiveKit video grid (replaces VideoConference)
  PipCameras.tsx        — fixed-position PiP camera overlays
  BoardToggleButton.tsx — open/close board button (tutor only)
```

## Component Interfaces

```ts
// CallRoom.tsx
interface CallRoomProps {
  lessonId: string
  serverUrl: string
  token: string          // LiveKit token
  role: 'tutor' | 'guest'
  onDisconnected: () => void
}

// Internal state
type Mode = 'call' | 'board'
```

`CallRoom` manages:
- `mode: Mode` — drives which sub-components render
- `boardToken: string | null` — board invite token (fetched when board opens)
- `courseId: string | null` — resolved from `lessonsApi.get(lessonId)` on mount (tutor only)

## DataChannel Protocol

Two JSON events transmitted as `Uint8Array` via `room.localParticipant.publishData()`:

```jsonc
{ "type": "board-open",  "board_token": "<invite-uuid>" }
{ "type": "board-close" }
```

**Tutor side (send):**
1. User clicks toggle → if `inviteToken` already cached in state, reuse it; otherwise call `whiteboardApi.createInvite(boardId)` → `{token}` and cache it for the session
2. Publish `board-open` with `board_token` to all participants
3. Set `mode = 'board'`, `boardToken = token`

**Student side (receive):**
- Listen via `RoomEvent.DataReceived`
- On `board-open`: save `boardToken`, set `mode = 'board'`
- On `board-close`: set `mode = 'call'`, clear `boardToken`

**Late-join handling:** When tutor opens the board, call `room.localParticipant.setMetadata(JSON.stringify({boardOpen: true, boardToken: "..."}))`. Students joining after `board-open` read room metadata on connect and immediately open the board.

## Board Access Flow

### Tutor
```
mount → lessonsApi.get(lessonId) → courseId
toggle open → whiteboardApi.getBoardByCourse(courseId) → board.id
           → whiteboardApi.createInvite(board.id) → invite.token
           → publishData({type:"board-open", board_token: invite.token})
           → <TldrawCanvas boardId={board.id} page={...} isGuest={false} />
```

### Student (guest)
```
receive DataReceived({type:"board-open", board_token}) →
  boardToken = board_token
  <TldrawCanvas ... token={boardToken} isGuest={true} />
```

`TldrawCanvas` already accepts `token` for guest WS auth (passes it to `getWsUrl(pageId, token)`). `useJoinByInvite(boardToken)` fetches board structure via `GET /public/board/join/:token`. No backend changes needed.

## VideoGrid Component

Uses `@livekit/components-react` hooks:

```ts
const tracks = useTracks([
  { source: Track.Source.Camera, withPlaceholder: true },
])
// Render <ParticipantTile> per track in CSS Grid
```

Replaces the black-box `<VideoConference>` with a layout we control, enabling the PiP transition.

## PipCameras Component

Same `useTracks` call, but renders as:
```css
position: fixed;
top: 12px;
right: 12px;
z-index: 50;
display: flex;
flex-direction: column;
gap: 8px;
```

Each tile: 120×80px, `border-radius: 8px`, `box-shadow`. No drag-and-drop.

## BoardToggleButton Component

- Shown only when `role === 'tutor'`
- Positioned `fixed`, top-left, `z-index: 50`
- Label toggles: "Открыть доску" / "Закрыть доску"
- Disabled state while board is loading (between click and WS connect)

## Edge Cases

| Situation | Behavior |
|-----------|----------|
| Student joins after `board-open` | Reads `boardOpen: true` + `boardToken` from room metadata, opens board immediately |
| Board has no invite yet | `createInvite` is called; backend upserts one invite per board |
| `createInvite` fails | Toast error, stay in `call` mode |
| Tutor disconnects mid-board | Student sees LiveKit disconnected state (existing behavior via `onDisconnected`) |
| Lesson has no associated board (no courseId) | `lessonsApi.get` returns lesson without `course_id` → toggle button hidden |

## No Backend Changes Required

All needed APIs exist:
- `GET /lessons/:id` → `Lesson.course_id`
- `GET /boards/course/:courseId` → `BoardWithPages`
- `POST /boards/:boardId/invite` → `BoardInvite.token`
- `GET /public/board/join/:token` → `BoardWithPages` (guest read)
- `WS /ws/board/:pageId?token=:inviteToken` → real-time sync (guest)
- LiveKit DataChannels → built into existing LiveKit session
