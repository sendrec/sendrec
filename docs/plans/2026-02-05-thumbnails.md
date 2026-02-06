# Thumbnail Generation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Generate server-side JPEG thumbnails from uploaded videos using ffmpeg and display them in the library, watch page, and as OG images for social sharing.

**Architecture:** After a video's status is set to "ready", an async goroutine downloads the WebM from S3 to a temp file, runs ffmpeg to extract a frame at 2 seconds, uploads the JPEG thumbnail back to S3, and stores the key in the database. The List and Watch APIs return a presigned `thumbnailUrl`. The watch page uses it for `poster` and `og:image`. Deletion cleans up both video and thumbnail files.

**Tech Stack:** Go, ffmpeg (Alpine apk), S3/MinIO, PostgreSQL, React/TypeScript

---

### Task 1: Migration 000007 — add `thumbnail_key` column

**Files:**
- Create: `migrations/000007_add_thumbnail_key.up.sql`
- Create: `migrations/000007_add_thumbnail_key.down.sql`

**Step 1: Write up migration**

```sql
-- migrations/000007_add_thumbnail_key.up.sql
ALTER TABLE videos ADD COLUMN thumbnail_key TEXT;
```

**Step 2: Write down migration**

```sql
-- migrations/000007_add_thumbnail_key.down.sql
ALTER TABLE videos DROP COLUMN IF EXISTS thumbnail_key;
```

**Step 3: Verify migrations compile**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make build`
Expected: Build succeeds (migrations are embedded via `migrations.go`)

**Step 4: Commit**

```
feat: add thumbnail_key column migration
```

---

### Task 2: Add `DownloadToFile` to ObjectStorage interface and implementation

**Files:**
- Modify: `internal/video/video.go` (ObjectStorage interface)
- Modify: `internal/storage/storage.go` (implementation)
- Modify: `internal/video/video_test.go` (mockStorage)
- Modify: `internal/server/server_test.go` (mockStorage)

**Step 1: Write failing test — mockStorage must implement updated interface**

Add `DownloadToFile` to the `ObjectStorage` interface in `internal/video/video.go`:

```go
type ObjectStorage interface {
	GenerateUploadURL(ctx context.Context, key string, contentType string, contentLength int64, expiry time.Duration) (string, error)
	GenerateDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	DeleteObject(ctx context.Context, key string) error
	HeadObject(ctx context.Context, key string) (int64, string, error)
	DownloadToFile(ctx context.Context, key string, destPath string) error
}
```

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make test`
Expected: FAIL — `mockStorage` and `mockStorage` in `server_test.go` don't implement `DownloadToFile`

**Step 2: Update mock in `video_test.go`**

Add to `mockStorage` struct:

```go
downloadToFileErr error
```

Add method:

```go
func (m *mockStorage) DownloadToFile(_ context.Context, _ string, _ string) error {
	return m.downloadToFileErr
}
```

**Step 3: Update mock in `server_test.go`**

Add method to the `mockStorage` in `internal/server/server_test.go`:

```go
func (m *mockStorage) DownloadToFile(_ context.Context, _ string, _ string) error {
	return nil
}
```

**Step 4: Implement `DownloadToFile` in `internal/storage/storage.go`**

```go
func (s *Storage) DownloadToFile(ctx context.Context, key string, destPath string) error {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("get object %s: %w", key, err)
	}
	defer out.Body.Close()

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", destPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, out.Body); err != nil {
		return fmt.Errorf("write file %s: %w", destPath, err)
	}

	return nil
}
```

Add `"io"` and `"os"` imports to `storage.go`.

**Step 5: Add `UploadFile` to ObjectStorage interface and implementation**

We also need a method to upload the generated thumbnail back to S3:

Add to `ObjectStorage` interface in `internal/video/video.go`:

```go
UploadFile(ctx context.Context, key string, filePath string, contentType string) error
```

Add to `mockStorage` in `video_test.go`:

```go
uploadFileErr error
```

```go
func (m *mockStorage) UploadFile(_ context.Context, _ string, _ string, _ string) error {
	return m.uploadFileErr
}
```

Add to `mockStorage` in `server_test.go`:

