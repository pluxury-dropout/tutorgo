# Whiteboard Follow / Presence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Сделать follow-режим доски мгновенным и плавным, устранив «большое опоздание» и «вообще не следует», через серверный presence-state, протокол follow на `onUserFollow`, интерполяцию камеры и батчинг курсоров.

**Architecture:** Сервер-хаб перестаёт быть тупым релеем эфемерных сообщений и становится держателем presence-state: хранит последний cursor/viewport каждого пира и карту `follower→target`, рассылает их коалесцированно по тикеру (~33мс). Follow становится серверно-маршрутизируемым: при подписке сервер мгновенно юникастит подписчику текущий вьюпорт цели (снимает «не следует»), а вьюпорт цели дальше течёт только её подписчикам. Клиент интерполирует камеру к целевому вьюпорту через rAF-цикл (снимает рывки) и батчит применение чужих курсоров раз в кадр (снимает насыщение главного потока).

**Tech Stack:** Go (gorilla/websocket, единый run()-goroutine на страницу, testify), TypeScript/React, Excalidraw 0.18.1 (`onUserFollow`, `zoomToFitBounds`, `updateScene`), node:test + Node 24 TS-strip.

## Global Constraints

- Presence-логика на сервере живёт внутри одной run()-горутины хаба — **никаких мьютексов** в `presenceRegistry` (владелец один).
- Никаких новых зависимостей: только stdlib Go + уже установленные (`testify`, gorilla/websocket) и уже импортированные экспорты Excalidraw.
- Эфемерные сообщения (`cursor`, `viewport`, `follow`) **не персистятся** и никогда не идут в БД-снапшот — только `snapshot`/`update` затрагивают документ.
- Фронт-тесты запускаются `node --test <файл>.test.ts` (Node 24 стрипает TS), импорт локальных модулей — с расширением `.ts`.
- WS-протокол: `cursor` — поля верхнего уровня `{type,x,y,name,uid}`; `viewport` — `{type,payload:{bounds}}`; новый `follow` — `{type,payload:{target,action}}`. Сервер штампует `peerId` на исходящих `cursor`/`viewport`.
- `UPDATE_THROTTLE_MS = 100` (уже есть) — троттл исходящего вьюпорта ведущего оставляем; сглаживание делает интерполяция на приёме.

---

### Task 1: Серверный presence-registry (чистая логика)

Чистое ядро presence без WebSocket: хранит последний cursor/viewport по peerID, карту `follower→target`, отдаёт коалесцированные рассылки. Тестируется изолированно.

**Files:**
- Create: `handlers/whiteboard_presence.go`
- Test: `handlers/whiteboard_presence_test.go`

**Interfaces:**
- Consumes: `encoding/json` (stdlib).
- Produces (используется Task 2):
  - `func newPresenceRegistry() *presenceRegistry`
  - `func (r *presenceRegistry) setCursor(peerID string, msg json.RawMessage)`
  - `func (r *presenceRegistry) setViewport(peerID string, msg json.RawMessage)`
  - `func (r *presenceRegistry) follow(follower, target string) json.RawMessage` — возвращает текущий viewport цели для мгновенного снапа (nil, если цель ещё не вещала).
  - `func (r *presenceRegistry) unfollow(follower string)`
  - `func (r *presenceRegistry) remove(peerID string)`
  - `func (r *presenceRegistry) followersOf(peerID string) []string`
  - `func (r *presenceRegistry) flush() []presenceOut`
  - `type presenceOut struct { data json.RawMessage; origin string; to []string; toAll bool }`

- [ ] **Step 1: Написать падающие тесты**

`handlers/whiteboard_presence_test.go`:

