# Quick Room Button Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить кнопку «Начать урок» в сайдбар, которая создаёт одноразовую LiveKit-комнату без привязки к уроку из БД.

**Architecture:** Состояние быстрых комнат хранится в in-memory map (`map[string]*quickRoom` + `sync.RWMutex`) внутри `CallHandler` — без миграций. Токен передаётся на страницу через `sessionStorage`. Ученик заходит через отдельный публичный маршрут `/join/room/[id]`.

**Tech Stack:** Go + Gin (бэкенд), Next.js 15 App Router + TypeScript (фронтенд), LiveKit (`@livekit/components-react`), Tailwind CSS.

---

## Файловая карта

| Статус | Файл | Роль |
|--------|------|------|
| Изменить | `handlers/call.go` | +struct `quickRoom`, +4 метода, +поля в `CallHandler` |
| Изменить | `router/router.go` | +4 маршрута |
| Изменить | `frontend/src/lib/api/calls.ts` | +interface `QuickRoomResponse`, +4 функции |
| Создать | `frontend/src/app/api/quick-status/[roomId]/route.ts` | Next.js прокси → `/public/quick/:id/status` |
| Создать | `frontend/src/app/api/quick-guest-token/[roomId]/route.ts` | Next.js прокси → `/public/quick/:id/guest-token` |
| Создать | `frontend/src/app/(dashboard)/room/[id]/page.tsx` | Страница репетитора: sessionStorage → LiveKit |
| Создать | `frontend/src/app/join/room/[id]/page.tsx` | Страница ученика: polling → LiveKit |
| Изменить | `frontend/src/components/layout/Sidebar.tsx` | Кнопка в `CalendarSidebarPanel` |

---

## Task 1: Бэкенд — quick room хэндлер

**Files:**
- Modify: `handlers/call.go`

- [ ] **Step 1: Добавить struct и поля в `CallHandler`**

В `handlers/call.go` заменить импорты и struct:

```go
import (
    "fmt"
    "log/slog"
    "net/http"
    "sync"
    "time"

    "tutorgo/service"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    lkauth "github.com/livekit/protocol/auth"
    livekit "github.com/livekit/protocol/livekit"
    lksdk "github.com/livekit/server-sdk-go/v2"
)

type quickRoom struct {
    tutorID string
    active  bool
}

type CallHandler struct {
    lessonService service.LessonService
    log           *slog.Logger
    livekitURL    string
    apiKey        string
    apiSecret     string
    roomClient    *lksdk.RoomServiceClient

    quickMu    sync.RWMutex
    quickRooms map[string]*quickRoom
}
```

Обновить `NewCallHandler` — добавить инициализацию map:

```go
func NewCallHandler(svc service.LessonService, log *slog.Logger, url, key, secret string) *CallHandler {
    var roomClient *lksdk.RoomServiceClient
    if key != "" {
        roomClient = lksdk.NewRoomServiceClient(url, key, secret)
    }
    return &CallHandler{
        lessonService: svc,
        log:           log,
        livekitURL:    url,
        apiKey:        key,
        apiSecret:     secret,
        roomClient:    roomClient,
        quickRooms:    make(map[string]*quickRoom),
    }
}
```

- [ ] **Step 2: Добавить `StartQuickRoom`**

В конец `handlers/call.go` добавить:

```go
// POST /calls/quick
func (h *CallHandler) StartQuickRoom(c *gin.Context) {
    if h.apiKey == "" {
        c.JSON(http.StatusServiceUnavailable, gin.H{"error": "video calls not configured"})
        return
    }
    tutorID := c.GetString("tutorID")
    if tutorID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    roomID := uuid.New().String()
    roomName := "quick-" + roomID

    h.quickMu.Lock()
    h.quickRooms[roomID] = &quickRoom{tutorID: tutorID, active: true}
    h.quickMu.Unlock()

    canPublish := true
    canSubscribe := true
    at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
    grant := &lkauth.VideoGrant{
        RoomJoin:     true,
        Room:         roomName,
        CanPublish:   &canPublish,
        CanSubscribe: &canSubscribe,
    }
    at.SetVideoGrant(grant).
        SetIdentity("tutor-" + tutorID).
        SetName("Репетитор").
        SetValidFor(3 * time.Hour)

    token, err := at.ToJWT()
    if err != nil {
        h.log.Error("Failed to generate quick room token", slog.String("error", err.Error()))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "room_id":    roomID,
        "token":      token,
        "server_url": h.livekitURL,
    })
}
```

