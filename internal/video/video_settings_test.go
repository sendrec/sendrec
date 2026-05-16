package video

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestExtend_AddsSevenDaysToCurrentExpiry(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, nil, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	currentExpiry := time.Now().Add(30 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT share_expires_at FROM videos WHERE`).
		WithArgs("vid-1", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"share_expires_at"}).AddRow(&currentExpiry))

	mock.ExpectExec(`UPDATE videos SET share_expires_at = share_expires_at \+ INTERVAL '7 days', updated_at = now\(\)`).
		WithArgs("vid-1", testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/extend", handler.Extend)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/vid-1/extend", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
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