```go
package handlers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPresenceCursorFlushBroadcastsOnceThenClears(t *testing.T) {
	r := newPresenceRegistry()
	r.setCursor("A", json.RawMessage(`{"type":"cursor","peerId":"A"}`))
	out := r.flush()
	assert.Len(t, out, 1)
	assert.True(t, out[0].toAll)
	assert.Equal(t, "A", out[0].origin)
	assert.Empty(t, r.flush(), "dirty должен очиститься после flush")
}

func TestPresenceFollowReturnsTargetViewportSnap(t *testing.T) {
	r := newPresenceRegistry()
	r.setViewport("A", json.RawMessage(`{"type":"viewport","peerId":"A"}`))
	snap := r.follow("B", "A")
	assert.JSONEq(t, `{"type":"viewport","peerId":"A"}`, string(snap))
}

func TestPresenceFollowNoViewportYetReturnsNil(t *testing.T) {
	r := newPresenceRegistry()
	assert.Nil(t, r.follow("B", "A"))
}

func TestPresenceViewportRoutedOnlyToFollowers(t *testing.T) {
	r := newPresenceRegistry()
	r.follow("B", "A")
	r.setViewport("A", json.RawMessage(`{"type":"viewport","peerId":"A"}`))
	out := r.flush()
	assert.Len(t, out, 1)
	assert.False(t, out[0].toAll)
	assert.Equal(t, []string{"B"}, out[0].to)
}

func TestPresenceViewportNoFollowersEmitsNothing(t *testing.T) {
	r := newPresenceRegistry()
	r.setViewport("A", json.RawMessage(`{"type":"viewport","peerId":"A"}`))
	assert.Empty(t, r.flush())
}

func TestPresenceUnfollowStopsRouting(t *testing.T) {
	r := newPresenceRegistry()
	r.follow("B", "A")
	r.unfollow("B")
	r.setViewport("A", json.RawMessage(`{"type":"viewport","peerId":"A"}`))
	assert.Empty(t, r.flush())
}

func TestPresenceRemoveTargetClearsFollowers(t *testing.T) {
	r := newPresenceRegistry()
	r.follow("B", "A")
	r.remove("A")
	assert.Empty(t, r.followersOf("A"))
	_, stillFollowing := r.following["B"]
	assert.False(t, stillFollowing)
}
```

- [ ] **Step 2: Запустить тесты — убедиться, что не компилируются/падают**

Run: `go test ./handlers/ -run TestPresence`
Expected: FAIL — `undefined: newPresenceRegistry` и т.д.

- [ ] **Step 3: Реализовать registry**

`handlers/whiteboard_presence.go`:

```go
package handlers

import "encoding/json"

// peerPresence — последнее эфемерное состояние одного соединения.
type peerPresence struct {
	cursor   json.RawMessage // последний cursor (уже с peerId)
	viewport json.RawMessage // последний viewport (уже с peerId)
}

// presenceRegistry — авторитетное эфемерное состояние одной страницы доски.
// НЕ потокобезопасен: владелец — единственная run()-горутина хаба.
type presenceRegistry struct {
	peers       map[string]*peerPresence
	following   map[string]string // follower peerID -> target peerID
	cursorDirty map[string]bool
	vpDirty     map[string]bool
}

func newPresenceRegistry() *presenceRegistry {
	return &presenceRegistry{
		peers:       map[string]*peerPresence{},
		following:   map[string]string{},
		cursorDirty: map[string]bool{},
		vpDirty:     map[string]bool{},
	}
}

func (r *presenceRegistry) peer(id string) *peerPresence {
	p := r.peers[id]
	if p == nil {
		p = &peerPresence{}
		r.peers[id] = p
	}
	return p
}

func (r *presenceRegistry) setCursor(peerID string, msg json.RawMessage) {
	r.peer(peerID).cursor = msg
	r.cursorDirty[peerID] = true
}

func (r *presenceRegistry) setViewport(peerID string, msg json.RawMessage) {
	r.peer(peerID).viewport = msg
	r.vpDirty[peerID] = true
}

// follow фиксирует follower->target и возвращает текущий viewport цели для
// мгновенного снапа (nil, если цель ещё не вещала вьюпорт).
func (r *presenceRegistry) follow(follower, target string) json.RawMessage {
	r.following[follower] = target
	if p := r.peers[target]; p != nil {
		return p.viewport
	}
	return nil
}

func (r *presenceRegistry) unfollow(follower string) {
	delete(r.following, follower)
}

// remove убирает presence пира и любые follow-связи, его касающиеся.
func (r *presenceRegistry) remove(peerID string) {
	delete(r.peers, peerID)
	delete(r.cursorDirty, peerID)
	delete(r.vpDirty, peerID)
	delete(r.following, peerID) // если пир был подписчиком
	for f, t := range r.following {
		if t == peerID { // пиры, следившие за ушедшим
			delete(r.following, f)
		}
	}
}

func (r *presenceRegistry) followersOf(peerID string) []string {
	var out []string
	for f, t := range r.following {
		if t == peerID {
			out = append(out, f)
		}
	}
	return out
}

// presenceOut — одна адресованная рассылка из flush.
// toAll=true: всем, кроме origin. Иначе — только пирам из to.
type presenceOut struct {
	data   json.RawMessage
	origin string
	to     []string
	toAll  bool
}

// flush отдаёт коалесцированные рассылки за тик и сбрасывает dirty-множества.
// Курсоры идут всем (Excalidraw показывает все курсоры), вьюпорт — только
// подписчикам данного пира.
func (r *presenceRegistry) flush() []presenceOut {
	var out []presenceOut
	for id := range r.cursorDirty {
		if p := r.peers[id]; p != nil && p.cursor != nil {
			out = append(out, presenceOut{data: p.cursor, origin: id, toAll: true})
		}
		delete(r.cursorDirty, id)
	}
	for id := range r.vpDirty {
		p := r.peers[id]
		if p != nil && p.viewport != nil {
			if fs := r.followersOf(id); len(fs) > 0 {
				out = append(out, presenceOut{data: p.viewport, to: fs})
			}
		}
		delete(r.vpDirty, id)
	}
	return out
}
```