- [ ] **Step 3: Добавить `EndQuickRoom`**

```go
// POST /calls/quick/:id/end
func (h *CallHandler) EndQuickRoom(c *gin.Context) {
    tutorID := c.GetString("tutorID")
    if tutorID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    roomID := c.Param("id")

    h.quickMu.Lock()
    room, ok := h.quickRooms[roomID]
    if ok && room.tutorID == tutorID {
        room.active = false
    }
    h.quickMu.Unlock()

    if !ok {
        c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
        return
    }

    if h.roomClient != nil {
        roomName := "quick-" + roomID
        _, _ = h.roomClient.DeleteRoom(c.Request.Context(), &livekit.DeleteRoomRequest{Room: roomName})
    }

    c.JSON(http.StatusOK, gin.H{"message": "room ended"})
}
```

- [ ] **Step 4: Добавить `GetQuickRoomStatus`**

```go
// GET /public/quick/:id/status
func (h *CallHandler) GetQuickRoomStatus(c *gin.Context) {
    roomID := c.Param("id")

    h.quickMu.RLock()
    room, ok := h.quickRooms[roomID]
    h.quickMu.RUnlock()

    if !ok || !room.active {
        c.JSON(http.StatusOK, gin.H{"status": "ended"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"status": "active"})
}
```

- [ ] **Step 5: Добавить `GetQuickGuestToken`**

```go
// GET /public/quick/:id/guest-token
func (h *CallHandler) GetQuickGuestToken(c *gin.Context) {
    if h.apiKey == "" {
        c.JSON(http.StatusServiceUnavailable, gin.H{"error": "video calls not configured"})
        return
    }
    roomID := c.Param("id")

    h.quickMu.RLock()
    room, ok := h.quickRooms[roomID]
    h.quickMu.RUnlock()

    if !ok || !room.active {
        c.JSON(http.StatusNotFound, gin.H{"error": "room not found or ended"})
        return
    }

    roomName := "quick-" + roomID
    canPublish := true
    canSubscribe := true
    identity := fmt.Sprintf("guest-%d", time.Now().UnixMilli())
    at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
    grant := &lkauth.VideoGrant{
        RoomJoin:     true,
        Room:         roomName,
        CanPublish:   &canPublish,
        CanSubscribe: &canSubscribe,
    }
    at.SetVideoGrant(grant).
        SetIdentity(identity).
        SetName("Ученик").
        SetValidFor(3 * time.Hour)

    token, err := at.ToJWT()
    if err != nil {
        h.log.Error("Failed to generate quick guest token", slog.String("error", err.Error()))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "token":      token,
        "room_name":  roomName,
        "server_url": h.livekitURL,
    })
}
```

- [ ] **Step 6: Проверить компиляцию**

```bash
go build ./...
```

Ожидаемый вывод: пусто (без ошибок).

- [ ] **Step 7: Коммит**

```bash
git add handlers/call.go
git commit -m "feat: add quick room handler (in-memory, no DB)"
```

---

## Task 2: Бэкенд — маршруты

**Files:**
- Modify: `router/router.go`

- [ ] **Step 1: Добавить публичные маршруты**

После строки `r.GET("/public/lessons/:id/room-status", callHandler.GetRoomStatus)` добавить:

