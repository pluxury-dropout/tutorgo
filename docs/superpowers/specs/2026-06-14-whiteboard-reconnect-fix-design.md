# Whiteboard Reconnect Fix — Design Spec

**Date:** 2026-06-14  
**Status:** Approved

## Problem

Two bugs cause the "Переподключение..." banner to appear immediately on board load:

1. **Stale object reference in `connect` deps** — `useWhiteboardSync` captures the full `page` object (not just `page.id`) in the `connect` useCallback deps. React Query returns a new JavaScript object on every background refetch even when data hasn't changed. Each new `page` reference causes `connect` to be recreated → `useEffect` re-runs → old WS is closed → new WS opens. During this reconnect window, if the new WS fails, status hits `disconnected`.

2. **Token race condition** — `getWsUrl` reads `localStorage.getItem('tg_token')` synchronously at the moment `connect()` is called. The Axios interceptor's `proactiveRefresh()` runs asynchronously before HTTP requests but its Promise is not exposed. If `connect()` fires while a proactive refresh is in-flight, it may read a stale (or expired) token. The server then returns HTTP 401 before the WebSocket upgrade, causing immediate `onerror` → `onclose` → `setStatus('disconnected')`.

## Approach

### Fix 1: `page?.id` in `connect` deps

**File:** `frontend/src/components/whiteboard/useWhiteboardSync.ts`

Replace `page` (object) with `page?.id` (string primitive) in the `useCallback` dependency array. Inside the callback, capture `page?.id` via the closure at the time of recreation — only the ID is needed for `getWsUrl`.

```ts
const connect = useCallback(async () => {
  const pageId = page?.id
  if (!pageId) return
  wsRef.current?.close()
  const wsToken = await getTokenAsync(token)
  const ws = new WebSocket(getWsUrl(pageId, wsToken))
  // ...
}, [page?.id, token, store])
```

`useCallback` now recreates only when the page ID string changes (i.e., user navigates to a different page), not on React Query refetches that return the same data.

### Fix 2: `getTokenAsync()` in `client.ts`

**File:** `frontend/src/lib/api/client.ts`

Expose `refreshPromise` as a module-level variable (instead of just the boolean `isRefreshing`). Export `getTokenAsync()` that awaits any in-flight refresh before reading from localStorage.

```ts
let refreshPromise: Promise<void> | null = null

export async function getTokenAsync(fallback?: string): Promise<string | undefined> {
  if (refreshPromise) await refreshPromise
  return localStorage.getItem('tg_token') ?? fallback ?? undefined
}
```

Update `proactiveRefresh()` to assign its own Promise to `refreshPromise` (clearing it in `finally`). The existing `isRefreshing` boolean guard stays for the early-return dedup check.

### `connect` becomes async

`connect` calls `await getTokenAsync(token)` before creating the WebSocket. The `useEffect` calls `connect()` without `await` (fire-and-forget). The cleanup ref guards (`closedRef`, `wsRef`) still work correctly because they check state at the moment the async call completes.

## Affected Files

| File | Change |
|------|--------|
| `frontend/src/lib/api/client.ts` | Add `refreshPromise` var; export `getTokenAsync()` |
| `frontend/src/components/whiteboard/useWhiteboardSync.ts` | `page?.id` in deps; `connect` async; call `getTokenAsync` |

## Not Changed

- `getWsUrl()` — signature unchanged (still accepts `string | undefined`)
- Backend (`whiteboard_ws.go`) — no changes
- Component layer (`TldrawCanvas`, board pages) — no changes

## Success Criteria

- Navigating to the board does not show the "Переподключение..." banner
- Page switches still reconnect correctly (different `page.id`)
- Token refresh in-flight no longer causes WS auth failure
