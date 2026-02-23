package video

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestBatchDelete_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoIDs := []string{"video-1", "video-2"}

	mock.ExpectQuery(`UPDATE videos SET status = 'deleted'`).
		WithArgs(videoIDs, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "file_key", "thumbnail_key", "webcam_key", "transcript_key", "title"}).
			AddRow("video-1", "recordings/user1/token1.webm", nil, nil, nil, "Recording 1").
			AddRow("video-2", "recordings/user1/token2.webm", nil, nil, nil, "Recording 2"))

	body, _ := json.Marshal(batchRequest{VideoIDs: videoIDs})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/batch/delete", handler.BatchDelete)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/batch/delete", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp batchDeleteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Deleted != 2 {
		t.Errorf("expected deleted 2, got %d", resp.Deleted)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestBatchDelete_PartialOwnership(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoIDs := []string{"video-1", "video-2"}

	mock.ExpectQuery(`UPDATE videos SET status = 'deleted'`).
		WithArgs(videoIDs, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "file_key", "thumbnail_key", "webcam_key", "transcript_key", "title"}).
			AddRow("video-1", "recordings/user1/token1.webm", nil, nil, nil, "Recording 1"))

	body, _ := json.Marshal(batchRequest{VideoIDs: videoIDs})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/batch/delete", handler.BatchDelete)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/batch/delete", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp batchDeleteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Deleted != 1 {
		t.Errorf("expected deleted 1, got %d", resp.Deleted)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestBatchDelete_EmptyList(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	body, _ := json.Marshal(batchRequest{VideoIDs: []string{}})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/batch/delete", handler.BatchDelete)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/batch/delete", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "videoIds must contain 1-100 items" {
		t.Errorf("expected error %q, got %q", "videoIds must contain 1-100 items", errMsg)
	}
}

func TestBatchDelete_TooManyItems(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	ids := make([]string, 101)
	for i := range ids {
		ids[i] = "video-id"
	}
	body, _ := json.Marshal(batchRequest{VideoIDs: ids})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/batch/delete", handler.BatchDelete)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/batch/delete", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "videoIds must contain 1-100 items" {
		t.Errorf("expected error %q, got %q", "videoIds must contain 1-100 items", errMsg)
	}
}

func TestBatchDelete_InvalidBody(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/batch/delete", handler.BatchDelete)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/batch/delete", []byte("{invalid json")))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "invalid request body" {
		t.Errorf("expected error %q, got %q", "invalid request body", errMsg)
	}
}

func TestBatchSetFolder_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoIDs := []string{"video-1", "video-2"}
	folderID := "folder-1"

	mock.ExpectQuery(`SELECT id FROM folders WHERE id = \$1 AND user_id = \$2`).
		WithArgs(folderID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(folderID))

	mock.ExpectExec(`UPDATE videos SET folder_id = \$1`).
		WithArgs(&folderID, videoIDs, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 2))

	body, _ := json.Marshal(batchFolderRequest{VideoIDs: videoIDs, FolderID: &folderID})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/batch/folder", handler.BatchSetFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/batch/folder", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestBatchSetFolder_NullClearsFolder(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoIDs := []string{"video-1", "video-2"}

	mock.ExpectExec(`UPDATE videos SET folder_id = \$1`).
		WithArgs(pgxmock.AnyArg(), videoIDs, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 2))

	body, _ := json.Marshal(batchFolderRequest{VideoIDs: videoIDs, FolderID: nil})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/batch/folder", handler.BatchSetFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/batch/folder", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestBatchSetFolder_InvalidFolder(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoIDs := []string{"video-1"}
	folderID := "nonexistent-folder"

	mock.ExpectQuery(`SELECT id FROM folders WHERE id = \$1 AND user_id = \$2`).
		WithArgs(folderID, testUserID).
		WillReturnError(pgx.ErrNoRows)

	body, _ := json.Marshal(batchFolderRequest{VideoIDs: videoIDs, FolderID: &folderID})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/batch/folder", handler.BatchSetFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/batch/folder", body))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "folder not found" {
		t.Errorf("expected error %q, got %q", "folder not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestBatchSetTags_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoIDs := []string{"video-1", "video-2"}
	tagIDs := []string{"tag-1", "tag-2"}

	// Verify all tags belong to user
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM tags WHERE id = ANY\(\$1\) AND user_id = \$2`).
		WithArgs(tagIDs, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	// Verify all videos belong to user
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM videos WHERE id = ANY\(\$1\) AND user_id = \$2 AND status != 'deleted'`).
		WithArgs(videoIDs, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	// For each video: delete existing tags, insert new ones
	for _, vid := range videoIDs {
		mock.ExpectExec(`DELETE FROM video_tags WHERE video_id = \$1`).
			WithArgs(vid).
			WillReturnResult(pgxmock.NewResult("DELETE", 0))
		for _, tid := range tagIDs {
			mock.ExpectExec(`INSERT INTO video_tags \(video_id, tag_id\) VALUES \(\$1, \$2\)`).
				WithArgs(vid, tid).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
		}
	}

	body, _ := json.Marshal(batchTagsRequest{VideoIDs: videoIDs, TagIDs: tagIDs})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/batch/tags", handler.BatchSetTags)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/batch/tags", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestBatchSetTags_InvalidTag(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoIDs := []string{"video-1"}
	tagIDs := []string{"tag-1", "nonexistent"}

	// Count returns 1 (only tag-1 found), but we passed 2
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM tags WHERE id = ANY\(\$1\) AND user_id = \$2`).
		WithArgs(tagIDs, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	body, _ := json.Marshal(batchTagsRequest{VideoIDs: videoIDs, TagIDs: tagIDs})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/batch/tags", handler.BatchSetTags)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/batch/tags", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "one or more tags not found" {
		t.Errorf("expected error %q, got %q", "one or more tags not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestBatchSetTags_EmptyTagsClearsAll(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoIDs := []string{"video-1", "video-2"}

	// Verify all videos belong to user
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM videos WHERE id = ANY\(\$1\) AND user_id = \$2 AND status != 'deleted'`).
		WithArgs(videoIDs, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	// For each video: delete existing tags only (no inserts)
	for _, vid := range videoIDs {
		mock.ExpectExec(`DELETE FROM video_tags WHERE video_id = \$1`).
			WithArgs(vid).
			WillReturnResult(pgxmock.NewResult("DELETE", 2))
	}

	body, _ := json.Marshal(batchTagsRequest{VideoIDs: videoIDs, TagIDs: []string{}})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/batch/tags", handler.BatchSetTags)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/batch/tags", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
