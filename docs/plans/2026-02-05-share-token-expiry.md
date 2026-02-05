# Share Token Expiry Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Share links expire after 7 days. Owners can extend them. Expired links show a "link expired" message.

**Architecture:** Add `share_expires_at` column to `videos` table. Watch endpoints check expiry and return a distinct "expired" response. New `POST /api/videos/{id}/extend` endpoint resets expiry to 7 days from now. List endpoint includes expiry info. Frontend shows expiry status in Library and an expired state on the watch page.

**Tech Stack:** Go (chi, pgx, pgxmock), PostgreSQL, React (TypeScript), HTML templates

---

## Task 1: Database Migration

Add `share_expires_at TIMESTAMPTZ` column to the `videos` table. Backfill existing rows with `created_at + 7 days`.

**Files:**
- Create: `migrations/000003_add_share_expires_at.up.sql`
- Create: `migrations/000003_add_share_expires_at.down.sql`

**Step 1: Create the up migration**

```sql
ALTER TABLE videos ADD COLUMN share_expires_at TIMESTAMPTZ;
UPDATE videos SET share_expires_at = created_at + INTERVAL '7 days';
ALTER TABLE videos ALTER COLUMN share_expires_at SET NOT NULL;
ALTER TABLE videos ALTER COLUMN share_expires_at SET DEFAULT now() + INTERVAL '7 days';
```

**Step 2: Create the down migration**

```sql
ALTER TABLE videos DROP COLUMN share_expires_at;
```

**Step 3: Verify migration syntax**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make test`
Expected: All existing tests still pass (migrations are embedded but tests use mocks, so this is a sanity check)

**Step 4: Commit**

```bash
git add migrations/000003_add_share_expires_at.up.sql migrations/000003_add_share_expires_at.down.sql
git commit -m "add share_expires_at column to videos table"
```

---

## Task 2: Watch API — Expire Check

Modify the `Watch` handler to distinguish between "video not found" and "link expired." The query adds `share_expires_at` to the SELECT. If the video exists but is expired, return a JSON response with `{"error": "link expired"}` and status 410 (Gone).

**Files:**
- Modify: `internal/video/video.go` — `Watch` method (lines 251-285)
- Modify: `internal/video/video_test.go` — Watch tests (lines 787-921)

**Step 1: Write failing tests**

Add two new tests to `video_test.go`:

`TestWatch_ExpiredLink` — mock returns a row where `share_expires_at` is in the past. Expect HTTP 410 with error `"link expired"`.

`TestWatch_ActiveLink` — mock returns a row where `share_expires_at` is in the future. Expect HTTP 200 (existing success behavior).

Update `TestWatch_Success` — the mock query must now return `share_expires_at` as an additional column. Set it to a future time.

Update `TestWatch_VideoNotFound` and `TestWatch_StorageError` — their mock queries must match the new SELECT that includes `share_expires_at`.

The new Watch query will be:
```sql
SELECT v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at
 FROM videos v
 JOIN users u ON u.id = v.user_id
 WHERE v.share_token = $1 AND v.status = 'ready'
```

The handler checks `share_expires_at` in Go after scanning, not in the WHERE clause. This lets us distinguish "not found" from "expired."

**Step 2: Run tests to verify they fail**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestWatch" -v`
Expected: FAIL — query mismatch because the handler still uses the old SELECT

**Step 3: Implement the Watch handler changes**

In `video.go`, modify the `Watch` method:

1. Add `var shareExpiresAt time.Time` variable
2. Update the SQL query to `SELECT v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`
3. Add `&shareExpiresAt` to the `.Scan()` call
4. After the scan succeeds, check `if time.Now().After(shareExpiresAt)` — if so, return `httputil.WriteError(w, http.StatusGone, "link expired")`
5. Rest of the handler stays the same

**Step 4: Run tests to verify they pass**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestWatch" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/video/video.go internal/video/video_test.go
git commit -m "return 410 Gone for expired share links in Watch API"
```

---

## Task 3: WatchPage — Expired State

Modify the `WatchPage` handler to show an "expired" HTML page instead of 404 when a share link is expired.

**Files:**
- Modify: `internal/video/watch_page.go` — `WatchPage` method (lines 92-127), add expired page template
- Modify: `internal/video/video_test.go` — WatchPage tests (lines 1008-1140)

**Step 1: Write failing test**

Add `TestWatchPage_ExpiredLink` — mock returns a row with `share_expires_at` in the past. Expect HTTP 410 with HTML body containing "This link has expired".

Update `TestWatchPage_Success` — mock query returns `share_expires_at` column (future time).

Update `TestWatchPage_VideoNotFound` and `TestWatchPage_StorageError` — mock queries match new SELECT with `share_expires_at`.

The new WatchPage query will be:
```sql
SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at
 FROM videos v
 JOIN users u ON u.id = v.user_id
 WHERE v.share_token = $1 AND v.status = 'ready'
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestWatchPage" -v`
Expected: FAIL

