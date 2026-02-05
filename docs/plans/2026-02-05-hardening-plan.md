# Security Hardening Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add rate limiting, security headers, upload integrity checks, and better cleanup to align SendRec with security/privacy expectations.

**Architecture:** Extend existing chi middleware stack for headers/limits; enhance storage/video handlers to validate uploaded objects; add S3 head support; keep changes minimal and backward compatible.

**Tech Stack:** Go 1.25, chi, pgx, AWS SDK v2 (S3), React (no changes planned), Docker Compose.

### Task 1: Rate limit video APIs
**Files:** modify `internal/server/server.go`, tests in `internal/server/server_test.go`.
- Add a limiter for `/api/videos` routes (reuse ratelimit.NewLimiter with balanced values).
- Ensure tests cover limiter registration (status 429 reachable).
- Run `go test ./internal/server`.

### Task 2: Security headers middleware
**Files:** add `internal/server/security.go`; wire in `internal/server/server.go`; tests in `internal/server/server_test.go`.
- Middleware sets: `Content-Security-Policy` (self + data: for media?), `Referrer-Policy` (no-referrer), `X-Frame-Options` (SAMEORIGIN), `X-Content-Type-Options` (nosniff), `Permissions-Policy` (restrict camera/mic/screen to self), `Strict-Transport-Security` when https BaseURL.
- Ensure API and watch pages receive headers; update tests.
- Run `go test ./internal/server`.

### Task 3: Upload integrity check before marking ready
**Files:** `internal/storage/storage.go`, `internal/video/video.go`, storage interface; tests in `internal/video/video_test.go`, `internal/storage/storage_test.go`.
- Add `HeadObject`/`StatObject` to storage to return size/content-type.
- In `Update` when setting status ready, fetch object metadata and verify size matches stored `file_size` and content-type `video/webm`; if mismatch, return 409/400.
- Adjust mocks and tests accordingly.
- Run `go test ./internal/video ./internal/storage`.

### Task 4: Safer delete cleanup
**Files:** `internal/video/video.go`.
- When async delete fails, log the error (do not fail request); consider future retry hook.
- Update tests to assert delete call still invoked.
- Run `go test ./internal/video`.

### Task 5: Docs/env tweaks (optional but quick)
**Files:** `README.md`, `.env.example`, `docker-compose.dev.yml`.
- Mention new headers/limits and any new env knobs if added (none expected beyond existing).
- No tests; just keep docs in sync.

### Final verification
- Run full suite: `go test ./...`.
- If docker compose in use, ensure migration still succeeds (manual note).
