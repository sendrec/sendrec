# View Counts Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Track anonymous video views and display total + unique counts in the library.

**Architecture:** New `video_views` table stores one row per watch API call. Viewer identity is a truncated SHA-256 hash of IP + User-Agent (no PII). The List API adds correlated subqueries for total and unique counts. Frontend shows counts inline in the library card metadata.

**Tech Stack:** Go (crypto/sha256, net/http), PostgreSQL, pgxmock, React/TypeScript

---

### Task 1: Migration — create video_views table

**Files:**
- Create: `migrations/000006_create_video_views.up.sql`
- Create: `migrations/000006_create_video_views.down.sql`

**Step 1: Write the up migration**

```sql
CREATE TABLE video_views (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id UUID NOT NULL REFERENCES videos(id),
    viewer_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_video_views_video_id ON video_views(video_id);
```

**Step 2: Write the down migration**

```sql
DROP TABLE IF EXISTS video_views;
```

**Step 3: Commit**

```bash
git add migrations/000006_create_video_views.up.sql migrations/000006_create_video_views.down.sql
git commit -m "Add video_views table migration"
```

---

### Task 2: Viewer hash helper + tests

**Files:**
- Modify: `internal/video/video.go` (add `viewerHash` function and `clientIP` helper)
- Modify: `internal/video/video_test.go` (add tests)

**Step 1: Write failing tests**

Add to `internal/video/video_test.go`:

```go
// --- viewerHash Tests ---

func TestViewerHash_DeterministicOutput(t *testing.T) {
	hash1 := viewerHash("192.168.1.1", "Mozilla/5.0")
	hash2 := viewerHash("192.168.1.1", "Mozilla/5.0")
	if hash1 != hash2 {
		t.Errorf("expected identical hashes, got %q and %q", hash1, hash2)
	}
}

func TestViewerHash_DifferentIPProducesDifferentHash(t *testing.T) {
	hash1 := viewerHash("192.168.1.1", "Mozilla/5.0")
	hash2 := viewerHash("10.0.0.1", "Mozilla/5.0")
	if hash1 == hash2 {
		t.Error("expected different hashes for different IPs")
	}
}

func TestViewerHash_DifferentUAProducesDifferentHash(t *testing.T) {
	hash1 := viewerHash("192.168.1.1", "Mozilla/5.0")
	hash2 := viewerHash("192.168.1.1", "Chrome/120")
	if hash1 == hash2 {
		t.Error("expected different hashes for different user agents")
	}
}

func TestViewerHash_Returns16Characters(t *testing.T) {
	hash := viewerHash("192.168.1.1", "Mozilla/5.0")
	if len(hash) != 16 {
		t.Errorf("expected 16-character hash, got %d: %q", len(hash), hash)
	}
}

// --- clientIP Tests ---

func TestClientIP_UsesXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18")
	if ip := clientIP(req); ip != "203.0.113.50" {
		t.Errorf("expected %q, got %q", "203.0.113.50", ip)
	}
}

func TestClientIP_FallsBackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	if ip := clientIP(req); ip != "192.168.1.100:54321" {
		t.Errorf("expected %q, got %q", "192.168.1.100:54321", ip)
	}
}

func TestClientIP_SingleXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	if ip := clientIP(req); ip != "203.0.113.50" {
		t.Errorf("expected %q, got %q", "203.0.113.50", ip)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestViewerHash|TestClientIP" -v`
Expected: FAIL — `viewerHash` and `clientIP` undefined

**Step 3: Write minimal implementation**

Add to `internal/video/video.go` (add `"crypto/sha256"` and `"strings"` to imports):

```go
func viewerHash(ip, userAgent string) string {
	h := sha256.Sum256([]byte(ip + "|" + userAgent))
	return fmt.Sprintf("%x", h[:8])
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if first, _, ok := strings.Cut(forwarded, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(forwarded)
	}
	return r.RemoteAddr
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestViewerHash|TestClientIP" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/video/video.go internal/video/video_test.go
git commit -m "Add viewer hash and client IP helpers for view tracking"
```

