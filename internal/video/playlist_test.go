package video

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestCreatePlaylist_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	now := time.Now().UTC().Truncate(time.Second)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM playlists WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	expectPlanQuery(mock, "free")

	mock.ExpectQuery(`INSERT INTO playlists`).
		WithArgs(testUserID, "My Playlist", (*string)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "position", "created_at", "updated_at"}).
			AddRow("playlist-1", 1, now, now))

	body, _ := json.Marshal(map[string]string{"title": "My Playlist"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/playlists", handler.CreatePlaylist)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/playlists", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp playlistItem
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != "playlist-1" {
		t.Errorf("expected ID %q, got %q", "playlist-1", resp.ID)
	}
	if resp.Title != "My Playlist" {
		t.Errorf("expected title %q, got %q", "My Playlist", resp.Title)
	}
	if resp.Position != 1 {
		t.Errorf("expected position %d, got %d", 1, resp.Position)
	}
	if resp.VideoCount != 0 {
		t.Errorf("expected videoCount 0, got %d", resp.VideoCount)
	}
	if resp.IsShared != false {
		t.Errorf("expected isShared false, got true")
	}
	if resp.CreatedAt != now.Format(time.RFC3339) {
		t.Errorf("expected createdAt %q, got %q", now.Format(time.RFC3339), resp.CreatedAt)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreatePlaylist_EmptyTitle(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	body, _ := json.Marshal(map[string]string{"title": ""})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/playlists", handler.CreatePlaylist)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/playlists", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "playlist title is required" {
		t.Errorf("expected error %q, got %q", "playlist title is required", errMsg)
	}
}

func TestCreatePlaylist_TitleTooLong(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	longTitle := strings.Repeat("a", 201)
	body, _ := json.Marshal(map[string]string{"title": longTitle})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/playlists", handler.CreatePlaylist)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/playlists", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "playlist title must be 200 characters or less" {
		t.Errorf("expected error %q, got %q", "playlist title must be 200 characters or less", errMsg)
	}
}

func TestCreatePlaylist_FreeTierLimitReached(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM playlists WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(3))

	expectPlanQuery(mock, "free")

	body, _ := json.Marshal(map[string]string{"title": "Another Playlist"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/playlists", handler.CreatePlaylist)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/playlists", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "free plan limited to 3 playlists, upgrade to create more" {
		t.Errorf("expected error %q, got %q", "free plan limited to 3 playlists, upgrade to create more", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreatePlaylist_ProUnlimited(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	now := time.Now().UTC().Truncate(time.Second)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM playlists WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(100))

	expectPlanQuery(mock, "pro")

	mock.ExpectQuery(`INSERT INTO playlists`).
		WithArgs(testUserID, "Pro Playlist", (*string)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "position", "created_at", "updated_at"}).
			AddRow("playlist-101", 100, now, now))

	body, _ := json.Marshal(map[string]string{"title": "Pro Playlist"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/playlists", handler.CreatePlaylist)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/playlists", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestListPlaylists_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	now := time.Now().UTC().Truncate(time.Second)
	earlier := now.Add(-1 * time.Hour)

	shareToken := "abc123token1"

	mock.ExpectQuery(`SELECT p\.id, p\.title, p\.description, p\.is_shared, p\.share_token, p\.position, p\.created_at, p\.updated_at`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "title", "description", "is_shared", "share_token", "position", "created_at", "updated_at", "video_count"}).
			AddRow("playlist-1", "First Playlist", (*string)(nil), true, &shareToken, 0, earlier, earlier, int64(3)).
			AddRow("playlist-2", "Second Playlist", (*string)(nil), false, (*string)(nil), 1, now, now, int64(0)))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/playlists", handler.ListPlaylists)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/playlists", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var items []playlistItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 playlists, got %d", len(items))
	}
	if items[0].Title != "First Playlist" {
		t.Errorf("expected first playlist title %q, got %q", "First Playlist", items[0].Title)
	}
	if items[0].VideoCount != 3 {
		t.Errorf("expected first playlist videoCount %d, got %d", 3, items[0].VideoCount)
	}
	if !items[0].IsShared {
		t.Errorf("expected first playlist isShared true")
	}
	expectedShareURL := testBaseURL + "/watch/playlist/" + shareToken
	if items[0].ShareURL == nil || *items[0].ShareURL != expectedShareURL {
		t.Errorf("expected first playlist shareUrl %q, got %v", expectedShareURL, items[0].ShareURL)
	}
	if items[1].Title != "Second Playlist" {
		t.Errorf("expected second playlist title %q, got %q", "Second Playlist", items[1].Title)
	}
	if items[1].VideoCount != 0 {
		t.Errorf("expected second playlist videoCount %d, got %d", 0, items[1].VideoCount)
	}
	if items[1].ShareURL != nil {
		t.Errorf("expected second playlist shareUrl nil, got %v", items[1].ShareURL)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestListPlaylists_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT p\.id, p\.title, p\.description, p\.is_shared, p\.share_token, p\.position, p\.created_at, p\.updated_at`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "title", "description", "is_shared", "share_token", "position", "created_at", "updated_at", "video_count"}))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/playlists", handler.ListPlaylists)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/playlists", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var items []playlistItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty array, got %d items", len(items))
	}

	trimmed := strings.TrimSpace(rec.Body.String())
	if trimmed != "[]" {
		t.Errorf("expected JSON empty array [], got %q", trimmed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestGetPlaylist_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://storage.sendrec.eu/thumb.jpg"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	now := time.Now().UTC().Truncate(time.Second)
	shareToken := "playlisttoken"

	mock.ExpectQuery(`SELECT p\.id, p\.title, p\.description, p\.is_shared, p\.share_token, p\.require_email, p\.position, p\.created_at, p\.updated_at`).
		WithArgs("playlist-1", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "title", "description", "is_shared", "share_token", "require_email", "position", "created_at", "updated_at"}).
			AddRow("playlist-1", "My Playlist", (*string)(nil), true, &shareToken, false, 0, now, now))

	thumbKey := "recordings/user1/thumb.jpg"
	mock.ExpectQuery(`SELECT v\.id, v\.title, v\.duration, v\.share_token, v\.status, v\.created_at`).
		WithArgs("playlist-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "title", "duration", "share_token", "status", "created_at", "thumbnail_key", "position"}).
			AddRow("video-1", "First Video", 120, "vtoken1", "ready", now, &thumbKey, 0).
			AddRow("video-2", "Second Video", 60, "vtoken2", "ready", now, (*string)(nil), 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/playlists/{id}", handler.GetPlaylist)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/playlists/playlist-1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp playlistDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != "playlist-1" {
		t.Errorf("expected ID %q, got %q", "playlist-1", resp.ID)
	}
	if resp.Title != "My Playlist" {
		t.Errorf("expected title %q, got %q", "My Playlist", resp.Title)
	}
	if !resp.IsShared {
		t.Errorf("expected isShared true")
	}
	if len(resp.Videos) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(resp.Videos))
	}
	if resp.Videos[0].Title != "First Video" {
		t.Errorf("expected first video title %q, got %q", "First Video", resp.Videos[0].Title)
	}
	if resp.Videos[0].Duration != 120 {
		t.Errorf("expected first video duration %d, got %d", 120, resp.Videos[0].Duration)
	}
	if resp.Videos[0].ThumbnailURL == nil {
		t.Errorf("expected first video thumbnailUrl to be set")
	}
	expectedShareURL := testBaseURL + "/watch/vtoken1"
	if resp.Videos[0].ShareURL != expectedShareURL {
		t.Errorf("expected first video shareUrl %q, got %q", expectedShareURL, resp.Videos[0].ShareURL)
	}
	if resp.Videos[1].ThumbnailURL != nil {
		t.Errorf("expected second video thumbnailUrl to be nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestGetPlaylist_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT p\.id, p\.title, p\.description, p\.is_shared, p\.share_token, p\.require_email, p\.position, p\.created_at, p\.updated_at`).
		WithArgs("nonexistent", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "title", "description", "is_shared", "share_token", "require_email", "position", "created_at", "updated_at"}))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/playlists/{id}", handler.GetPlaylist)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/playlists/nonexistent", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "playlist not found" {
		t.Errorf("expected error %q, got %q", "playlist not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdatePlaylist_Rename(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	playlistID := "playlist-1"
	newTitle := "Renamed Playlist"

	mock.ExpectExec(`UPDATE playlists SET title = \$1 WHERE id = \$2 AND user_id = \$3`).
		WithArgs(newTitle, playlistID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(map[string]any{"title": newTitle})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Patch("/api/playlists/{id}", handler.UpdatePlaylist)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPatch, "/api/playlists/"+playlistID, body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdatePlaylist_EnableSharing(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	playlistID := "playlist-1"

	mock.ExpectExec(`UPDATE playlists SET is_shared = \$1, share_token = \$2 WHERE id = \$3 AND user_id = \$4`).
		WithArgs(true, pgxmock.AnyArg(), playlistID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(map[string]any{"isShared": true})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Patch("/api/playlists/{id}", handler.UpdatePlaylist)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPatch, "/api/playlists/"+playlistID, body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdatePlaylist_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	playlistID := "nonexistent"
	newTitle := "Updated Title"

	mock.ExpectExec(`UPDATE playlists SET title = \$1 WHERE id = \$2 AND user_id = \$3`).
		WithArgs(newTitle, playlistID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	body, _ := json.Marshal(map[string]any{"title": newTitle})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Patch("/api/playlists/{id}", handler.UpdatePlaylist)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPatch, "/api/playlists/"+playlistID, body))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "playlist not found" {
		t.Errorf("expected error %q, got %q", "playlist not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestDeletePlaylist_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	playlistID := "playlist-1"

	mock.ExpectExec(`DELETE FROM playlists WHERE id = \$1 AND user_id = \$2`).
		WithArgs(playlistID, testUserID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/playlists/{id}", handler.DeletePlaylist)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/playlists/"+playlistID, nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestDeletePlaylist_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	playlistID := "nonexistent"

	mock.ExpectExec(`DELETE FROM playlists WHERE id = \$1 AND user_id = \$2`).
		WithArgs(playlistID, testUserID).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/playlists/{id}", handler.DeletePlaylist)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/playlists/"+playlistID, nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "playlist not found" {
		t.Errorf("expected error %q, got %q", "playlist not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestAddPlaylistVideos_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	playlistID := "playlist-1"

	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(playlistID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs("video-1", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

	mock.ExpectExec(`INSERT INTO playlist_videos`).
		WithArgs(playlistID, "video-1").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs("video-2", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

	mock.ExpectExec(`INSERT INTO playlist_videos`).
		WithArgs(playlistID, "video-2").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectExec(`UPDATE playlists SET updated_at`).
		WithArgs(playlistID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(addPlaylistVideosRequest{VideoIDs: []string{"video-1", "video-2"}})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/playlists/{id}/videos", handler.AddPlaylistVideos)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/playlists/"+playlistID+"/videos", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestAddPlaylistVideos_EmptyVideoIDs(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	body, _ := json.Marshal(addPlaylistVideosRequest{VideoIDs: []string{}})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/playlists/{id}/videos", handler.AddPlaylistVideos)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/playlists/playlist-1/videos", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "at least 1 video ID is required" {
		t.Errorf("expected error %q, got %q", "at least 1 video ID is required", errMsg)
	}
}

func TestAddPlaylistVideos_PlaylistNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	playlistID := "nonexistent"

	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(playlistID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

	body, _ := json.Marshal(addPlaylistVideosRequest{VideoIDs: []string{"video-1"}})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/playlists/{id}/videos", handler.AddPlaylistVideos)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/playlists/"+playlistID+"/videos", body))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "playlist not found" {
		t.Errorf("expected error %q, got %q", "playlist not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRemovePlaylistVideo_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	playlistID := "playlist-1"
	videoID := "video-1"

	mock.ExpectExec(`DELETE FROM playlist_videos`).
		WithArgs(playlistID, videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	mock.ExpectExec(`UPDATE playlists SET updated_at`).
		WithArgs(playlistID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/playlists/{id}/videos/{videoId}", handler.RemovePlaylistVideo)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/playlists/"+playlistID+"/videos/"+videoID, nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRemovePlaylistVideo_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	playlistID := "playlist-1"
	videoID := "nonexistent"

	mock.ExpectExec(`DELETE FROM playlist_videos`).
		WithArgs(playlistID, videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/playlists/{id}/videos/{videoId}", handler.RemovePlaylistVideo)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/playlists/"+playlistID+"/videos/"+videoID, nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "video not in playlist" {
		t.Errorf("expected error %q, got %q", "video not in playlist", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestReorderPlaylistVideos_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	playlistID := "playlist-1"

	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(playlistID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

	mock.ExpectExec(`UPDATE playlist_videos SET position`).
		WithArgs(1, playlistID, "video-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec(`UPDATE playlist_videos SET position`).
		WithArgs(0, playlistID, "video-2").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec(`UPDATE playlists SET updated_at`).
		WithArgs(playlistID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(reorderPlaylistVideosRequest{
		Items: []reorderItem{
			{VideoID: "video-1", Position: 1},
			{VideoID: "video-2", Position: 0},
		},
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Patch("/api/playlists/{id}/videos/reorder", handler.ReorderPlaylistVideos)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPatch, "/api/playlists/"+playlistID+"/videos/reorder", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestReorderPlaylistVideos_PlaylistNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	playlistID := "nonexistent"

	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(playlistID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

	body, _ := json.Marshal(reorderPlaylistVideosRequest{
		Items: []reorderItem{
			{VideoID: "video-1", Position: 0},
		},
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Patch("/api/playlists/{id}/videos/reorder", handler.ReorderPlaylistVideos)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPatch, "/api/playlists/"+playlistID+"/videos/reorder", body))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "playlist not found" {
		t.Errorf("expected error %q, got %q", "playlist not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