- [ ] **Step 4: Запустить тесты — убедиться, что проходят**

Run: `go test ./handlers/ -run TestPresence -v`
Expected: PASS (7 тестов).

- [ ] **Step 5: Коммит**

```bash
git add handlers/whiteboard_presence.go handlers/whiteboard_presence_test.go
git commit -m "feat(board): presence-registry — коалесинг курсоров и маршрутизация вьюпорта по подписчикам"
```

---

### Task 2: Встроить presence-registry в хаб (тикер, follow, снап, remove)

Заменяем немедленный релей `cursor`/`viewport` на запись в registry; добавляем тикер-рассылку, обработку `follow` с мгновенным снапом и очистку presence при отключении.

**Files:**
- Modify: `handlers/whiteboard_ws.go` — `run()` (`:93-178`), `unregister` case (`:104-118`), удалить старый `case "cursor", "viewport"` (`:150-155`).

**Interfaces:**
- Consumes: `newPresenceRegistry`, `setCursor`, `setViewport`, `follow`, `unfollow`, `remove`, `flush`, `presenceOut` (Task 1); `WbMsg` (`:22`), `wbClient.peerID` (`:44`).
- Produces: WS-поведение — исходящие `cursor`/`viewport` с `peerId`, follow-снап; используется клиентом (Task 4).

- [ ] **Step 1: Добавить тикер и registry в run()**

В `handlers/whiteboard_ws.go`, в начале `func (h *wbHub) run() {` (сразу после строки `func (h *wbHub) run() {`, перед `for {`):

```go
	reg := newPresenceRegistry()
	// Коалесцируем эфемерные сообщения: свежий кадр вытесняет старый, рассылаем
	// пачкой раз в ~33мс — курсоры всем, вьюпорт только подписчикам.
	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()

	send := func(data []byte, to *wbClient) {
		select {
		case to.send <- data:
		default:
			delete(h.clients, to)
			close(to.send)
		}
	}
	// ponytail: линейный поиск клиента по peerID. Клиентов на страницу единицы
	// (препод+ученик), карта byPeer себя не окупает; ввести, если N вырастет.
	clientByPeer := func(peerID string) *wbClient {
		for c := range h.clients {
			if c.peerID == peerID {
				return c
			}
		}
		return nil
	}
```

- [ ] **Step 2: Добавить case тикера в select**

В `select` внутри `run()` (рядом с `case msg := <-h.broadcast:`) добавить:

```go
		case <-ticker.C:
			for _, o := range reg.flush() {
				if o.toAll {
					for c := range h.clients {
						if c.peerID == o.origin {
							continue
						}
						send(o.data, c)
					}
					continue
				}
				for _, pid := range o.to {
					if c := clientByPeer(pid); c != nil {
						send(o.data, c)
					}
				}
			}
```

- [ ] **Step 3: Заменить обработку cursor/viewport и добавить follow**

Заменить существующий блок (`:150-155`):

