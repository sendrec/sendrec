package organization

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/auth"
)

const testJWTSecret = "test-secret-for-org-tests"
const testUserID = "550e8400-e29b-41d4-a716-446655440000"
const testBaseURL = "https://sendrec.eu"

func authenticatedRequest(t *testing.T, method, target string, body []byte) *http.Request {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	token, err := auth.GenerateAccessToken(testJWTSecret, testUserID)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func newAuthMiddleware() func(http.Handler) http.Handler {
	return auth.NewHandler(nil, testJWTSecret, false).Middleware
}

func parseErrorResponse(t *testing.T, body []byte) string {
	t.Helper()
	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	return errResp.Error
}

func TestCreate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	now := time.Now().UTC().Truncate(time.Second)

	mock.ExpectQuery(`SELECT subscription_plan FROM users WHERE id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"subscription_plan"}).AddRow("pro"))

	mock.ExpectQuery(`INSERT INTO organizations`).
		WithArgs("Acme Corp", "acme-corp").
		WillReturnRows(pgxmock.NewRows([]string{"id", "slug", "created_at", "updated_at"}).
			AddRow("org-1", "acme-corp", now, now))

	mock.ExpectExec(`INSERT INTO organization_members`).
		WithArgs("org-1", testUserID).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(createOrgRequest{Name: "Acme Corp"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/organizations", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/organizations", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp orgResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != "org-1" {
		t.Errorf("expected ID %q, got %q", "org-1", resp.ID)
	}
	if resp.Name != "Acme Corp" {
		t.Errorf("expected name %q, got %q", "Acme Corp", resp.Name)
	}
	if resp.Slug != "acme-corp" {
		t.Errorf("expected slug %q, got %q", "acme-corp", resp.Slug)
	}
	if resp.SubscriptionPlan != "free" {
		t.Errorf("expected subscriptionPlan %q, got %q", "free", resp.SubscriptionPlan)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreate_FreeLimitOneOrg(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)

	mock.ExpectQuery(`SELECT subscription_plan FROM users WHERE id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"subscription_plan"}).AddRow("free"))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM organization_members WHERE user_id = \$1 AND role = 'owner'`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	body, _ := json.Marshal(createOrgRequest{Name: "Second Org"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/organizations", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/organizations", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "free plan allows 1 organization" {
		t.Errorf("expected error %q, got %q", "free plan allows 1 organization", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreate_InvalidName(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)

	body, _ := json.Marshal(createOrgRequest{Name: ""})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/organizations", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/organizations", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "organization name is required" {
		t.Errorf("expected error %q, got %q", "organization name is required", errMsg)
	}
}

func TestList(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)

	mock.ExpectQuery(`SELECT o\.id, o\.name, o\.slug, o\.subscription_plan, om\.role`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "slug", "subscription_plan", "role", "member_count"}).
			AddRow("org-1", "Acme Corp", "acme-corp", "free", "owner", int64(3)).
			AddRow("org-2", "Beta Inc", "beta-inc", "pro", "member", int64(5)))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/organizations", handler.List)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/organizations", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var items []orgListItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 organizations, got %d", len(items))
	}
	if items[0].Name != "Acme Corp" {
		t.Errorf("expected first org name %q, got %q", "Acme Corp", items[0].Name)
	}
	if items[0].Role != "owner" {
		t.Errorf("expected first org role %q, got %q", "owner", items[0].Role)
	}
	if items[0].MemberCount != 3 {
		t.Errorf("expected first org memberCount %d, got %d", 3, items[0].MemberCount)
	}
	if items[1].Name != "Beta Inc" {
		t.Errorf("expected second org name %q, got %q", "Beta Inc", items[1].Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestGet(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	now := time.Now().UTC().Truncate(time.Second)
	orgID := "org-1"

	mock.ExpectQuery(`SELECT o\.id, o\.name, o\.slug, o\.subscription_plan, o\.created_at, o\.updated_at, om\.role`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "slug", "subscription_plan", "created_at", "updated_at", "role", "member_count"}).
			AddRow(orgID, "Acme Corp", "acme-corp", "free", now, now, "owner", int64(3)))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/organizations/{orgId}", handler.Get)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/organizations/"+orgID, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp orgDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != orgID {
		t.Errorf("expected ID %q, got %q", orgID, resp.ID)
	}
	if resp.Name != "Acme Corp" {
		t.Errorf("expected name %q, got %q", "Acme Corp", resp.Name)
	}
	if resp.Role != "owner" {
		t.Errorf("expected role %q, got %q", "owner", resp.Role)
	}
	if resp.MemberCount != 3 {
		t.Errorf("expected memberCount %d, got %d", 3, resp.MemberCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestGet_NonMember(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	orgID := "org-1"

	mock.ExpectQuery(`SELECT o\.id, o\.name, o\.slug, o\.subscription_plan, o\.created_at, o\.updated_at, om\.role`).
		WithArgs(orgID, testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/organizations/{orgId}", handler.Get)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/organizations/"+orgID, nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "organization not found" {
		t.Errorf("expected error %q, got %q", "organization not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdate_AsOwner(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	orgID := "org-1"
	now := time.Now().UTC().Truncate(time.Second)
	newName := "Updated Corp"

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("owner"))

	mock.ExpectExec(`UPDATE organizations SET name = \$1, updated_at = now\(\) WHERE id = \$2`).
		WithArgs(newName, orgID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectQuery(`SELECT o\.id, o\.name, o\.slug, o\.subscription_plan, o\.created_at, o\.updated_at`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "slug", "subscription_plan", "created_at", "updated_at", "role", "member_count"}).
			AddRow(orgID, newName, "acme-corp", "free", now, now, "owner", int64(1)))

	body, _ := json.Marshal(map[string]any{"name": newName})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/organizations/{orgId}", handler.Update)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/organizations/"+orgID, body))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp orgDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Name != newName {
		t.Errorf("expected name %q, got %q", newName, resp.Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdate_AsMember(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	orgID := "org-1"

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("member"))

	body, _ := json.Marshal(map[string]any{"name": "New Name"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/organizations/{orgId}", handler.Update)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/organizations/"+orgID, body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "only owners and admins can update the organization" {
		t.Errorf("expected error %q, got %q", "only owners and admins can update the organization", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestDelete_AsOwner(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	orgID := "org-1"

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("owner"))

	mock.ExpectExec(`DELETE FROM organizations WHERE id = \$1`).
		WithArgs(orgID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/organizations/{orgId}", handler.Delete)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/organizations/"+orgID, nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestDelete_AsAdmin(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	orgID := "org-1"

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("admin"))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/organizations/{orgId}", handler.Delete)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/organizations/"+orgID, nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "only owners can delete the organization" {
		t.Errorf("expected error %q, got %q", "only owners can delete the organization", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Acme Corp", "acme-corp"},
		{"Hello World 123", "hello-world-123"},
		{"   spaces   ", "spaces"},
		{"---dashes---", "dashes"},
		{"UPPERCASE", "uppercase"},
		{"special!@#chars", "special-chars"},
		{"", "org"},
		{"!!!", "org"},
	}
	for _, tt := range tests {
		got := generateSlug(tt.name)
		if got != tt.want {
			t.Errorf("generateSlug(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
