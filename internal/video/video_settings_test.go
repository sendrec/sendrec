package video

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	pgx "github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestTogglePin_PinsVideo(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, nil, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`UPDATE videos SET pinned = NOT pinned, updated_at = now\(\)`).
		WithArgs("vid-1", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"pinned"}).AddRow(true))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/pin", handler.TogglePin)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/vid-1/pin", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp["pinned"] {
		t.Errorf("expected pinned=true, got %v", resp["pinned"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestTogglePin_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, nil, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`UPDATE videos SET pinned = NOT pinned, updated_at = now\(\)`).
		WithArgs("vid-missing", testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/pin", handler.TogglePin)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/vid-missing/pin", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