```go
			case "cursor", "viewport":
				// Attach sender's peerId, then relay. Follow-mode matches the
				// viewport sender against the follower's userToFollow.socketId,
				// which equals this same peerId (learned via cursor messages).
				parsed.PeerID = msg.sender.peerID
				msg.data, _ = json.Marshal(parsed)
```

на:

```go
			case "cursor":
				// Штампуем peerId и кладём в presence; рассылку делает тикер.
				parsed.PeerID = msg.sender.peerID
				stamped, _ := json.Marshal(parsed)
				reg.setCursor(msg.sender.peerID, stamped)
				continue

			case "viewport":
				parsed.PeerID = msg.sender.peerID
				stamped, _ := json.Marshal(parsed)
				reg.setViewport(msg.sender.peerID, stamped)
				continue

			case "follow":
				// Подписка/отписка. При FOLLOW сразу юникастим текущий вьюпорт
				// цели — камера ведомого снапится, не дожидаясь движения ведущего.
				var f struct {
					Target string `json:"target"`
					Action string `json:"action"`
				}
				_ = json.Unmarshal(parsed.Payload, &f)
				if f.Action == "UNFOLLOW" {
					reg.unfollow(msg.sender.peerID)
				} else if snap := reg.follow(msg.sender.peerID, f.Target); snap != nil {
					send(snap, msg.sender)
				}
				continue
```

- [ ] **Step 4: Очищать presence при отключении**

В `unregister` case, внутри `if _, ok := h.clients[client]; ok {` (`:105`), сразу после `delete(h.clients, client)` (`:106`) добавить:

```go
			reg.remove(client.peerID)
```

- [ ] **Step 5: Собрать и прогнать серверные тесты**

Run: `go build ./... && go test ./handlers/`
Expected: PASS, компиляция без ошибок. (Старый `case "cursor","viewport"` больше не существует; нижний релей-цикл теперь обслуживает только `update`/`default`.)

- [ ] **Step 6: Ручной smoke — мгновенный снап**

Две вкладки на одной странице доски (препод + ученик). Ученик стоит на месте, препод в стороне. Кликнуть по аватару препода у ученика → камера ученика **сразу** прыгает к области препода, не дожидаясь, пока препод подвигает холст.
Expected: снап < 500мс, без ожидания движения ведущего.

- [ ] **Step 7: Коммит**

```bash
git add handlers/whiteboard_ws.go
git commit -m "feat(board): хаб держит presence-state — тикер-коалесинг, follow-маршрутизация, мгновенный снап"
```

---

### Task 3: Клиентская интерполяция камеры (чистая функция)

Чистые хелперы lerp камеры и проверки сходимости — ядро плавного follow, тестируется на node:test.

**Files:**
- Create: `frontend/src/components/whiteboard/viewportInterp.ts`
- Test: `frontend/src/components/whiteboard/viewportInterp.test.ts`

**Interfaces:**
- Produces (используется Task 4):
  - `type Camera = { scrollX: number; scrollY: number; zoom: number }`
  - `function lerpCamera(from: Camera, to: Camera, t: number): Camera`
  - `function camerasClose(a: Camera, b: Camera, posEps?: number, zoomEps?: number): boolean`

- [ ] **Step 1: Написать падающие тесты**

`frontend/src/components/whiteboard/viewportInterp.test.ts`:

```ts
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { lerpCamera, camerasClose } from './viewportInterp.ts'

test('lerpCamera на t=0.5 берёт середину по всем осям', () => {
  const mid = lerpCamera(
    { scrollX: 0, scrollY: 0, zoom: 1 },
    { scrollX: 10, scrollY: 20, zoom: 2 },
    0.5
  )
  assert.equal(mid.scrollX, 5)
  assert.equal(mid.scrollY, 10)
  assert.equal(mid.zoom, 1.5)
})

test('lerpCamera клампит t за пределами [0,1]', () => {
  const to = { scrollX: 10, scrollY: 10, zoom: 2 }
  assert.deepEqual(lerpCamera({ scrollX: 0, scrollY: 0, zoom: 1 }, to, 5), to)
  assert.deepEqual(
    lerpCamera({ scrollX: 0, scrollY: 0, zoom: 1 }, to, -1),
    { scrollX: 0, scrollY: 0, zoom: 1 }
  )
})

test('camerasClose: почти совпали — true, далеко — false', () => {
  const base = { scrollX: 0, scrollY: 0, zoom: 1 }
  assert.equal(camerasClose(base, { scrollX: 0.1, scrollY: 0, zoom: 1 }), true)
  assert.equal(camerasClose(base, { scrollX: 5, scrollY: 0, zoom: 1 }), false)
})
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `cd frontend && node --test src/components/whiteboard/viewportInterp.test.ts`
Expected: FAIL — модуль `./viewportInterp.ts` не найден.

- [ ] **Step 3: Реализовать хелперы**

`frontend/src/components/whiteboard/viewportInterp.ts`:

```ts
export type Camera = { scrollX: number; scrollY: number; zoom: number }