**Step 3: Implement**

In `watch_page.go`:

1. Add an `expiredPageTemplate` — same styling as watch page but with message "This link has expired" and a "Go to SendRec" button linking to `https://sendrec.eu`
2. Modify `WatchPage` handler: update query to select `share_expires_at`, scan it, check if expired. If expired, render expired template with 410 status.

```go
var expiredPageTemplate = template.Must(template.New("expired").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Link Expired — SendRec</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            background: #0a1628;
            color: #ffffff;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container { text-align: center; padding: 2rem; }
        h1 { font-size: 1.5rem; margin-bottom: 0.75rem; }
        p { color: #94a3b8; margin-bottom: 1.5rem; }
        a {
            display: inline-block;
            background: #00b67a;
            color: #fff;
            padding: 0.625rem 1.5rem;
            border-radius: 8px;
            text-decoration: none;
            font-weight: 600;
        }
        a:hover { opacity: 0.9; }
    </style>
</head>
<body>
    <div class="container">
        <h1>This link has expired</h1>
        <p>The video owner can extend the link to make it available again.</p>
        <a href="https://sendrec.eu">Go to SendRec</a>
    </div>
</body>
</html>`))
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestWatchPage" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/video/watch_page.go internal/video/video_test.go
git commit -m "render expired page for expired share links"
```

---

## Task 4: Extend Endpoint

New `POST /api/videos/{id}/extend` endpoint. Requires auth. Resets `share_expires_at` to `now() + 7 days`. Only the video owner can extend. Returns 204.

**Files:**
- Modify: `internal/video/video.go` — add `Extend` method and `shareLinkDuration` constant
- Modify: `internal/video/video_test.go` — add Extend tests
- Modify: `internal/server/server.go:89-95` — add route

**Step 1: Write failing tests**

Add to `video_test.go`:

`TestExtend_Success` — mock expects `UPDATE videos SET share_expires_at = now() + INTERVAL '7 days', updated_at = now() WHERE id = $1 AND user_id = $2 AND status != 'deleted'` with args `(videoID, testUserID)`, returns 1 row affected. Expect HTTP 204.

`TestExtend_VideoNotFound` — mock returns 0 rows affected. Expect HTTP 404 with `"video not found"`.

`TestExtend_DatabaseError` — mock returns error. Expect HTTP 500.

**Step 2: Run tests to verify they fail**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestExtend" -v`
Expected: FAIL — `Extend` method doesn't exist

**Step 3: Implement**

In `video.go`, add a constant and method:

```go
const shareLinkDuration = "7 days"

func (h *Handler) Extend(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET share_expires_at = now() + INTERVAL '7 days', updated_at = now()
		 WHERE id = $1 AND user_id = $2 AND status != 'deleted'`,
		videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to extend share link")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

In `server.go`, add the route inside the `/api/videos` group (line 93):
```go
r.Post("/{id}/extend", s.videoHandler.Extend)
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestExtend" -v`
Expected: PASS

**Step 5: Run all tests**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make test`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/video/video.go internal/video/video_test.go internal/server/server.go
git commit -m "add POST /api/videos/{id}/extend endpoint"
```

---

## Task 5: List API — Include Expiry Info

Add `shareExpiresAt` to the List API response so the frontend can show expiry status.

**Files:**
- Modify: `internal/video/video.go` — `listItem` struct (line 47-55), `List` method (lines 182-226)
- Modify: `internal/video/video_test.go` — List tests (lines 516-705)

**Step 1: Write failing test**

Update `TestList_SuccessWithVideos`:
- Add `share_expires_at` to the mock query's expected SELECT columns
- Add a future `time.Time` value to each `AddRow` call
- Assert `items[0].ShareExpiresAt` equals the expected RFC3339 string

Update all other List tests (`TestList_ShareURLIncludesBaseURL`, `TestList_EmptyList`, `TestList_DatabaseError`) to match the new SELECT query (add `share_expires_at` column to mocked rows).

