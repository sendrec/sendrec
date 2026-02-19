# AGENTS.md

This file provides guidance to AI coding agents working with this repository.

## Project Overview

SendRec is an open-source, EU-native async video messaging platform (alternative to Loom). Single Go binary serves a REST API, embedded React SPA, and server-rendered watch pages. Deployed at app.sendrec.eu.

## Prerequisites

Go 1.25+, Node 24+, pnpm, Docker

## Commands

```bash
# Full stack (Docker)
make docker-up              # Start app + PostgreSQL + Garage
make docker-down            # Stop all

# Backend
make run                    # Go server (needs DATABASE_URL, S3 env vars)
make test                   # All Go tests
go test ./internal/auth/... # Single package
go test ./internal/video/... -run TestCreate  # Single test
make build                  # Build binary (builds frontend first)

# Frontend (from web/)
cd web && pnpm dev          # Dev server (port 5173, proxies API to 8080)
cd web && pnpm test         # Run all frontend tests
cd web && pnpm test -- --run  # Run once (no watch)
cd web && pnpm test:coverage
cd web && pnpm typecheck
cd web && pnpm build

# Linting
golangci-lint run           # v2.8.0+ required (v1 doesn't support Go 1.25)
```

## Architecture

Single Go binary (`cmd/sendrec/main.go`) that:
- Serves the React SPA (embedded via `//go:embed` in `web/embed.go`)
- Handles REST API at `/api/*`
- Renders server-side watch pages at `/watch/:token` and `/embed/:token` (Go HTML templates with vanilla JS, NOT React)
- Runs database migrations on startup
- Starts background workers (cleanup loop, transcription worker, view digest worker)

### Backend Packages

- `internal/server/` — chi router, middleware chain, security headers (CSP nonce per request)
- `internal/auth/` — JWT (15min access + 7-day refresh), registration, login, password reset, email confirmation, API keys
- `internal/video/` — Video CRUD, watch page, embed page, comments, analytics, transcription, ffmpeg processing
- `internal/storage/` — S3-compatible storage (Garage), presigned URLs for direct browser upload
- `internal/database/` — pgx connection pool, golang-migrate migrations
- `internal/email/` — Listmonk integration for notifications
- `internal/plans/` — Free tier limits
- `internal/httputil/` — `WriteJSON()`/`WriteError()` helpers, CSP nonce context
- `internal/ratelimit/` — Per-IP token bucket
- `internal/docs/` — OpenAPI spec and Scalar docs at `/api/docs`
- `migrations/` — Sequential SQL migrations (000001 through 000026)

### Frontend (React 19 + TypeScript + Vite 7)

- `web/src/App.tsx` — React Router v7, `ProtectedRoute` wrapper
- `web/src/api/client.ts` — Global access token, auto-refresh on 401
- `web/src/pages/` — Record, Library, Analytics, Login, Register, Settings, etc.

### Key Architectural Patterns

**Video upload flow:** Browser records via `getDisplayMedia` + `MediaRecorder` → POST `/api/videos` returns presigned S3 URLs → browser uploads directly to S3 → PATCH marks video ready → server triggers thumbnail generation + transcription.

**Watch page:** `internal/video/watch_page.go` is a large Go HTML template (~65KB) with embedded CSS and vanilla JS. It handles the video player, transcript panel, comments UI, emoji reactions, emoji picker, and timestamp editing. CSP nonce is injected via template data. This is NOT React — it's vanilla JS in a Go `html/template`.

**Embedded SPA:** `web/embed.go` uses `//go:embed all:dist`. The SPA fallback is registered via `router.NotFound()`.

**Database access:** Direct SQL in handler methods (no repository layer). Handlers accept `database.DBTX` interface for testability. Nullable columns use pointer types (`*string`).

**Auth tokens:** Access token (15min) in memory only, refresh token (7 days) in HttpOnly cookie + DB. On 401, frontend auto-refreshes via cookie.

## Design Constraints

- **EU data sovereignty:** Self-hosted fonts, no external CDNs, all assets served from same origin
- **GDPR-native:** Privacy-first defaults throughout
- **Patent avoidance:** No bubble/circle iconography (Loom patents)
- **Visual identity:** Navy + green color scheme
- **Accessibility:** Semantic HTML, skip links, ARIA labels

## Testing

**Go:** Standard `testing` package, table-driven tests, `pgxmock` for database, `httptest` for HTTP handlers. Tests create a `newTestHandler()` with mock pool, set query expectations, then assert responses.

**Frontend:** Vitest + React Testing Library. Tests mock `globalThis.fetch`. 317 tests across 22 files.

**CI:** GitHub Actions runs golangci-lint v2.8.0, `go test ./...`, `govulncheck`, frontend tests with coverage, typecheck, and build.

## Important Technical Notes