// Линейная интерполяция камеры. t клампится в [0,1]: устойчиво к рывкам dt.
export function lerpCamera(from: Camera, to: Camera, t: number): Camera {
  const k = t < 0 ? 0 : t > 1 ? 1 : t
  return {
    scrollX: from.scrollX + (to.scrollX - from.scrollX) * k,
    scrollY: from.scrollY + (to.scrollY - from.scrollY) * k,
    zoom: from.zoom + (to.zoom - from.zoom) * k,
  }
}

// Камеры практически совпали — интерполяцию можно остановить до нового кадра.
export function camerasClose(
  a: Camera,
  b: Camera,
  posEps = 0.5,
  zoomEps = 0.001
): boolean {
  return (
    Math.abs(a.scrollX - b.scrollX) < posEps &&
    Math.abs(a.scrollY - b.scrollY) < posEps &&
    Math.abs(a.zoom - b.zoom) < zoomEps
  )
}
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `cd frontend && node --test src/components/whiteboard/viewportInterp.test.ts`
Expected: PASS (3 теста).

- [ ] **Step 5: Коммит**

```bash
git add frontend/src/components/whiteboard/viewportInterp.ts frontend/src/components/whiteboard/viewportInterp.test.ts
git commit -m "feat(board): чистые хелперы интерполяции камеры для follow"
```

---

### Task 4: Клиентский протокол follow + интерполяция + батч курсоров

Подключаем `onUserFollow`, шлём намерение follow серверу, принятый вьюпорт кладём в цель и плавно доводим камеру через rAF-цикл; чужие курсоры применяем батчем раз в кадр вместо `updateScene` на каждое сообщение.

**Files:**
- Modify: `frontend/src/components/whiteboard/useExcalidrawSync.ts` — импорты (`:1-38`), рефы (`:82-109`), `pushCollaborators` (`:118-131`), `cursor`/`leave`/`viewport` обработчики (`:328-360`), page-эффект (`:381-414`), `broadcastViewport` (`:474-489`), интерфейс `ExcalidrawSyncResult` (`:50-58`) и `return` (`:509-517`).
- Modify: `frontend/src/components/whiteboard/ExcalidrawCanvas.tsx` — деструктуризация хука (`:135-138`), пропсы Excalidraw (`:513-514`).

**Interfaces:**
- Consumes: `lerpCamera`, `camerasClose`, `Camera` (Task 3); серверный follow-снап и маршрутизация вьюпорта (Task 2); `NormalizedZoomValue`, `zoomToFitBounds`, `getVisibleSceneBounds` (Excalidraw).
- Produces: `sendFollow(target: string | null, action: 'FOLLOW' | 'UNFOLLOW'): void` в `ExcalidrawSyncResult`.

- [ ] **Step 1: Импорты и типы**

В `useExcalidrawSync.ts` в импорт из `@excalidraw/excalidraw/types` (`:12-18`) добавить `NormalizedZoomValue`:

```ts
import type {
  ExcalidrawImperativeAPI,
  BinaryFileData,
  DataURL,
  Collaborator,
  SocketId,
  NormalizedZoomValue,
} from '@excalidraw/excalidraw/types'
```

Добавить импорт хелперов (после импорта `./excalidrawSync`, `:35`):

```ts
import { lerpCamera, camerasClose, type Camera } from './viewportInterp'
```

Добавить константу рядом с `UPDATE_THROTTLE_MS` (`:42`):

```ts
const FOLLOW_LERP = 0.3 // доля пути к цели за кадр — компромисс плавность/лаг
```

