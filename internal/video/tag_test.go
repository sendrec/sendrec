package video

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
)

func TestCreateTag_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	now := time.Now().UTC().Truncate(time.Second)
	color := "#ff0000"

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM tags WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`INSERT INTO tags`).
		WithArgs(testUserID, "Important", &color).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow("tag-1", now))

	body, _ := json.Marshal(createTagRequest{Name: "Important", Color: &color})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/tags", handler.CreateTag)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/tags", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp tagItem
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != "tag-1" {
		t.Errorf("expected ID %q, got %q", "tag-1", resp.ID)
	}
	if resp.Name != "Important" {
		t.Errorf("expected name %q, got %q", "Important", resp.Name)
	}
	if resp.Color == nil || *resp.Color != "#ff0000" {
		t.Errorf("expected color %q, got %v", "#ff0000", resp.Color)
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

func TestCreateTag_WithoutColor(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	now := time.Now().UTC().Truncate(time.Second)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM tags WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`INSERT INTO tags`).
		WithArgs(testUserID, "No Color Tag", (*string)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow("tag-2", now))

	body, _ := json.Marshal(createTagRequest{Name: "No Color Tag"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/tags", handler.CreateTag)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/tags", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp tagItem
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != "tag-2" {
		t.Errorf("expected ID %q, got %q", "tag-2", resp.ID)
	}
	if resp.Color != nil {
		t.Errorf("expected nil color, got %v", resp.Color)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreateTag_InvalidColor(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	badColor := "red"
	body, _ := json.Marshal(createTagRequest{Name: "Bad Color", Color: &badColor})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/tags", handler.CreateTag)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/tags", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "color must be a valid hex color (e.g. #ff0000)" {
		t.Errorf("expected error %q, got %q", "color must be a valid hex color (e.g. #ff0000)", errMsg)
	}
}

func TestCreateTag_NameTooLong(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	longName := strings.Repeat("a", 51)
	body, _ := json.Marshal(createTagRequest{Name: longName})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/tags", handler.CreateTag)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/tags", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "tag name must be 50 characters or less" {
		t.Errorf("expected error %q, got %q", "tag name must be 50 characters or less", errMsg)
	}
}

func TestCreateTag_EmptyName(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	body, _ := json.Marshal(createTagRequest{Name: ""})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/tags", handler.CreateTag)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/tags", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "tag name is required" {
		t.Errorf("expected error %q, got %q", "tag name is required", errMsg)
	}
}

func TestCreateTag_DuplicateName(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM tags WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`INSERT INTO tags`).
		WithArgs(testUserID, "Existing Tag", (*string)(nil)).
		WillReturnError(&pgconn.PgError{Code: "23505"})

	body, _ := json.Marshal(createTagRequest{Name: "Existing Tag"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/tags", handler.CreateTag)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/tags", body))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "a tag with this name already exists" {
		t.Errorf("expected error %q, got %q", "a tag with this name already exists", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreateTag_MaxTagsReached(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM tags WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(100))

	body, _ := json.Marshal(createTagRequest{Name: "One More Tag"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/tags", handler.CreateTag)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/tags", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "maximum of 100 tags reached" {
		t.Errorf("expected error %q, got %q", "maximum of 100 tags reached", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestListTags_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	now := time.Now().UTC().Truncate(time.Second)
	earlier := now.Add(-1 * time.Hour)
	color := "#00ff00"

	mock.ExpectQuery(`SELECT t\.id, t\.name, t\.color, t\.created_at`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "color", "created_at", "video_count"}).
			AddRow("tag-1", "Design", &color, earlier, int64(3)).
			AddRow("tag-2", "Marketing", (*string)(nil), now, int64(0)))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/tags", handler.ListTags)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/tags", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var items []tagItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(items))
	}
	if items[0].Name != "Design" {
		t.Errorf("expected first tag name %q, got %q", "Design", items[0].Name)
	}
	if items[0].Color == nil || *items[0].Color != "#00ff00" {
		t.Errorf("expected first tag color %q, got %v", "#00ff00", items[0].Color)
	}
	if items[0].VideoCount != 3 {
		t.Errorf("expected first tag videoCount %d, got %d", 3, items[0].VideoCount)
	}
	if items[1].Name != "Marketing" {
		t.Errorf("expected second tag name %q, got %q", "Marketing", items[1].Name)
	}
	if items[1].Color != nil {
		t.Errorf("expected second tag color nil, got %v", items[1].Color)
	}
	if items[1].VideoCount != 0 {
		t.Errorf("expected second tag videoCount %d, got %d", 0, items[1].VideoCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestListTags_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT t\.id, t\.name, t\.color, t\.created_at`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "color", "created_at", "video_count"}))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/tags", handler.ListTags)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/tags", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var items []tagItem
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

func TestUpdateTag_Rename(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	tagID := "tag-1"
	newName := "Renamed Tag"

	mock.ExpectExec(`UPDATE tags SET name = \$1 WHERE id = \$2 AND user_id = \$3`).
		WithArgs(newName, tagID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(map[string]any{"name": newName})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/tags/{id}", handler.UpdateTag)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/tags/"+tagID, body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdateTag_ChangeColor(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	tagID := "tag-1"
	newColor := "#0000ff"

	mock.ExpectExec(`UPDATE tags SET color = \$1 WHERE id = \$2 AND user_id = \$3`).
		WithArgs(newColor, tagID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(map[string]any{"color": newColor})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/tags/{id}", handler.UpdateTag)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/tags/"+tagID, body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestDeleteTag_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	tagID := "tag-1"

	mock.ExpectExec(`DELETE FROM tags WHERE id = \$1 AND user_id = \$2`).
		WithArgs(tagID, testUserID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/tags/{id}", handler.DeleteTag)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/tags/"+tagID, nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestDeleteTag_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	tagID := "nonexistent"

	mock.ExpectExec(`DELETE FROM tags WHERE id = \$1 AND user_id = \$2`).
		WithArgs(tagID, testUserID).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/tags/{id}", handler.DeleteTag)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/tags/"+tagID, nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "tag not found" {
		t.Errorf("expected error %q, got %q", "tag not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