- **Nil slices in JSON:** Go nil slices marshal to `null`. Always `make([]T, 0)` for API arrays.
- **CSP and inline styles:** CSP nonces only work for `<style>` tags, not HTML `style` attributes.
- **S3_PUBLIC_ENDPOINT:** Presigned URLs must use public endpoint, not internal Docker hostname.
- **pgxmock:** Nullable columns need `&variable` for non-nil, `(*type)(nil)` for nil in AddRow.
- **`.gitignore` pattern:** `sendrec` in `.gitignore` matches `cmd/sendrec/` — use `git add -f cmd/sendrec/main.go`.
- **golangci-lint:** Must use v2.8.0+ (v1 doesn't support Go 1.25).

## Deployment

- **Server:** Hetzner CX33 (Helsinki), Caddy reverse proxy, Docker Compose
- **Environments:** Preview (`pr-{N}.app.sendrec.eu`), Staging (`staging.app.sendrec.eu`), Production (`app.sendrec.eu`)
- **Production deploy:** Tag `v*` → GitHub Actions (requires approval)
- **Services:** sendrec-app, sendrec-db (PostgreSQL 18), Garage, Caddy

## Git Workflow

- **Branch protection:** main requires PR + 1 code owner approval + CI pass
- **Always create feature branches and PRs** — never push directly to main

## Current In-Progress Work: Emoji Reactions (PR #43)

**Branch:** `feature/emoji-reactions` (3 commits ahead of main)

**What it does:** Adds quick emoji reactions (👍 👎 ❤️ 😂 😮 🎉) at specific video timestamps on the watch page. Reactions reuse the existing comment system — an emoji reaction is just a comment with an emoji body + timestamp. Zero backend changes, zero migrations.

**File modified:** `internal/video/watch_page.go` only.

### What's been implemented

1. **Reaction bar** — 6 emoji buttons below the markers-bar, gated by `{{if ne .CommentMode "disabled"}}`
2. **Click handler** — clicking an emoji posts it as a comment at the current playback timestamp
3. **Floating animation** — CSS `@keyframes float-up` makes emoji float upward and fade out
4. **Compact rendering** — emoji-only comments render as small inline badges instead of full comment cards
5. **Markers-bar dots** — reactions appear as dots on the timeline bar

### Known Issues to Fix

1. **Markers-bar dot positioning** — Dots may appear at position 0% (beginning) instead of their correct timestamp. Root cause: `videoDuration` could be `Infinity` for some video formats (especially WebM recordings), causing `timestamp / Infinity * 100 = 0`. Fix: add a `durationchange` event listener alongside `loadedmetadata` to handle cases where duration updates after initial metadata load. Also consider that `player.duration` might not be available when `renderMarkers` first runs.

2. **Click-to-seek on emoji reactions** — Clicking on a compact emoji reaction badge in the comments list should seek the video to that timestamp. The timestamp text badge (e.g., "0:03") already has a click handler via the `.comment-timestamp` class, but clicking the emoji itself doesn't seek. Fix: make the entire `.emoji-reaction` div clickable, adding a `cursor: pointer` style and a click handler that reads the `data-ts` from the nested `.comment-timestamp` span.

### How the watch page JS works

Key variables in the comments IIFE (`internal/video/watch_page.go`):
- `player` — the `<video>` element
- `videoDuration` — set from `player.duration` on `loadedmetadata` event (line ~708, initially 0)
- `lastComments` — array of comment objects from the API, used to re-render markers
- `markersBar` — the thin timeline bar element showing comment/reaction dots
- `shareToken` — from Go template `{{.ShareToken}}`
- `commentMode` — from Go template `{{.CommentMode}}` (disabled/anonymous/name_required/name_email_required)
- `reactionEmojis` — array of the 6 emoji characters used for reaction detection
- `isReactionEmoji(text)` — checks if a comment body is a single reaction emoji

Key functions:
- `renderMarkers(comments)` — clears and re-renders dots on markers-bar, groups comments by second
- `renderComment(c)` — returns HTML string, uses compact layout for emoji reactions
- `loadComments()` — fetches comments from API, renders list and markers
- `formatTimestamp(seconds)` — formats seconds to "M:SS" string
- `escapeHtml(text)` — HTML entity escaping

### Comment API endpoints (no changes needed)

- `GET /api/watch/{shareToken}/comments` — list comments for a video
- `POST /api/watch/{shareToken}/comments` — create a comment (rate limited: 0.2/sec, burst 3)
  - Body: `{ authorName, authorEmail, body, isPrivate, videoTimestamp }`
  - Comment mode gating applies (disabled = 403, name_required = validates name, etc.)

### Verification after fixes

```bash
cd /Users/aneamtu/Development/personal/sendrec/app
go test ./internal/video/... -count=1    # Backend tests (no changes expected)
cd web && pnpm test -- --run             # Frontend tests (no changes expected)
cd web && pnpm typecheck && pnpm build   # Type check + build
```

Then push and verify on PR preview: `https://pr-43.app.sendrec.eu/watch/{any-share-token}`
