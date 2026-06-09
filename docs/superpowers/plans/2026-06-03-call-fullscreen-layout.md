# Call Fullscreen Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Вынести страницы `/lessons/[id]/call` и `/room/[id]` из route group `(dashboard)` в новый route group `(call)`, чтобы они рендерились без сайдбара на весь экран.

**Architecture:** Создаём `app/(call)/layout.tsx` — минимальный лейаут с auth-проверкой, без Sidebar/MobileBottomNav. Перемещаем два page.tsx туда. Старые файлы удаляем. URL не меняются.

**Tech Stack:** Next.js 14 App Router, React, Zustand (`useAuthStore`), LiveKit `@livekit/components-react`

---

### Task 1: Создать `(call)/layout.tsx`

**Files:**
- Create: `frontend/src/app/(call)/layout.tsx`

- [ ] **Step 1: Создать файл лейаута**

```tsx
'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/stores/auth'

export default function CallLayout({ children }: { children: React.ReactNode }) {
  const { token } = useAuthStore()
  const isAuthenticated = !!token
  const router = useRouter()
  const [mounted, setMounted] = useState(false)

  useEffect(() => { setMounted(true) }, [])

  useEffect(() => {
    if (mounted && !isAuthenticated) router.replace('/login')
  }, [mounted, isAuthenticated, router])

  if (!mounted || !isAuthenticated) return null

  return (
    <div className="h-screen overflow-hidden">
      {children}
    </div>
  )
}
```

- [ ] **Step 2: Коммит**

```bash
git add frontend/src/app/\(call\)/layout.tsx
git commit -m "feat: add (call) route group layout without sidebar"
```

---

### Task 2: Перенести страницу урока

**Files:**
- Create: `frontend/src/app/(call)/lessons/[id]/call/page.tsx`
- Delete: `frontend/src/app/(dashboard)/lessons/[id]/call/page.tsx`

- [ ] **Step 1: Создать новый файл** — содержимое идентично старому, кроме строки с высотой

Полное содержимое `frontend/src/app/(call)/lessons/[id]/call/page.tsx`:

```tsx
'use client'

import { Component, useState, type ReactNode } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { LiveKitRoom, VideoConference } from '@livekit/components-react'
import '@livekit/components-styles'

import { callsApi, type RoomTokenResponse } from '@/lib/api/calls'
import { Button } from '@/components/ui/button'
import { Link, Check } from 'lucide-react'

class VideoConferenceBoundary extends Component<
  { children: ReactNode },
  { hasError: boolean }
> {
  state = { hasError: false }

  static getDerivedStateFromError() {
    return { hasError: true }
  }

  componentDidCatch() {
    setTimeout(() => this.setState({ hasError: false }), 0)
  }

  render() {
    return this.state.hasError ? null : this.props.children
  }
}

type Stage = 'idle' | 'starting' | 'connecting' | 'in-room'

export default function CallPage() {
  const { id } = useParams<{ id: string }>()
  const router  = useRouter()

  const [stage, setStage]   = useState<Stage>('idle')
  const [room, setRoom]     = useState<RoomTokenResponse | null>(null)
  const [error, setError]   = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  function handleCopyLink() {
    const url = `${window.location.origin}/join/${id}`
    navigator.clipboard.writeText(url)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  async function handleStart() {
    setStage('starting')
    setError(null)
    try {
      await callsApi.startRoom(id)
      navigator.clipboard.writeText(`${window.location.origin}/join/${id}`)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
      setStage('connecting')
      const data = await callsApi.getRoomToken(id)
      setRoom(data)
      setStage('in-room')
    } catch {
      setError('Не удалось запустить урок')
      setStage('idle')
    }
  }

  async function handleDisconnected() {
    try { await callsApi.endRoom(id) } catch {}
    router.back()
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-[80vh] gap-4">
        <p className="text-destructive">{error}</p>
        <Button variant="outline" onClick={() => router.back()}>Назад</Button>
      </div>
    )
  }

  if (stage === 'idle') {
    return (
      <div className="flex flex-col items-center justify-center h-[80vh] gap-4">
        <p className="text-muted-foreground">Нажмите кнопку, чтобы открыть комнату для учеников</p>
        <Button onClick={handleStart}>Начать урок</Button>
      </div>
    )
  }

  if (!room) {
    return (
      <div className="flex items-center justify-center h-[80vh]">
        <p className="text-muted-foreground">
          {stage === 'starting' ? 'Открываем комнату...' : 'Подключение...'}
        </p>
      </div>
    )
  }

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
}
```

- [ ] **Step 2: Удалить старый файл**

```bash
rm -rf "frontend/src/app/(dashboard)/lessons"
```

- [ ] **Step 3: Коммит**

```bash
git add "frontend/src/app/(call)/lessons/[id]/call/page.tsx"
git add -u "frontend/src/app/(dashboard)/lessons/"
git commit -m "feat: move lessons call page to (call) route group"
```

---

### Task 3: Перенести страницу быстрой комнаты

**Files:**
- Create: `frontend/src/app/(call)/room/[id]/page.tsx`
- Delete: `frontend/src/app/(dashboard)/room/[id]/page.tsx`

- [ ] **Step 1: Создать новый файл** — содержимое идентично старому, кроме строки с высотой

Полное содержимое `frontend/src/app/(call)/room/[id]/page.tsx`:

```tsx
'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { LiveKitRoom, VideoConference } from '@livekit/components-react'
import '@livekit/components-styles'

import { callsApi } from '@/lib/api/calls'
import { Check, Copy, Loader2 } from 'lucide-react'

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
    // LiveKit bug: placeholder→real track transition triggers a spurious console.error
    const orig = console.error.bind(console)
    console.error = (...args: unknown[]) => {
      if (typeof args[0] === 'string' && args[0].includes('Element not part of the array')) return
      orig(...args)
    }
    return () => { console.error = orig }
  }, [])

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
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100dvh' }}>
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (stage === 'error' || !token || !serverUrl) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100dvh', gap: 16 }}>
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
    <div style={{ height: '100dvh', position: 'relative' }}>
      <LiveKitRoom
        key={token}
        serverUrl={serverUrl}
        token={token}
        onDisconnected={handleDisconnected}
        data-lk-theme="default"
        style={{ height: '100%' }}
      >
        <VideoConference />
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

- [ ] **Step 2: Удалить старый файл**

```bash
rm -rf "frontend/src/app/(dashboard)/room"
```

- [ ] **Step 3: Коммит**

```bash
git add "frontend/src/app/(call)/room/[id]/page.tsx"
git add -u "frontend/src/app/(dashboard)/room/"
git commit -m "feat: move quick room page to (call) route group"
```

---

### Task 4: Проверить сборку

**Files:** нет изменений файлов

- [ ] **Step 1: Запустить TypeScript и сборку**

```bash
cd frontend && npm run build
```

Ожидается: успешная сборка без ошибок. Если ошибки TypeScript — проверить импорты в перенесённых файлах.

- [ ] **Step 2: Проверить роутинг вручную**

Запустить `cd frontend && npm run dev`, открыть:
- `/lessons/<любой-id>/call` — должна открываться без сайдбара
- `/room/<любой-id>` — должна открываться без сайдбара
- `/dashboard` — сайдбар должен работать как раньше

- [ ] **Step 3: Коммит не нужен** — изменений нет, только проверка