```go
func (m *mockStorage) UploadFile(_ context.Context, _ string, _ string, _ string) error {
	return nil
}
```

Implement in `internal/storage/storage.go`:

```go
func (s *Storage) UploadFile(ctx context.Context, key string, filePath string, contentType string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file %s: %w", filePath, err)
	}
	defer f.Close()

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        f,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("upload file %s: %w", key, err)
	}

	return nil
}
```

**Step 6: Run tests**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make test`
Expected: All tests pass

**Step 7: Commit**

```
feat: add DownloadToFile and UploadFile to ObjectStorage interface
```

---

### Task 3: Implement `GenerateThumbnail` function

**Files:**
- Create: `internal/video/thumbnail.go`
- Create: `internal/video/thumbnail_test.go`

**Step 1: Write failing test for `thumbnailFileKey`**

In `internal/video/thumbnail_test.go`:

```go
package video

import "testing"

func TestThumbnailFileKey(t *testing.T) {
	key := thumbnailFileKey("user-123", "abc123defghi")
	expected := "recordings/user-123/abc123defghi.jpg"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}
```

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run TestThumbnailFileKey -v`
Expected: FAIL — `thumbnailFileKey` not defined

**Step 2: Implement `thumbnailFileKey`**

In `internal/video/thumbnail.go`:

```go
package video

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/sendrec/sendrec/internal/database"
)

func thumbnailFileKey(userID, shareToken string) string {
	return fmt.Sprintf("recordings/%s/%s.jpg", userID, shareToken)
}
```

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run TestThumbnailFileKey -v`
Expected: PASS

**Step 3: Write failing test for `extractFrame`**

`extractFrame` wraps the ffmpeg command. We test the command construction but skip if ffmpeg is not installed (CI may not have it).

In `internal/video/thumbnail_test.go`:

```go
func TestExtractFrame_InvalidInput(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	err := extractFrame("/nonexistent/input.webm", "/tmp/output.jpg")
	if err == nil {
		t.Error("expected error for nonexistent input file")
	}
}
```

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run TestExtractFrame -v`
Expected: FAIL — `extractFrame` not defined

**Step 4: Implement `extractFrame`**

In `internal/video/thumbnail.go`:

```go
func extractFrame(inputPath, outputPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-ss", "2",
		"-frames:v", "1",
		"-vf", "scale=640:360:force_original_aspect_ratio=decrease,pad=640:360:(ow-iw)/2:(oh-ih)/2",
		"-q:v", "5",
		"-y",
		outputPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, string(output))
	}
	return nil
}
```

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run TestExtractFrame -v`
Expected: PASS (or SKIP if no ffmpeg)

**Step 5: Write test for `GenerateThumbnail` with mocked storage/DB**

In `internal/video/thumbnail_test.go`:

```go
func TestGenerateThumbnail_StorageDownloadError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{downloadToFileErr: fmt.Errorf("s3 down")}

	GenerateThumbnail(context.Background(), mock, s, "video-123", "recordings/user/abc.webm", "recordings/user/abc.jpg")

	// Should log the error but not panic. No DB update expected since download failed.
}
```

Add needed imports: `"context"`, `"fmt"`, `"github.com/pashagolub/pgxmock/v4"`.

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run TestGenerateThumbnail -v`
Expected: FAIL — `GenerateThumbnail` not defined

**Step 6: Implement `GenerateThumbnail`**

In `internal/video/thumbnail.go`:

```go
func GenerateThumbnail(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey, thumbnailKey string) {
	tmpVideo, err := os.CreateTemp("", "sendrec-thumb-*.webm")
	if err != nil {
		log.Printf("thumbnail: failed to create temp video file: %v", err)
		return
	}
	tmpVideoPath := tmpVideo.Name()
	tmpVideo.Close()
	defer os.Remove(tmpVideoPath)

	if err := storage.DownloadToFile(ctx, fileKey, tmpVideoPath); err != nil {
		log.Printf("thumbnail: failed to download video %s: %v", videoID, err)
		return
	}

	tmpThumb, err := os.CreateTemp("", "sendrec-thumb-*.jpg")
	if err != nil {
		log.Printf("thumbnail: failed to create temp thumbnail file: %v", err)
		return
	}
	tmpThumbPath := tmpThumb.Name()
	tmpThumb.Close()
	defer os.Remove(tmpThumbPath)

	if err := extractFrame(tmpVideoPath, tmpThumbPath); err != nil {
		log.Printf("thumbnail: ffmpeg failed for video %s: %v", videoID, err)
		return
	}

	if err := storage.UploadFile(ctx, thumbnailKey, tmpThumbPath, "image/jpeg"); err != nil {
		log.Printf("thumbnail: failed to upload thumbnail for video %s: %v", videoID, err)
		return
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET thumbnail_key = $1, updated_at = now() WHERE id = $2`,
		thumbnailKey, videoID,
	); err != nil {
		log.Printf("thumbnail: failed to update thumbnail_key for video %s: %v", videoID, err)
	}
}
```

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run TestGenerateThumbnail -v`
Expected: PASS

**Step 7: Run all tests**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make test`
Expected: All tests pass

**Step 8: Commit**

```
feat: add GenerateThumbnail function with ffmpeg frame extraction
```

---

### Task 4: Trigger thumbnail generation in Update handler

**Files:**
- Modify: `internal/video/video.go` (Update handler)
- Modify: `internal/video/video_test.go` (Update tests)

**Step 1: Update Update handler to fetch share_token and user_id for thumbnail key**

The Update handler currently queries `file_key` and `file_size` when status="ready". We also need `share_token` to construct the thumbnail key. Modify the SELECT query:

In `internal/video/video.go`, Update handler, change:

```go
var fileKey string
var fileSize int64
err := h.db.QueryRow(r.Context(),
	`SELECT file_key, file_size FROM videos
	 WHERE id = $1 AND user_id = $2 AND status = 'uploading'`,
	videoID, userID,
).Scan(&fileKey, &fileSize)
```

To:

```go
var fileKey string
var fileSize int64
var shareToken string
err := h.db.QueryRow(r.Context(),
	`SELECT file_key, file_size, share_token FROM videos
	 WHERE id = $1 AND user_id = $2 AND status = 'uploading'`,
	videoID, userID,
).Scan(&fileKey, &fileSize, &shareToken)
```

Then after the status UPDATE succeeds, add the async goroutine:

```go
go GenerateThumbnail(
	context.Background(),
	h.db, h.storage,
	videoID, fileKey,
	thumbnailFileKey(userID, shareToken),
)
```

**Step 2: Update existing Update tests**

Tests that mock `SELECT file_key, file_size FROM videos` need updating to also return `share_token`.

In `TestUpdate_Success`, change:

```go
mock.ExpectQuery(`SELECT file_key, file_size FROM videos`).
	WithArgs(videoID, testUserID).
	WillReturnRows(pgxmock.NewRows([]string{"file_key", "file_size"}).AddRow(fileKey, fileSize))
```

To:

```go
mock.ExpectQuery(`SELECT file_key, file_size, share_token FROM videos`).
	WithArgs(videoID, testUserID).
	WillReturnRows(pgxmock.NewRows([]string{"file_key", "file_size", "share_token"}).AddRow(fileKey, fileSize, "abc123defghi"))
```

Apply the same pattern to `TestUpdate_VideoNotFound` and `TestUpdate_DatabaseError`.

For `TestUpdate_Success`, also add mock expectations for the async thumbnail goroutine (DownloadToFile will be called). Since this is async and uses mockStorage which returns an error by default for `downloadToFileErr` (nil), the goroutine will proceed to call ffmpeg which won't exist in test. The simplest approach is to accept that the goroutine will log errors in tests — it's fire-and-forget and doesn't affect the HTTP response.

**Step 3: Run tests**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make test`
Expected: All tests pass (goroutine may log errors from ffmpeg not being available, but tests should pass)

**Step 4: Commit**

```
feat: trigger async thumbnail generation when video status set to ready
```

---

### Task 5: Add `thumbnailUrl` to List and Watch APIs

**Files:**
- Modify: `internal/video/video.go` (List handler, Watch handler, response structs)
- Modify: `internal/video/video_test.go` (List and Watch tests)

**Step 1: Write failing test — List response includes thumbnailUrl**