---

### Task 3: Record view in Watch handler + tests

**Files:**
- Modify: `internal/video/video.go` (add insert to `Watch` handler)
- Modify: `internal/video/video_test.go` (update Watch tests)

The Watch handler (`GET /api/watch/{shareToken}`) needs to record a view after confirming the video is valid and not expired, before returning the JSON response. The insert runs in a fire-and-forget goroutine so it never blocks the response.

**Step 1: Write failing test**

Add to `internal/video/video_test.go`:

```go
func TestWatch_RecordsView(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	downloadURL := "https://s3.example.com/download?signed=xyz"
	storage := &mockStorage{downloadURL: downloadURL}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)
	videoID := "video-001"

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "duration", "file_key", "name", "created_at", "share_expires_at"}).
				AddRow(videoID, "Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt),
		)

	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs(videoID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.Watch)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	req.Header.Set("User-Agent", "TestBrowser/1.0")
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	// Give goroutine time to execute INSERT
	time.Sleep(100 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestWatch_RecordsView" -v`
Expected: FAIL — the Watch query doesn't select `v.id`, and no INSERT INTO video_views happens

**Step 3: Modify Watch handler**

In `internal/video/video.go`, update the `Watch` method:

1. Add `videoID` to the variables scanned from DB:
   - Add `var videoID string`
   - Update the query to `SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`
   - Update `.Scan(&videoID, &title, &duration, &fileKey, &creator, &createdAt, &shareExpiresAt)`

2. After the expiry check and before generating the download URL, add the fire-and-forget view insert:
```go
go func() {
    ip := clientIP(r)
    hash := viewerHash(ip, r.UserAgent())
    if _, err := h.db.Exec(context.Background(),
        `INSERT INTO video_views (video_id, viewer_hash) VALUES ($1, $2)`,
        videoID, hash,
    ); err != nil {
        log.Printf("failed to record view for %s: %v", videoID, err)
    }
}()
```

**Step 4: Update existing Watch tests**

The existing `TestWatch_Success`, `TestWatch_StorageError`, and `TestWatch_ExpiredLink` tests need their mock query expectations updated to match the new SELECT that includes `v.id`. Update the column list in `ExpectQuery` and add `videoID` to `AddRow`. For `TestWatch_Success` and `TestWatch_StorageError` (non-expired), also add `mock.ExpectExec("INSERT INTO video_views")` expectation + `time.Sleep(100 * time.Millisecond)` before `ExpectationsWereMet`. For `TestWatch_ExpiredLink`, no INSERT expectation is needed (expired links should NOT record a view).

