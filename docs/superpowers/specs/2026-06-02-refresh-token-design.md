# Refresh Token — Design Spec

**Date:** 2026-06-02
**Status:** Approved

## Problem

JWT access tokens live 24 hours and cannot be invalidated. After a password change, old tokens remain valid up to 24 hours. With 10+ users on multiple devices, this is a real security gap.

## Goals

- Sessions persist 30 days without re-login
- Password change immediately invalidates all sessions (all devices)
- Logout invalidates only the current device session
- XSS cannot steal the refresh token

## Approach: Access Token (localStorage) + Refresh Token (httpOnly cookie)

Access token: short-lived JWT (15 min), stored in localStorage, used on every API request.
Refresh token: long-lived random string (30 days), stored in httpOnly cookie, invisible to JS.

## Database

New table `refresh_tokens`:

```sql
CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tutor_id   UUID NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON refresh_tokens(token);
CREATE INDEX ON refresh_tokens(tutor_id);
```

- `token` — 32 random bytes encoded as base64url, not a JWT
- `ON DELETE CASCADE` — deleting a tutor cleans up all sessions automatically
- Index on `token` for O(1) lookup on every refresh
- Index on `tutor_id` for O(1) bulk delete on password change

## Backend Changes

### New migration
`migrations/010_refresh_tokens.sql` — creates the table above.

### New files
- `models/auth.go` — add `RefreshTokenResponse { AccessToken string }`
- `repository/refresh_token.go` — interface + pgx implementation:
  - `Create(ctx, tutorID, token string, expiresAt time.Time) error`
  - `GetByToken(ctx, token string) (tutorID string, expiresAt time.Time, err error)`
  - `DeleteByToken(ctx, token string) error`
  - `DeleteAllByTutorID(ctx, tutorID string) error`
- `service/refresh_token.go` — thin wrapper over repository

### Modified files
- `handlers/auth.go`:
  - `Login`: access token TTL reduced to 15 min; generates random refresh token, stores in DB, sets httpOnly cookie
  - `Refresh` (new): reads cookie, validates against DB, returns new access token JSON
  - `Logout` (new): deletes refresh token from DB, clears cookie
- `handlers/tutor.go`:
  - `ChangePassword`: after `UpdatePassword`, calls `refreshTokenService.DeleteAllByTutorID`
- `router/router.go`:
  - Add `POST /auth/refresh` (public, rate-limited)
  - Add `POST /auth/logout` (protected)
  - Wire `refreshTokenRepo` and `refreshTokenService`

### Cookie config
```go
http.SetCookie(c.Writer, &http.Cookie{
    Name:     "refresh_token",
    Value:    token,
    HttpOnly: true,
    Secure:   cfg.Env == "production", // false in dev (HTTP), true in prod (HTTPS)
    SameSite: http.SameSiteStrictMode,
    MaxAge:   30 * 24 * 60 * 60,
    Path:     "/auth",
})
```

`Path: "/auth"` — browser only sends cookie to `/auth/*` routes, not every API call.
`Secure` is conditional — `localhost` runs over HTTP so `Secure: true` would silently drop the cookie.

## Frontend Changes

### `lib/api/client.ts`
Response interceptor: on 401, attempt `POST /auth/refresh` with `withCredentials: true` (browser sends the httpOnly cookie automatically). On success, save new access token and retry original request. On failure, clear auth and redirect to `/login`.

Flag to prevent infinite retry loops if `/auth/refresh` itself returns 401.

### `stores/auth.ts`
`clearAuth()`: call `POST /auth/logout` before clearing localStorage. This deletes the DB record and clears the cookie server-side.

### `lib/api/auth.ts`
`login()`: response now contains only `access_token`. Save it to localStorage as `tg_token`. Refresh token arrives as httpOnly cookie — no JS handling needed.

## Behaviour Summary

| Event | Result |
|---|---|
| Login | access_token (15 min) + refresh_token cookie (30 days) |
| API call with valid access_token | Normal flow |
| API call with expired access_token | Interceptor calls /auth/refresh → new access_token → retry |
| Logout | refresh_token deleted from DB + cookie cleared (current device only) |
| Password change | All refresh_tokens for tutorID deleted (all devices logged out) |
| Tutor deleted | All refresh_tokens deleted via CASCADE |

## Out of Scope

- Token rotation (each refresh issues a new refresh token)
- Active session listing / per-device revocation UI
- Redis-based token store (not needed at current scale)