In `internal/video/video_test.go`, add a new test:

```go
func TestList_IncludesThumbnailUrl(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	downloadURL := "https://s3.example.com/thumbnail?signed=xyz"
	storage := &mockStorage{downloadURL: downloadURL}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	createdAt := time.Date(2026, 2, 5, 10, 30, 0, 0, time.UTC)
	shareExpiresAt := createdAt.Add(7 * 24 * time.Hour)
	thumbnailKey := "recordings/user-1/abc123defghi.jpg"

	mock.ExpectQuery(`SELECT v.id, v.title, v.status, v.duration, v.share_token, v.created_at, v.share_expires_at`).
		WithArgs(testUserID, 50, 0).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "status", "duration", "share_token", "created_at", "share_expires_at", "view_count", "unique_view_count", "thumbnail_key"}).
				AddRow("video-1", "My Video", "ready", 120, "abc123defghi", createdAt, shareExpiresAt, int64(5), int64(3), &thumbnailKey),
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

	if items[0].ThumbnailURL == "" {
		t.Error("expected non-empty thumbnailUrl")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
```

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run TestList_IncludesThumbnailUrl -v`
Expected: FAIL — `ThumbnailURL` field doesn't exist on `listItem`

**Step 2: Update `listItem` struct and List handler**

In `internal/video/video.go`:

Add to `listItem`:
```go
ThumbnailURL string `json:"thumbnailUrl,omitempty"`
```

Update the List query to also select `v.thumbnail_key`:

```go
rows, err := h.db.Query(r.Context(),
	`SELECT v.id, v.title, v.status, v.duration, v.share_token, v.created_at, v.share_expires_at,
	    (SELECT COUNT(*) FROM video_views vv WHERE vv.video_id = v.id) AS view_count,
	    (SELECT COUNT(DISTINCT vv.viewer_hash) FROM video_views vv WHERE vv.video_id = v.id) AS unique_view_count,
	    v.thumbnail_key
	 FROM videos v
	 WHERE v.user_id = $1 AND v.status != 'deleted'
	 ORDER BY v.created_at DESC
	 LIMIT $2 OFFSET $3`,
	userID, limit, offset,
)
```

Update the Scan to include `thumbnailKey`:

```go
var thumbnailKey *string
if err := rows.Scan(&item.ID, &item.Title, &item.Status, &item.Duration, &item.ShareToken, &createdAt, &shareExpiresAt, &item.ViewCount, &item.UniqueViewCount, &thumbnailKey); err != nil {
```

After scan, generate presigned URL if thumbnail exists:

```go
if thumbnailKey != nil {
	thumbURL, err := h.storage.GenerateDownloadURL(r.Context(), *thumbnailKey, 1*time.Hour)
	if err == nil {
		item.ThumbnailURL = thumbURL
	}
}
```

**Step 3: Update existing List tests**

All existing List tests that mock the query need the `thumbnail_key` column added. For tests without thumbnails, use `nil`:

In `TestList_SuccessWithVideos`, `TestList_ShareURLIncludesBaseURL`, `TestList_EmptyList`, `TestList_DatabaseError`, `TestList_IncludesViewCounts`:

Update mock rows to include `"thumbnail_key"` column. For non-thumbnail rows, pass `(*string)(nil)`.

Example for `TestList_SuccessWithVideos`:
```go
pgxmock.NewRows([]string{"id", "title", "status", "duration", "share_token", "created_at", "share_expires_at", "view_count", "unique_view_count", "thumbnail_key"}).
	AddRow("video-1", "First Video", "ready", 120, "abc123defghi", createdAt, shareExpiresAt, int64(0), int64(0), (*string)(nil)).
	AddRow("video-2", "Second Video", "uploading", 60, "xyz789uvwklm", createdAt.Add(-time.Hour), shareExpiresAt, int64(0), int64(0), (*string)(nil)),
```

**Step 4: Update Watch API response**

Add to `watchResponse`:
```go
ThumbnailURL string `json:"thumbnailUrl,omitempty"`
```

Update Watch handler query to also select `v.thumbnail_key`:

```go
var thumbnailKey *string

err := h.db.QueryRow(r.Context(),
	`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key
	 FROM videos v
	 JOIN users u ON u.id = v.user_id
	 WHERE v.share_token = $1 AND v.status = 'ready'`,
	shareToken,
).Scan(&videoID, &title, &duration, &fileKey, &creator, &createdAt, &shareExpiresAt, &thumbnailKey)
```

After generating the video download URL, generate thumbnail URL:

```go
var thumbnailURL string
if thumbnailKey != nil {
	if u, err := h.storage.GenerateDownloadURL(r.Context(), *thumbnailKey, 1*time.Hour); err == nil {
		thumbnailURL = u
	}
}
```

Include in response:
```go
httputil.WriteJSON(w, http.StatusOK, watchResponse{
	Title:        title,
	VideoURL:     videoURL,
	Duration:     duration,
	Creator:      creator,
	CreatedAt:    createdAt.Format(time.RFC3339),
	ThumbnailURL: thumbnailURL,
})
```

**Step 5: Update Watch tests**

All Watch tests that mock the SELECT query need `v.thumbnail_key` added. Update mock queries from:

```go
mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
```

To:

```go
mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
```

And rows from:

```go
pgxmock.NewRows([]string{"id", "title", "duration", "file_key", "name", "created_at", "share_expires_at"}).
	AddRow(videoID, "Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt),
```

To:

```go
pgxmock.NewRows([]string{"id", "title", "duration", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key"}).
	AddRow(videoID, "Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt, (*string)(nil)),
```

Also update `TestWatchRouteRegisteredWithDB` in `internal/server/server_test.go` to match the new query pattern.

**Step 6: Run all tests**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make test`
Expected: All tests pass

**Step 7: Commit**

```
feat: include thumbnailUrl in List and Watch API responses
```

---

### Task 6: Add thumbnail to watch page template

**Files:**
- Modify: `internal/video/watch_page.go` (template + data struct)
- Modify: `internal/video/video_test.go` (WatchPage tests)

**Step 1: Write failing test**

In `internal/video/video_test.go`, add:

```go
func TestWatchPage_ContainsPosterAttribute(t *testing.T) {
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
	thumbnailKey := "recordings/user-1/abc123defghi.jpg"

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key"}).
				AddRow("Demo Recording", "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt, &thumbnailKey),
		)

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.WatchPage)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, `poster="`) {
		t.Error("expected watch page to contain poster attribute on video tag")
	}
	if !strings.Contains(body, `og:image`) {
		t.Error("expected watch page to contain og:image meta tag")
	}
}
```

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run TestWatchPage_ContainsPosterAttribute -v`
Expected: FAIL

**Step 2: Update watch page template and WatchPage handler**

In `internal/video/watch_page.go`:

Add to `watchPageData`:
```go
ThumbnailURL string
```

Update `watchPageTemplate` — add `og:image` meta tag and `poster` attribute:

After the existing `og:video:type` meta tag:
```html
{{if .ThumbnailURL}}<meta property="og:image" content="{{.ThumbnailURL}}">{{end}}
```

Update the video tag:
```html
<video id="player" controls{{if .ThumbnailURL}} poster="{{.ThumbnailURL}}"{{end}}>
```

Update `WatchPage` handler to query `v.thumbnail_key`:

```go
var thumbnailKey *string

err := h.db.QueryRow(r.Context(),
	`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key
	 FROM videos v
	 JOIN users u ON u.id = v.user_id
	 WHERE v.share_token = $1 AND v.status = 'ready'`,
	shareToken,
).Scan(&title, &fileKey, &creator, &createdAt, &shareExpiresAt, &thumbnailKey)
```

After generating `videoURL`, generate thumbnail URL:

```go
var thumbnailURL string
if thumbnailKey != nil {
	if u, err := h.storage.GenerateDownloadURL(r.Context(), *thumbnailKey, 1*time.Hour); err == nil {
		thumbnailURL = u
	}
}
```

Pass to template data:
```go
watchPageData{
	Title:        title,
	VideoURL:     videoURL,
	Creator:      creator,
	Date:         createdAt.Format("Jan 2, 2006"),
	Nonce:        nonce,
	ThumbnailURL: thumbnailURL,
}
```

**Step 3: Update existing WatchPage tests**

All WatchPage tests that mock `SELECT v.title, v.file_key` need `v.thumbnail_key` added to the query and rows:

Update mock expectations in: `TestWatchPage_Success`, `TestWatchPage_VideoNotFound`, `TestWatchPage_StorageError`, `TestWatchPage_ExpiredLink`, `TestWatchPage_ContainsNonceInStyleTag`, `TestWatchPage_ContainsNonceInScriptTag`, `TestWatchPage_ExpiredContainsNonce`.

Change query from:
```go
mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at`).
```
To:
```go
mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
```

And rows from:
```go
pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at"}).
	AddRow(...)
```
To:
```go
pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key"}).
	AddRow(..., (*string)(nil))
```

Also update `TestWatchPageRouteRegisteredWithDB` in `internal/server/server_test.go`.

**Step 4: Run all tests**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make test`
Expected: All tests pass

**Step 5: Commit**

```
feat: add poster attribute and og:image to watch page template
```

---

### Task 7: Add thumbnail to Library UI

**Files:**
- Modify: `web/src/pages/Library.tsx`

**Step 1: Add `thumbnailUrl` to Video interface**

```tsx
interface Video {
  // ... existing fields ...
  thumbnailUrl?: string;
}
```

**Step 2: Add thumbnail image to video card**

In the video card div, before the title/metadata section, add an optional thumbnail:

```tsx
{video.thumbnailUrl && (
  <img
    src={video.thumbnailUrl}
    alt=""
    style={{
      width: 120,
      height: 68,
      objectFit: "cover",
      borderRadius: 4,
      flexShrink: 0,
      background: "var(--color-border)",
    }}
  />
)}
```

**Step 3: Verify frontend build**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app/web && pnpm typecheck && pnpm build`
Expected: No errors

**Step 4: Commit**

```
feat: display video thumbnails in Library UI
```

---

### Task 8: Update Dockerfile to include ffmpeg

**Files:**
- Modify: `Dockerfile`

**Step 1: Add ffmpeg to final Alpine stage**

In `Dockerfile`, change:

```dockerfile
RUN apk add --no-cache ca-certificates
```

To:

```dockerfile
RUN apk add --no-cache ca-certificates ffmpeg
```

**Step 2: Verify Docker build**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && docker build -t sendrec-test .`
Expected: Build succeeds

**Step 3: Commit**

```
feat: add ffmpeg to Docker image for thumbnail generation
```

---

### Task 9: Update cleanup to delete thumbnails

**Files:**
- Modify: `internal/video/cleanup.go`
- Modify: `internal/video/video.go` (Delete handler)
- Modify: `internal/video/cleanup_test.go`

**Step 1: Write failing test for cleanup including thumbnails**

In `internal/video/cleanup_test.go`, add:

```go
func TestPurgeOrphanedFiles_DeletesThumbnails(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	deletedKeys := []string{}
	s := &trackingMockStorage{deletedKeys: &deletedKeys}

	thumbnailKey := "recordings/user-1/abc.jpg"

	mock.ExpectQuery(`SELECT file_key, thumbnail_key FROM videos`).
		WillReturnRows(
			pgxmock.NewRows([]string{"file_key", "thumbnail_key"}).
				AddRow("recordings/user-1/abc.webm", &thumbnailKey),
		)

	mock.ExpectExec(`UPDATE videos SET file_purged_at`).
		WithArgs("recordings/user-1/abc.webm").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	PurgeOrphanedFiles(context.Background(), mock, s)

	if len(deletedKeys) != 2 {
		t.Fatalf("expected 2 deletes (video + thumbnail), got %d: %v", len(deletedKeys), deletedKeys)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
```

You'll need a `trackingMockStorage` type for this test (or extend existing mockStorage).

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && go test ./internal/video/ -run TestPurgeOrphanedFiles_DeletesThumbnails -v`
Expected: FAIL

**Step 2: Update cleanup query and logic**

In `internal/video/cleanup.go`, update `PurgeOrphanedFiles`:

```go
rows, err := db.Query(ctx,
	`SELECT file_key, thumbnail_key FROM videos
	 WHERE status = 'deleted' AND file_purged_at IS NULL
	 LIMIT 50`)
```

Update scan and delete logic:

```go
for rows.Next() {
	var fileKey string
	var thumbnailKey *string
	if err := rows.Scan(&fileKey, &thumbnailKey); err != nil {
		log.Printf("cleanup: failed to scan file key: %v", err)
		continue
	}
	if err := deleteWithRetry(ctx, storage, fileKey, 3); err != nil {
		log.Printf("cleanup: failed to delete %s: %v", fileKey, err)
		continue
	}
	if thumbnailKey != nil {
		if err := deleteWithRetry(ctx, storage, *thumbnailKey, 3); err != nil {
			log.Printf("cleanup: failed to delete thumbnail %s: %v", *thumbnailKey, err)
		}
	}
	if _, err := db.Exec(ctx,
		`UPDATE videos SET file_purged_at = now() WHERE file_key = $1`,
		fileKey,
	); err != nil {
		log.Printf("cleanup: failed to mark purged for %s: %v", fileKey, err)
	}
}
```

**Step 3: Update Delete handler to also delete thumbnail**

In `internal/video/video.go`, Delete handler, update the RETURNING query:

```go
var fileKey string
var thumbnailKey *string
err := h.db.QueryRow(r.Context(),
	`UPDATE videos SET status = 'deleted', updated_at = now()
	 WHERE id = $1 AND user_id = $2 AND status != 'deleted'
	 RETURNING file_key, thumbnail_key`,
	videoID, userID,
).Scan(&fileKey, &thumbnailKey)
```

Update the goroutine to also delete thumbnail:

```go
go func() {
	ctx := context.Background()
	if err := deleteWithRetry(ctx, h.storage, fileKey, 3); err != nil {
		log.Printf("all delete retries failed for %s: %v", fileKey, err)
		return
	}
	if thumbnailKey != nil {
		if err := deleteWithRetry(ctx, h.storage, *thumbnailKey, 3); err != nil {
			log.Printf("thumbnail delete failed for %s: %v", *thumbnailKey, err)
		}
	}
	if _, err := h.db.Exec(ctx,
		`UPDATE videos SET file_purged_at = now() WHERE file_key = $1`,
		fileKey,
	); err != nil {
		log.Printf("failed to mark file_purged_at for %s: %v", fileKey, err)
	}
}()
```

**Step 4: Update Delete tests**

Tests that mock `UPDATE videos SET status = 'deleted'` with `RETURNING file_key` need to also return `thumbnail_key`:

In `TestDelete_Success` and `TestDelete_MarksFilePurgedOnSuccess`:
```go
mock.ExpectQuery(`UPDATE videos SET status = 'deleted'`).
	WithArgs(videoID, testUserID).
	WillReturnRows(pgxmock.NewRows([]string{"file_key", "thumbnail_key"}).AddRow(fileKey, (*string)(nil)))
```

In `TestDelete_VideoNotFound`:
```go
// No change needed — this returns an error, not rows
```

**Step 5: Update existing cleanup tests**

Update the cleanup tests in `cleanup_test.go` to use the new query with `thumbnail_key`.

**Step 6: Run all tests**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make test`
Expected: All tests pass

**Step 7: Commit**

```
feat: clean up thumbnail files on video deletion
```

---

### Task 10: Final verification and deploy

**Step 1: Run full test suite**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && make test`
Expected: All tests pass

**Step 2: Run frontend checks**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app/web && pnpm typecheck && pnpm build`
Expected: No errors

**Step 3: Verify Docker build**

Run: `cd /Users/aneamtu/Development/personal/sendrec/app && docker build -t sendrec-test .`
Expected: Build succeeds

**Step 4: Deploy to production**

Run deploy command (after user approval):
```bash
ssh -i ~/.ssh/alexneamtu_new root@77.42.44.160 "cd /opt/sendrec/app && git pull && cd /opt/sendrec && docker compose up -d --build sendrec-app"
```

**Step 5: Verify deployment**

- Migration 000007 applied (check logs)
- Upload a new video, confirm thumbnail appears in library after status = ready
- Watch page shows poster image before play
- Share URL shows og:image in link previews
- Delete a video, verify both video and thumbnail cleaned up