**Step 5: Run all Watch tests**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestWatch" -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/video/video.go internal/video/video_test.go
git commit -m "Record anonymous view on watch API calls"
```

---

### Task 4: Add view counts to List API + tests

**Files:**
- Modify: `internal/video/video.go` (update `listItem` struct and `List` query)
- Modify: `internal/video/video_test.go` (update List tests)

**Step 1: Write failing test**

Add to `internal/video/video_test.go`:

```go
func TestList_IncludesViewCounts(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	createdAt := time.Date(2026, 2, 5, 10, 30, 0, 0, time.UTC)
	shareExpiresAt := createdAt.Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.status, v.duration, v.share_token, v.created_at, v.share_expires_at`).
		WithArgs(testUserID, 50, 0).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "status", "duration", "share_token", "created_at", "share_expires_at", "view_count", "unique_view_count"}).
				AddRow("video-1", "First Video", "ready", 120, "abc123defghi", createdAt, shareExpiresAt, int64(15), int64(8)),
		)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos", handler.List)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var items []listItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ViewCount != 15 {
		t.Errorf("expected ViewCount 15, got %d", items[0].ViewCount)
	}
	if items[0].UniqueViewCount != 8 {
		t.Errorf("expected UniqueViewCount 8, got %d", items[0].UniqueViewCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestList_IncludesViewCounts" -v`
Expected: FAIL — `listItem` has no `ViewCount` field

**Step 3: Implement changes**

In `internal/video/video.go`:

1. Add fields to `listItem` struct:
```go
ViewCount       int64  `json:"viewCount"`
UniqueViewCount int64  `json:"uniqueViewCount"`
```

2. Update the List handler query:
```go
rows, err := h.db.Query(r.Context(),
    `SELECT v.id, v.title, v.status, v.duration, v.share_token, v.created_at, v.share_expires_at,
        (SELECT COUNT(*) FROM video_views vv WHERE vv.video_id = v.id) AS view_count,
        (SELECT COUNT(DISTINCT vv.viewer_hash) FROM video_views vv WHERE vv.video_id = v.id) AS unique_view_count
     FROM videos v
     WHERE v.user_id = $1 AND v.status != 'deleted'
     ORDER BY v.created_at DESC
     LIMIT $2 OFFSET $3`,
    userID, limit, offset,
)
```

3. Update the Scan call to include the two new fields:
```go
if err := rows.Scan(&item.ID, &item.Title, &item.Status, &item.Duration, &item.ShareToken, &createdAt, &shareExpiresAt, &item.ViewCount, &item.UniqueViewCount); err != nil {
```

**Step 4: Update existing List tests**

The existing `TestList_SuccessWithVideos`, `TestList_ShareURLIncludesBaseURL`, `TestList_EmptyList`, and `TestList_DatabaseError` tests need their mock expectations updated:
- Change query pattern from `SELECT id, title, status` to `SELECT v.id, v.title, v.status`
- For success tests, add `view_count` and `unique_view_count` to the `NewRows` columns and `AddRow` values (use `int64(0)` for simplicity)

**Step 5: Run all List tests**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run "TestList" -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/video/video.go internal/video/video_test.go
git commit -m "Add view counts to video list API"
```

---

### Task 5: Display view counts in Library UI

**Files:**
- Modify: `web/src/pages/Library.tsx`

**Step 1: Update Video interface**

Add to the `Video` interface:
```typescript
viewCount: number;
uniqueViewCount: number;
```

**Step 2: Add view count display to library cards**

In the metadata `<p>` element (after the duration and date), add view count display:

```tsx
{video.status === "ready" && video.viewCount > 0 && (
  <span style={{ marginLeft: 8 }}>
    &middot; {video.viewCount === video.uniqueViewCount
      ? `${video.viewCount} view${video.viewCount !== 1 ? "s" : ""}`
      : `${video.viewCount} views (${video.uniqueViewCount} unique)`}
  </span>
)}
{video.status === "ready" && video.viewCount === 0 && (
  <span style={{ color: "var(--color-text-secondary)", marginLeft: 8, opacity: 0.6 }}>
    &middot; No views yet
  </span>
)}
```

**Step 3: Verify frontend builds**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app/web && pnpm typecheck && pnpm build`
Expected: PASS

**Step 4: Commit**

```bash
git add web/src/pages/Library.tsx
git commit -m "Display view counts in video library"
```

---

### Task 6: Run full test suite and verify

**Step 1: Run all Go tests**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make test`
Expected: PASS

**Step 2: Run linter**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make lint`
Expected: PASS

**Step 3: Build frontend**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app/web && pnpm typecheck && pnpm build`
Expected: PASS

---

## Summary

| File | Action |
|------|--------|
| `migrations/000006_create_video_views.up.sql` | Create |
| `migrations/000006_create_video_views.down.sql` | Create |
| `internal/video/video.go` | Modify (add `viewerHash`, `clientIP`, view insert in Watch, view counts in List) |
| `internal/video/video_test.go` | Modify (add hash/IP tests, view recording test, view count test, update existing mocks) |
| `web/src/pages/Library.tsx` | Modify (add viewCount/uniqueViewCount to interface + display) |