- [ ] **Step 2: Расширить интерфейс результата**

В `ExcalidrawSyncResult` (`:50-58`) добавить поле:

```ts
  sendFollow: (target: string | null, action: 'FOLLOW' | 'UNFOLLOW') => void
```

- [ ] **Step 3: Добавить рефы**

После `collaboratorsRef` (`:99`) добавить:

```ts
  // Follow: за кем следим (peerId) и куда ведём камеру.
  const followTargetRef = useRef<string | null>(null)
  const targetCamRef = useRef<Camera | null>(null)
  // Гард: пока сами двигаем камеру в follow, onScrollChange не вещаем (эхо).
  const applyingRemoteRef = useRef(false)
  // Курсоры пиров меняются — применим пачкой в rAF, не на каждое сообщение.
  const collaboratorsDirtyRef = useRef(false)
  const rafRef = useRef<number | null>(null)
```

- [ ] **Step 4: Курсор/leave — помечать dirty вместо немедленного push; чистить follow при уходе цели**

Заменить обработчик `cursor` (`:328-336`):

```ts
      if (msg.type === 'cursor' && msg.peerId) {
        collaboratorsRef.current.set(msg.peerId as SocketId, {
          pointer: { x: msg.x ?? 0, y: msg.y ?? 0, tool: 'pointer' },
          username: msg.name || 'Гость',
          id: msg.uid,
        })
        collaboratorsDirtyRef.current = true
        return
      }
```

Заменить обработчик `leave` (`:338-342`):

```ts
      if (msg.type === 'leave' && msg.peerId) {
        collaboratorsRef.current.delete(msg.peerId as SocketId)
        collaboratorsDirtyRef.current = true
        // Ушёл тот, за кем следили — снимаем слежку, иначе камера застынет.
        if (followTargetRef.current === msg.peerId) {
          followTargetRef.current = null
          targetCamRef.current = null
        }
        return
      }
```

- [ ] **Step 5: viewport — не применять напрямую, а класть целевую камеру**

Заменить обработчик `viewport` (`:347-360`):

```ts
      // Follow: вьюпорт приходит уже только для нашей цели (сервер
      // маршрутизирует). Считаем целевую камеру, доводит к ней rAF-цикл.
      if (msg.type === 'viewport' && msg.peerId) {
        if (msg.peerId !== followTargetRef.current) return
        const api = apiRef.current
        if (!api) return
        const bounds = (msg.payload as { bounds?: SceneBounds } | undefined)
          ?.bounds
        if (!bounds) return
        const fit = zoomToFitBounds({
          bounds,
          appState: api.getAppState(),
          fitToViewport: true,
        }).appState
        targetCamRef.current = {
          scrollX: fit.scrollX,
          scrollY: fit.scrollY,
          zoom: fit.zoom.value,
        }
        return
      }
```

- [ ] **Step 6: rAF-цикл (интерполяция камеры + флаш курсоров)**

Добавить перед page-эффектом (`:381`):

```ts
  // Единый кадровый цикл: раз в кадр применяем накопленные курсоры и,
  // если следим за кем-то, подводим камеру к цели интерполяцией.
  const tick = useCallback(() => {
    const api = apiRef.current
    if (api) {
      if (collaboratorsDirtyRef.current) {
        collaboratorsDirtyRef.current = false
        pushCollaborators()
      }
      const target = targetCamRef.current
      if (followTargetRef.current && target) {
        const s = api.getAppState()
        const cur: Camera = { scrollX: s.scrollX, scrollY: s.scrollY, zoom: s.zoom.value }
        if (!camerasClose(cur, target)) {
          const next = lerpCamera(cur, target, FOLLOW_LERP)
          applyingRemoteRef.current = true
          api.updateScene({
            appState: {
              scrollX: next.scrollX,
              scrollY: next.scrollY,
              zoom: { value: next.zoom as NormalizedZoomValue },
            },
          })
          applyingRemoteRef.current = false
        }
      }
    }
    rafRef.current = requestAnimationFrame(tick)
  }, [pushCollaborators])
```

- [ ] **Step 7: Запускать/останавливать цикл в page-эффекте**

В page-эффекте (`:381-414`), сразу после `void connect()` (`:394`) добавить:

```ts
    rafRef.current = requestAnimationFrame(tick)
```