```go
r.GET("/public/quick/:id/status", callHandler.GetQuickRoomStatus)
r.GET("/public/quick/:id/guest-token", middleware.RateLimit(rate.Every(3*time.Second), 5), callHandler.GetQuickGuestToken)
```

- [ ] **Step 2: Добавить защищённые маршруты**

После строки `auth.POST("/lessons/:id/end-room", callHandler.EndRoom)` добавить:

```go
auth.POST("/calls/quick", callHandler.StartQuickRoom)
auth.POST("/calls/quick/:id/end", callHandler.EndQuickRoom)
```

- [ ] **Step 3: Проверить компиляцию**

```bash
go build ./...
```

Ожидаемый вывод: пусто.

- [ ] **Step 4: Коммит**

```bash
git add router/router.go
git commit -m "feat: register quick room routes"
```

---

## Task 3: Фронтенд — API клиент + Next.js прокси

**Files:**
- Modify: `frontend/src/lib/api/calls.ts`
- Create: `frontend/src/app/api/quick-status/[roomId]/route.ts`
- Create: `frontend/src/app/api/quick-guest-token/[roomId]/route.ts`

- [ ] **Step 1: Добавить типы и функции в `calls.ts`**

После `RoomStatusResponse` добавить интерфейс:

```typescript
export interface QuickRoomResponse {
  room_id:    string
  token:      string
  server_url: string
}
```

В конец объекта `callsApi` добавить:

```typescript
  startQuickRoom: () =>
    api.post<QuickRoomResponse>('/calls/quick').then((r) => r.data),

  endQuickRoom: (roomId: string) =>
    api.post(`/calls/quick/${roomId}/end`).then((r) => r.data),

  getQuickRoomStatus: (roomId: string) =>
    fetch(`/api/quick-status/${roomId}`)
      .then((r) => { if (!r.ok) throw new Error(); return r.json() as Promise<RoomStatusResponse> }),

  getQuickGuestToken: (roomId: string) =>
    fetch(`/api/quick-guest-token/${roomId}`)
      .then((r) => { if (!r.ok) throw new Error(); return r.json() as Promise<RoomTokenResponse> }),
```

- [ ] **Step 2: Создать прокси для статуса**

Создать файл `frontend/src/app/api/quick-status/[roomId]/route.ts`:

```typescript
import { NextRequest, NextResponse } from 'next/server'

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ roomId: string }> },
) {
  const { roomId } = await params
  const backendURL = process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, '')
  if (!backendURL) {
    return NextResponse.json({ error: 'NEXT_PUBLIC_API_URL not configured' }, { status: 503 })
  }
  try {
    const res = await fetch(`${backendURL}/public/quick/${roomId}/status`, { cache: 'no-store' })
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json({ error: 'backend unavailable' }, { status: 502 })
  }
}
```

- [ ] **Step 3: Создать прокси для гостевого токена**

Создать файл `frontend/src/app/api/quick-guest-token/[roomId]/route.ts`:

```typescript
import { NextRequest, NextResponse } from 'next/server'

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ roomId: string }> },
) {
  const { roomId } = await params
  const backendURL = process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, '')
  if (!backendURL) {
    return NextResponse.json({ error: 'NEXT_PUBLIC_API_URL not configured' }, { status: 503 })
  }
  try {
    const res = await fetch(`${backendURL}/public/quick/${roomId}/guest-token`, { cache: 'no-store' })
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json({ error: 'backend unavailable' }, { status: 502 })
  }
}
```

- [ ] **Step 4: Проверить TypeScript**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```

Ожидаемый вывод: пусто или только предупреждения (не ошибки).

- [ ] **Step 5: Коммит**

```bash
git add frontend/src/lib/api/calls.ts \
        frontend/src/app/api/quick-status \
        frontend/src/app/api/quick-guest-token
