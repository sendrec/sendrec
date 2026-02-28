package organization

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/auth"
)

func TestMiddleware_NoHeader(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	var calledNext bool
	var capturedOrgID, capturedRole string

	handler := Middleware(mock)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		capturedOrgID = auth.OrgIDFromContext(r.Context())
		capturedRole = auth.OrgRoleFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	ctx := auth.ContextWithUserID(req.Context(), testUserID)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !calledNext {
		t.Fatal("expected next handler to be called")
	}
	if capturedOrgID != "" {
		t.Errorf("expected empty orgID, got %q", capturedOrgID)
	}
	if capturedRole != "" {
		t.Errorf("expected empty role, got %q", capturedRole)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestMiddleware_ValidMember(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	orgID := "org-1"

	mock.ExpectQuery(`SELECT om\.role FROM organization_members om`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("admin"))

	var calledNext bool
	var capturedOrgID, capturedRole string

	handler := Middleware(mock)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		capturedOrgID = auth.OrgIDFromContext(r.Context())
		capturedRole = auth.OrgRoleFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	ctx := auth.ContextWithUserID(req.Context(), testUserID)
	req = req.WithContext(ctx)
	req.Header.Set("X-Organization-Id", orgID)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !calledNext {
		t.Fatal("expected next handler to be called")
	}
	if capturedOrgID != orgID {
		t.Errorf("expected orgID %q, got %q", orgID, capturedOrgID)
	}
	if capturedRole != "admin" {
		t.Errorf("expected role %q, got %q", "admin", capturedRole)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestMiddleware_NonMember(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	orgID := "org-1"

	mock.ExpectQuery(`SELECT om\.role FROM organization_members om`).
		WithArgs(orgID, testUserID).
		WillReturnError(pgx.ErrNoRows)

	handler := Middleware(mock)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	ctx := auth.ContextWithUserID(req.Context(), testUserID)
	req = req.WithContext(ctx)
	req.Header.Set("X-Organization-Id", orgID)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "not a member of this organization" {
		t.Errorf("expected error %q, got %q", "not a member of this organization", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestMiddleware_NoAuth(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := Middleware(mock)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	req.Header.Set("X-Organization-Id", "org-1")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "authentication required" {
		t.Errorf("expected error %q, got %q", "authentication required", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRequireRole_Allowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	ctx := auth.ContextWithOrg(req.Context(), "org-1", "admin")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	role := RequireRole(rec, req, "owner", "admin")

	if role != "admin" {
		t.Errorf("expected role %q, got %q", "admin", role)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected no error written, got status %d", rec.Code)
	}
}

func TestRequireRole_Denied(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	ctx := auth.ContextWithOrg(req.Context(), "org-1", "member")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	role := RequireRole(rec, req, "owner", "admin")

	if role != "" {
		t.Errorf("expected empty role, got %q", role)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if errResp.Error != "insufficient permissions" {
		t.Errorf("expected error %q, got %q", "insufficient permissions", errResp.Error)
	}
}