В cleanup этого эффекта (`:395`), после `closedRef.current = true` добавить:

```ts
      if (rafRef.current) cancelAnimationFrame(rafRef.current)
      followTargetRef.current = null
      targetCamRef.current = null
```

Добавить `tick` в массив зависимостей эффекта (`:414`): `}, [pageId, connect, sendSnapshotElements, tick])`.

- [ ] **Step 8: Эхо-гард в broadcastViewport + sendFollow**

В начало `broadcastViewport` (`:474`, первой строкой тела) добавить:

```ts
    if (applyingRemoteRef.current) return // не вещаем свои же follow-движения
```

Добавить `sendFollow` (рядом с `broadcastViewport`, после `:489`):

```ts
  const sendFollow = useCallback(
    (target: string | null, action: 'FOLLOW' | 'UNFOLLOW') => {
      followTargetRef.current = action === 'FOLLOW' ? target : null
      if (action === 'UNFOLLOW') targetCamRef.current = null
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(
          JSON.stringify({ type: 'follow', payload: { target, action } })
        )
      }
    },
    []
  )
```

Добавить `sendFollow` в `return` (`:509-517`): строкой `sendFollow,`.

- [ ] **Step 9: Прокинуть в Excalidraw**

В `ExcalidrawCanvas.tsx` в деструктуризацию хука (`:135-138`) добавить `sendFollow`.

Заменить `onScrollChange` (`:514`) и добавить `onUserFollow`:

```tsx
          onPointerUpdate={(p) => sendCursor(p.pointer.x, p.pointer.y)}
          onScrollChange={() => broadcastViewport()}
          onUserFollow={(payload) =>
            sendFollow(payload.userToFollow.socketId, payload.action)
          }
```

- [ ] **Step 10: Проверка типов и сборка**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: без ошибок типов и сборки.

- [ ] **Step 11: Ручной smoke — плавность и отсутствие эха**

Две вкладки. Ученик кликает аватар препода → камера **плавно** доезжает (не прыжками). Препод панорамирует/зумит → камера ученика плавно следует. Ученик сам двигает холст → Excalidraw снимает слежку (`onUserFollow` UNFOLLOW), камера остаётся под контролем ученика, не дёргается обратно.
Expected: нет рывков 10fps, нет обратного самопинания камеры.

- [ ] **Step 12: Коммит**

```bash
git add frontend/src/components/whiteboard/useExcalidrawSync.ts frontend/src/components/whiteboard/ExcalidrawCanvas.tsx
git commit -m "feat(board): follow через onUserFollow + интерполяция камеры + батч курсоров на rAF"
```

---

### Task 5: Троттлинг исходящих курсоров

Курсор сейчас шлётся на каждое `onPointerUpdate` (десятки/сек). Троттлим отправку до ~20 Гц — сеть и хаб разгружаются; приём уже батчится (Task 4).

**Files:**
- Modify: `frontend/src/components/whiteboard/useExcalidrawSync.ts` — `sendCursor` (`:459-468`), константа рядом с `UPDATE_THROTTLE_MS` (`:42`), реф рядом с прочими (`:82-99`).

**Interfaces:**
- Consumes: `wsRef`, `displayName`, `myUid` (существующие).
- Produces: не меняет сигнатуру `sendCursor`.

- [ ] **Step 1: Константа и реф**

Рядом с `UPDATE_THROTTLE_MS` (`:42`):

```ts
const CURSOR_THROTTLE_MS = 50
```

Рядом с рефами (после `:88`):

```ts
  const cursorSentRef = useRef(0)
```

- [ ] **Step 2: Троттлить sendCursor**

Заменить `sendCursor` (`:459-468`):

```ts
  const sendCursor = useCallback(
    (x: number, y: number) => {
      const now = performance.now()
      if (now - cursorSentRef.current < CURSOR_THROTTLE_MS) return
      cursorSentRef.current = now
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(
          JSON.stringify({ type: 'cursor', x, y, name: displayName, uid: myUid })
        )
      }
    },
    [displayName, myUid]
  )
```

- [ ] **Step 3: Сборка**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок.

- [ ] **Step 4: Ручной smoke**

Две вкладки; препод быстро водит мышью. Курсор у ученика движется плавно, без фризов холста; в Network WS-фрейма `cursor` не чаще ~20/сек.