git commit -m "feat: add quick room API client + Next.js proxies"
```

---

## Task 4: Фронтенд — страница репетитора `/room/[id]`

**Files:**
- Create: `frontend/src/app/(dashboard)/room/[id]/page.tsx`

Страница читает токен из `sessionStorage` (ключ `quick-room-{id}`), сразу подключается к LiveKit. Если токена нет — редирект на `/dashboard`.

- [ ] **Step 1: Создать страницу**

Создать файл `frontend/src/app/(dashboard)/room/[id]/page.tsx`:

```typescript
'use client'

import { Component, useEffect, useState, type ReactNode } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { LiveKitRoom, VideoConference } from '@livekit/components-react'
import '@livekit/components-styles'

import { callsApi } from '@/lib/api/calls'
import { Check, Copy, Loader2 } from 'lucide-react'

class VideoConferenceBoundary extends Component<
  { children: ReactNode },
  { hasError: boolean }
> {
  state = { hasError: false }
  static getDerivedStateFromError() { return { hasError: true } }
  componentDidCatch() { setTimeout(() => this.setState({ hasError: false }), 0) }
  render() { return this.state.hasError ? null : this.props.children }
}

type Stage = 'loading' | 'in-room' | 'error'

export default function QuickRoomPage() {
  const { id }  = useParams<{ id: string }>()
  const router  = useRouter()

  const [stage, setStage]         = useState<Stage>('loading')
  const [token, setToken]         = useState<string | null>(null)
  const [serverUrl, setServerUrl] = useState<string | null>(null)
  const [copied, setCopied]       = useState(false)

  const joinUrl = typeof window !== 'undefined'
    ? `${window.location.origin}/join/room/${id}`
    : `/join/room/${id}`

  function handleCopy() {
    navigator.clipboard.writeText(joinUrl)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  useEffect(() => {
    const raw = sessionStorage.getItem(`quick-room-${id}`)
    if (!raw) { router.replace('/dashboard'); return }
    try {
      const { token: t, server_url: s } = JSON.parse(raw) as { token: string; server_url: string }
      setToken(t)
      setServerUrl(s)
      setStage('in-room')
    } catch {
      router.replace('/dashboard')
    }
  }, [id, router])

  async function handleDisconnected() {
    try { await callsApi.endQuickRoom(id) } catch {}
    sessionStorage.removeItem(`quick-room-${id}`)
    router.replace('/dashboard')
  }

  if (stage === 'loading') {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '80vh' }}>
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (stage === 'error' || !token || !serverUrl) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '80vh', gap: 16 }}>
        <p className="text-muted-foreground text-sm">Не удалось подключиться к комнате</p>
        <button
          onClick={() => router.replace('/dashboard')}
          style={{ padding: '8px 16px', borderRadius: 8, border: '1px solid var(--border)', cursor: 'pointer', fontSize: 13 }}
        >
          На главную
        </button>
      </div>
    )
  }

  return (
    <div style={{ height: 'calc(100vh - 64px)', position: 'relative' }}>
      <LiveKitRoom
        key={token}
        serverUrl={serverUrl}
        token={token}
        onDisconnected={handleDisconnected}
        data-lk-theme="default"
        style={{ height: '100%' }}
      >
        <VideoConferenceBoundary>
          <VideoConference />
        </VideoConferenceBoundary>
      </LiveKitRoom>

      <button
        onClick={handleCopy}
        title="Скопировать ссылку для ученика"
        style={{
          position: 'absolute', top: 12, right: 12, zIndex: 50,
          display: 'flex', alignItems: 'center', gap: 6,
          padding: '6px 12px', borderRadius: 8,
          background: 'rgba(0,0,0,0.55)', backdropFilter: 'blur(8px)',
          border: '1px solid rgba(255,255,255,0.12)',
          color: '#fff', fontSize: 12, fontWeight: 600, cursor: 'pointer',
        }}
      >
        {copied ? <Check size={13} /> : <Copy size={13} />}
        {copied ? 'Скопировано' : 'Ссылка для ученика'}
      </button>
    </div>
  )
}
```

- [ ] **Step 2: Проверить TypeScript**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```

