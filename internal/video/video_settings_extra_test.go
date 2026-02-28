package video

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestGenerateDocument_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	handler.SetAIEnabled(true)

	mock.ExpectExec(`UPDATE videos SET document_status = 'pending'`).
		WithArgs("vid-1", testUserID, (*string)(nil)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/generate-document", handler.GenerateDocument)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/vid-1/generate-document", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestGenerateDocument_AIDisabled(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/generate-document", handler.GenerateDocument)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/vid-1/generate-document", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestGenerateDocument_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	handler.SetAIEnabled(true)

	mock.ExpectExec(`UPDATE videos SET document_status = 'pending'`).
		WithArgs("vid-missing", testUserID, (*string)(nil)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/generate-document", handler.GenerateDocument)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/vid-missing/generate-document", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