- [ ] **Step 5: Коммит**

```bash
git add frontend/src/components/whiteboard/useExcalidrawSync.ts
git commit -m "perf(board): троттл исходящих курсоров до 20 Гц"
```

---

### Task 6: Backpressure по bufferedAmount для эфемерных отправок

Если сокет забит bulk-данными (снапшот до 512 КБ, диффы), эфемерный кадр не должен вставать в очередь за ними — при переполненном буфере кадр дропаем (следующий приедет свежим).

**Files:**
- Modify: `frontend/src/components/whiteboard/useExcalidrawSync.ts` — `broadcastViewport` (`:474-489`), `sendCursor` (после Task 5), константа (`:42`).

**Interfaces:**
- Consumes: `wsRef` (существующий).
- Produces: без изменения сигнатур.

- [ ] **Step 1: Константа порога**

Рядом с прочими константами (`:42`):

```ts
// Буфер сокета выше порога — bulk-данные (снапшот/диффы) забили пайп;
// эфемерный кадр дропаем, следующий приедет свежим.
const EPHEMERAL_BACKPRESSURE_BYTES = 128 * 1024
```

- [ ] **Step 2: Гард в broadcastViewport**

В `send`-замыкании внутри `broadcastViewport` (`:476-483`), после проверки `readyState`, перед `getVisibleSceneBounds`:

```ts
    const send = () => {
      const api = apiRef.current
      const ws = wsRef.current
      if (!api || ws?.readyState !== WebSocket.OPEN) return
      if (ws.bufferedAmount > EPHEMERAL_BACKPRESSURE_BYTES) return
      const bounds = getVisibleSceneBounds(api.getAppState())
      ws.send(JSON.stringify({ type: 'viewport', payload: { bounds } }))
    }
```

- [ ] **Step 3: Гард в sendCursor**

В `sendCursor` (Task 5) после проверки `readyState` добавить строку перед `.send`:

```ts
      if (wsRef.current.bufferedAmount > EPHEMERAL_BACKPRESSURE_BYTES) return
```

(строку `cursorSentRef.current = now` оставить до этой проверки — троттл-окно двигаем даже при дропе, чтобы не долбить проверкой каждый кадр.)

- [ ] **Step 4: Сборка**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: без ошибок.

- [ ] **Step 5: Ручной smoke — под нагрузкой**

Препод вставляет крупный PDF/картинку (большой снапшот) и одновременно панорамирует. Камера ученика не зависает на секунды позади bulk-данных — вьюпорт либо доезжает свежим, либо кратко пропускается, но не копится в очереди.

- [ ] **Step 6: Коммит**

```bash
git add frontend/src/components/whiteboard/useExcalidrawSync.ts
git commit -m "perf(board): backpressure по bufferedAmount — эфемерный кадр не встаёт за bulk-данными"
```

---

## Self-Review

**Spec coverage (4 диагностированные проблемы → задачи):**
- Проблема 1 «вообще не следует» (нет мгновенной синхронизации) → Task 2 (follow-снап на сервере) + Task 4 (`onUserFollow`, `sendFollow`). ✓
- Проблема 2 «большое опоздание» (вьюпорт в очереди за bulk) → Task 6 (backpressure) + Task 2 (коалесинг на тикере снимает накопление). ✓
- Проблема 3 «курсоры насыщают ведомого» → Task 4 (батч применения на rAF) + Task 5 (троттл отправки). ✓
- Проблема 4 «камера 10fps рывками» → Task 3 + Task 4 (интерполяция на rAF). ✓

**Placeholder scan:** нет TODO/«обработать ошибки»/«аналогично Task N» — весь код приведён. ✓

**Type consistency:** `presenceOut{data,origin,to,toAll}`, `follow()→json.RawMessage`, `sendFollow(target,action)`, `Camera{scrollX,scrollY,zoom}`, `lerpCamera/camerasClose` — имена и сигнатуры совпадают между Task 1↔2 и Task 3↔4. `zoom.value: NormalizedZoomValue` согласован с Excalidraw. ✓

**Открытое решение (для ревью на исполнении):** `FOLLOW_LERP = 0.3` и `CURSOR_THROTTLE_MS = 50` — эмпирические; подстроить по ощущению на реальной сессии.