Ожидаемый вывод: пусто или предупреждения (не ошибки).

- [ ] **Step 3: Коммит**

```bash
git add frontend/src/app/\(dashboard\)/room
git commit -m "feat: add quick room tutor page /room/[id]"
```

---

## Task 5: Фронтенд — страница ученика `/join/room/[id]`

**Files:**
- Create: `frontend/src/app/join/room/[id]/page.tsx`

Полностью аналогична `/join/[lessonId]/page.tsx` — polling каждые 5с, но использует `getQuickRoomStatus` и `getQuickGuestToken`.

- [ ] **Step 1: Создать страницу**

Создать файл `frontend/src/app/join/room/[id]/page.tsx`:

```typescript
'use client'

import { useEffect, useRef, useState } from 'react'
import { useParams } from 'next/navigation'
import { LiveKitRoom, VideoConference } from '@livekit/components-react'
import '@livekit/components-styles'

import { callsApi, type RoomTokenResponse } from '@/lib/api/calls'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { GraduationCap } from 'lucide-react'

type Stage = 'form' | 'waiting' | 'in-room' | 'ended'

export default function JoinRoomPage() {
  const { id }  = useParams<{ id: string }>()

  const [stage, setStage]     = useState<Stage>('form')
  const [name, setName]       = useState('')
  const [room, setRoom]       = useState<RoomTokenResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  function clearPolling() {
    if (intervalRef.current) { clearInterval(intervalRef.current); intervalRef.current = null }
  }
  useEffect(() => () => clearPolling(), [])

  async function tryJoin(): Promise<boolean> {
    try {
      const { status } = await callsApi.getQuickRoomStatus(id)
      if (status === 'ended') { clearPolling(); setLoading(false); setStage('ended'); return true }
      if (status !== 'active') return false
      const data = await callsApi.getQuickGuestToken(id)
      clearPolling(); setLoading(false); setRoom(data); setStage('in-room')
      return true
    } catch { return false }
  }

  async function handleJoin() {
    if (!name.trim()) return
    setLoading(true)
    const joined = await tryJoin()
    if (!joined) { setLoading(false); setStage('waiting'); intervalRef.current = setInterval(tryJoin, 5000) }
  }

  async function handleDisconnected() {
    try {
      const { status } = await callsApi.getQuickRoomStatus(id)
      if (status === 'ended') { setRoom(null); setStage('ended'); return }
    } catch {}
    setRoom(null); setStage('form')
  }

  if (stage === 'in-room' && room) {
    return (
      <div style={{ height: '100dvh' }}>
        <LiveKitRoom serverUrl={room.server_url} token={room.token} video audio
          onDisconnected={handleDisconnected} data-lk-theme="default" style={{ height: '100%' }}>
          <VideoConference />
        </LiveKitRoom>
      </div>
    )
  }

  if (stage === 'ended') {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background p-4">
        <div className="w-full max-w-sm space-y-6">
          <div className="flex items-center gap-2 justify-center">
            <GraduationCap className="h-6 w-6 text-primary" />
            <span className="font-semibold text-lg">TutorGo</span>
          </div>
          <div className="rounded-lg border p-6 text-center space-y-2">
            <p className="font-semibold">Урок завершён</p>
            <p className="text-sm text-muted-foreground">Спасибо за занятие!</p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="flex items-center gap-2 justify-center">
          <GraduationCap className="h-6 w-6 text-primary" />
          <span className="font-semibold text-lg">TutorGo</span>
        </div>
        <div className="rounded-lg border p-6 space-y-4">
          <h1 className="text-base font-semibold">Присоединиться к уроку</h1>
          {stage === 'waiting' ? (
            <p className="text-sm text-muted-foreground text-center py-2">
              Урок ещё не начался. Ожидаем начала...
            </p>
          ) : (
            <>
              <div className="space-y-2">
                <label className="text-sm text-muted-foreground">Ваше имя</label>
                <Input placeholder="Введите ваше имя" value={name}
                  onChange={(e) => setName(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleJoin()} autoFocus />
              </div>
              <Button className="w-full" onClick={handleJoin} disabled={!name.trim() || loading}>
                {loading ? 'Подключение...' : 'Войти в урок'}
              </Button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Проверить TypeScript**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```