**Step 2: Run tests to verify they fail**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestList" -v`
Expected: FAIL — mock expects the new column but handler queries old columns

**Step 3: Implement**

1. Add field to `listItem` struct:
```go
ShareExpiresAt string `json:"shareExpiresAt"`
```

2. Update List query to:
```sql
SELECT id, title, status, duration, share_token, created_at, share_expires_at
 FROM videos
 WHERE user_id = $1 AND status != 'deleted'
 ORDER BY created_at DESC
 LIMIT $2 OFFSET $3
```

3. Update `rows.Scan` to include `&shareExpiresAt` (as `time.Time`), then set `item.ShareExpiresAt = shareExpiresAt.Format(time.RFC3339)`.

**Step 4: Run tests to verify they pass**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestList" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/video/video.go internal/video/video_test.go
git commit -m "include shareExpiresAt in video list API response"
```

---

## Task 6: Frontend — Library Expiry Status and Extend Button

Show expiry status on each video card in the Library. Add an "Extend" button that calls `POST /api/videos/{id}/extend`.

**Files:**
- Modify: `web/src/pages/Library.tsx`

**Step 1: Update the Video interface**

Add `shareExpiresAt: string` to the `Video` interface.

**Step 2: Add helper function**

```typescript
function expiryLabel(shareExpiresAt: string): { text: string; expired: boolean } {
  const expiry = new Date(shareExpiresAt);
  const now = new Date();
  if (expiry <= now) {
    return { text: "Expired", expired: true };
  }
  const diffMs = expiry.getTime() - now.getTime();
  const diffDays = Math.ceil(diffMs / (1000 * 60 * 60 * 24));
  if (diffDays === 1) {
    return { text: "Expires tomorrow", expired: false };
  }
  return { text: `Expires in ${diffDays} days`, expired: false };
}
```

**Step 3: Add extend function**

```typescript
async function extendVideo(id: string) {
  await apiFetch(`/api/videos/${id}/extend`, { method: "POST" });
  // Refresh the list
  const result = await apiFetch<Video[]>("/api/videos");
  setVideos(result ?? []);
}
```

**Step 4: Update the video card rendering**

In the metadata line (below duration/date), add the expiry label:
- If `video.status === "ready"`, show the expiry text
- Style expired text in `var(--color-error)` and active expiry in `var(--color-text-secondary)`

In the button group, add an "Extend" button (shown when status is "ready"):
```tsx
<button onClick={() => extendVideo(video.id)} style={{...}}>
  Extend
</button>
```

**Step 5: Verify frontend builds**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app/web && pnpm typecheck && pnpm build`
Expected: PASS

**Step 6: Commit**

```bash
git add web/src/pages/Library.tsx
git commit -m "show expiry status and extend button in video library"
```

---

## Task 7: Frontend — Watch Page Expired State

The watch page HTML is server-rendered (Task 3 handled the backend). The React SPA also fetches `/api/watch/{shareToken}` — handle the 410 response in the frontend.

**Files:**
- Modify: `web/src/api/client.ts` — Don't throw on 410, or handle it in the page
- Check if there's a React watch page component. If not, this task only applies to the API error handling.

Based on the codebase exploration: the React SPA doesn't have a watch page component. The `/watch/{shareToken}` route is handled server-side by `WatchPage` (HTML template). The `/api/watch/{shareToken}` endpoint is used by the HTML template's JavaScript.

Since the watch page is fully server-rendered and Task 3 already handles the expired state with an HTML template, **this task is complete with no frontend changes needed.**

**Step 1: Verify the full test suite passes**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make test`
Expected: PASS

Run: `cd /Users/aneamtu/Development/personal/sendrec/app/web && pnpm typecheck && pnpm build`
Expected: PASS

---

## Summary

| Task | What | Files |
|------|------|-------|
| 1 | DB migration: `share_expires_at` column | `migrations/000003_*` |
| 2 | Watch API: return 410 for expired links | `video.go`, `video_test.go` |
| 3 | WatchPage: render expired HTML template | `watch_page.go`, `video_test.go` |
| 4 | Extend endpoint: `POST /api/videos/{id}/extend` | `video.go`, `video_test.go`, `server.go` |
| 5 | List API: include `shareExpiresAt` | `video.go`, `video_test.go` |
| 6 | Frontend: Library expiry + extend button | `Library.tsx` |
| 7 | Verification: full test suite | (no changes) |
