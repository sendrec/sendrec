package video

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
)

func TestCreateFolder_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	now := time.Now().UTC().Truncate(time.Second)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM folders WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`INSERT INTO folders`).
		WithArgs(testUserID, "My Folder").
		WillReturnRows(pgxmock.NewRows([]string{"id", "position", "created_at"}).
			AddRow("folder-1", 2, now))

	body, _ := json.Marshal(createFolderRequest{Name: "My Folder"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/folders", handler.CreateFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/folders", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp folderItem
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != "folder-1" {
		t.Errorf("expected ID %q, got %q", "folder-1", resp.ID)
	}
	if resp.Name != "My Folder" {
		t.Errorf("expected name %q, got %q", "My Folder", resp.Name)
	}
	if resp.Position != 2 {
		t.Errorf("expected position %d, got %d", 2, resp.Position)
	}
	if resp.VideoCount != 0 {
		t.Errorf("expected videoCount 0, got %d", resp.VideoCount)
	}
	if resp.CreatedAt != now.Format(time.RFC3339) {
		t.Errorf("expected createdAt %q, got %q", now.Format(time.RFC3339), resp.CreatedAt)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreateFolder_NameTooLong(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	longName := strings.Repeat("a", 101)
	body, _ := json.Marshal(createFolderRequest{Name: longName})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/folders", handler.CreateFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/folders", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "folder name must be 100 characters or less" {
		t.Errorf("expected error %q, got %q", "folder name must be 100 characters or less", errMsg)
	}
}

func TestCreateFolder_EmptyName(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	body, _ := json.Marshal(createFolderRequest{Name: ""})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/folders", handler.CreateFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/folders", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "folder name is required" {
		t.Errorf("expected error %q, got %q", "folder name is required", errMsg)
	}
}

func TestCreateFolder_WhitespaceOnlyName(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	body, _ := json.Marshal(createFolderRequest{Name: "   "})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/folders", handler.CreateFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/folders", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "folder name is required" {
		t.Errorf("expected error %q, got %q", "folder name is required", errMsg)
	}
}

func TestCreateFolder_DuplicateName(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM folders WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`INSERT INTO folders`).
		WithArgs(testUserID, "Existing Folder").
		WillReturnError(&pgconn.PgError{Code: "23505"})

	body, _ := json.Marshal(createFolderRequest{Name: "Existing Folder"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/folders", handler.CreateFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/folders", body))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "a folder with this name already exists" {
		t.Errorf("expected error %q, got %q", "a folder with this name already exists", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreateFolder_MaxFoldersReached(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM folders WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(50))

	body, _ := json.Marshal(createFolderRequest{Name: "One More Folder"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/folders", handler.CreateFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/folders", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "maximum of 50 folders reached" {
		t.Errorf("expected error %q, got %q", "maximum of 50 folders reached", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestListFolders_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	now := time.Now().UTC().Truncate(time.Second)
	earlier := now.Add(-1 * time.Hour)

	mock.ExpectQuery(`SELECT f\.id, f\.name, f\.position, f\.created_at`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "position", "created_at", "video_count"}).
			AddRow("folder-1", "Marketing", 0, earlier, int64(3)).
			AddRow("folder-2", "Engineering", 1, now, int64(0)))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/folders", handler.ListFolders)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/folders", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var items []folderItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(items))
	}
	if items[0].Name != "Marketing" {
		t.Errorf("expected first folder name %q, got %q", "Marketing", items[0].Name)
	}
	if items[0].VideoCount != 3 {
		t.Errorf("expected first folder videoCount %d, got %d", 3, items[0].VideoCount)
	}
	if items[1].Name != "Engineering" {
		t.Errorf("expected second folder name %q, got %q", "Engineering", items[1].Name)
	}
	if items[1].VideoCount != 0 {
		t.Errorf("expected second folder videoCount %d, got %d", 0, items[1].VideoCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestListFolders_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT f\.id, f\.name, f\.position, f\.created_at`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "position", "created_at", "video_count"}))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/folders", handler.ListFolders)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/folders", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var items []folderItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty array, got %d items", len(items))
	}

	// Verify the JSON is [] not null
	trimmed := strings.TrimSpace(rec.Body.String())
	if trimmed != "[]" {
		t.Errorf("expected JSON empty array [], got %q", trimmed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdateFolder_Rename(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	folderID := "folder-1"
	newName := "Renamed Folder"

	mock.ExpectExec(`UPDATE folders SET name = \$1 WHERE id = \$2 AND user_id = \$3`).
		WithArgs(newName, folderID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(map[string]any{"name": newName})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/folders/{id}", handler.UpdateFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/folders/"+folderID, body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdateFolder_Reorder(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	folderID := "folder-1"
	newPosition := 3

	mock.ExpectExec(`UPDATE folders SET position = \$1 WHERE id = \$2 AND user_id = \$3`).
		WithArgs(newPosition, folderID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(map[string]any{"position": newPosition})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/folders/{id}", handler.UpdateFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/folders/"+folderID, body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdateFolder_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	folderID := "nonexistent"
	newName := "Updated Name"

	mock.ExpectExec(`UPDATE folders SET name = \$1 WHERE id = \$2 AND user_id = \$3`).
		WithArgs(newName, folderID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	body, _ := json.Marshal(map[string]any{"name": newName})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/folders/{id}", handler.UpdateFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/folders/"+folderID, body))

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

func TestDeleteFolder_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	folderID := "folder-1"

	mock.ExpectExec(`DELETE FROM folders WHERE id = \$1 AND user_id = \$2`).
		WithArgs(folderID, testUserID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/folders/{id}", handler.DeleteFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/folders/"+folderID, nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestDeleteFolder_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	folderID := "nonexistent"

	mock.ExpectExec(`DELETE FROM folders WHERE id = \$1 AND user_id = \$2`).
		WithArgs(folderID, testUserID).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/folders/{id}", handler.DeleteFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/folders/"+folderID, nil))

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

func TestSetVideoFolder_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	videoID := "video-1"
	folderID := "folder-1"

	mock.ExpectQuery(`SELECT id FROM folders WHERE id = \$1 AND user_id = \$2`).
		WithArgs(folderID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(folderID))

	mock.ExpectExec(`UPDATE videos SET folder_id = \$1 WHERE id = \$2 AND user_id = \$3 AND status != 'deleted'`).
		WithArgs(&folderID, videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(setVideoFolderRequest{FolderID: &folderID})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/folder", handler.SetVideoFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/"+videoID+"/folder", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestSetVideoFolder_Unfile(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	videoID := "video-1"

	mock.ExpectExec(`UPDATE videos SET folder_id = \$1 WHERE id = \$2 AND user_id = \$3 AND status != 'deleted'`).
		WithArgs((*string)(nil), videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body := []byte(`{"folderId":null}`)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/folder", handler.SetVideoFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/"+videoID+"/folder", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestSetVideoFolder_FolderNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	videoID := "video-1"
	folderID := "nonexistent"

	mock.ExpectQuery(`SELECT id FROM folders WHERE id = \$1 AND user_id = \$2`).
		WithArgs(folderID, testUserID).
		WillReturnError(pgx.ErrNoRows)

	body, _ := json.Marshal(setVideoFolderRequest{FolderID: &folderID})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/folder", handler.SetVideoFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/"+videoID+"/folder", body))

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

func TestSetVideoFolder_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	videoID := "nonexistent"
	folderID := "folder-1"

	mock.ExpectQuery(`SELECT id FROM folders WHERE id = \$1 AND user_id = \$2`).
		WithArgs(folderID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(folderID))

	mock.ExpectExec(`UPDATE videos SET folder_id = \$1 WHERE id = \$2 AND user_id = \$3 AND status != 'deleted'`).
		WithArgs(&folderID, videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	body, _ := json.Marshal(setVideoFolderRequest{FolderID: &folderID})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/folder", handler.SetVideoFolder)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/"+videoID+"/folder", body))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "video not found" {
		t.Errorf("expected error %q, got %q", "video not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