Ожидаемый вывод: пусто или предупреждения.

- [ ] **Step 3: Коммит**

```bash
git add frontend/src/app/join/room
git commit -m "feat: add quick room student join page /join/room/[id]"
```

---

## Task 6: Фронтенд — кнопка в сайдбаре

**Files:**
- Modify: `frontend/src/components/layout/Sidebar.tsx`

Кнопка добавляется в `CalendarSidebarPanel` — под `TodayList`, перед закрывающим `</div>`.

- [ ] **Step 1: Добавить импорты в `Sidebar.tsx`**

В существующий блок импортов добавить:

```typescript
import { useRouter } from 'next/navigation'
import { callsApi } from '@/lib/api/calls'
```

- [ ] **Step 2: Добавить состояние и обработчик в `CalendarSidebarPanel`**

В начало функции `CalendarSidebarPanel` добавить:

```typescript
const router = useRouter()
const [starting, setStarting] = useState(false)

async function handleStartLesson() {
  if (starting) return
  setStarting(true)
  try {
    const { room_id, token, server_url } = await callsApi.startQuickRoom()
    sessionStorage.setItem(`quick-room-${room_id}`, JSON.stringify({ token, server_url }))
    router.push(`/room/${room_id}`)
  } catch {
    setStarting(false)
  }
}
```

- [ ] **Step 3: Добавить кнопку в JSX `CalendarSidebarPanel`**

В return `CalendarSidebarPanel` найти закрывающий `</div>` всего компонента и добавить кнопку перед ним:

```tsx
{/* Кнопка быстрого урока */}
<div className="px-2 py-2 border-t border-border shrink-0">
  <button
    onClick={handleStartLesson}
    disabled={starting}
    className="w-full flex items-center gap-2 px-3 py-2 rounded-md text-xs transition-colors border border-border hover:bg-[var(--sidebar-hover-bg)] disabled:opacity-50 disabled:cursor-not-allowed"
    style={{ background: 'var(--sidebar-bg, transparent)' }}
  >
    <span
      style={{
        width: 7, height: 7, borderRadius: '50%', flexShrink: 0,
        background: starting ? '#888' : '#22c55e',
        animation: starting ? 'none' : 'liveDot 2s ease-in-out infinite',
      }}
    />
    <style>{`
      @keyframes liveDot {
        0%,100% { opacity:1; box-shadow: 0 0 0 0 rgba(34,197,94,0.5); }
        50% { opacity:.75; box-shadow: 0 0 0 4px rgba(34,197,94,0); }
      }
    `}</style>
    <span style={{ flex: 1, textAlign: 'left', color: 'var(--foreground)', fontWeight: 600 }}>
      {starting ? 'Подключение...' : 'Начать урок'}
    </span>
    {!starting && (
      <span style={{ color: 'var(--muted-foreground)', fontSize: 11 }}>→</span>
    )}
  </button>
</div>
```

- [ ] **Step 4: Проверить TypeScript**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```

Ожидаемый вывод: пусто.

- [ ] **Step 5: Коммит**

```bash
git add frontend/src/components/layout/Sidebar.tsx
git commit -m "feat: add start lesson button to sidebar"
```

---

## Финальная проверка

- [ ] Запустить Go-тесты

```bash
go test ./...
```

- [ ] Запустить бэкенд и фронтенд, убедиться что кнопка появилась в сайдбаре и переход работает

```bash
# Терминал 1
air

# Терминал 2
cd frontend && npm run dev
```

Открыть http://localhost:3000 → нажать «Начать урок» → убедиться что открывается страница комнаты.
