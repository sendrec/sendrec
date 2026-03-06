package scim

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func newTestHandler(t *testing.T) (*Handler, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(mock, "http://localhost:8080"), mock
}

func withOrgID(r *http.Request, orgID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", orgID)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestServiceProviderConfig(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = withOrgID(req, "org-1")
	rec := httptest.NewRecorder()

	handler.ServiceProviderConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/scim+json" {
		t.Errorf("Content-Type = %q, want application/scim+json", ct)
	}

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	schemas := body["schemas"].([]interface{})
	if schemas[0] != SPConfigSchema {
		t.Errorf("schema = %q, want %q", schemas[0], SPConfigSchema)
	}
}

func TestSchemas(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = withOrgID(req, "org-1")
	rec := httptest.NewRecorder()

	handler.Schemas(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
}

func TestCreateUser_NewUser(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	// resolveUser step 1: external identity not found
	mock.ExpectQuery(`SELECT user_id FROM external_identities WHERE provider = \$1 AND external_id = \$2`).
		WithArgs("org-1", "ext-123").
		WillReturnError(pgx.ErrNoRows)

	// resolveUser step 2: user not found by email
	mock.ExpectQuery(`SELECT id, email_verified FROM users WHERE email = \$1`).
		WithArgs("jane@example.com").
		WillReturnError(pgx.ErrNoRows)

	// resolveUser step 3: create user
	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs("jane@example.com", "", "Jane Doe").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("user-uuid-1"))

	// Link external identity
	mock.ExpectExec(`INSERT INTO external_identities`).
		WithArgs("user-uuid-1", "org-1", "ext-123", "jane@example.com").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// Add to org
	mock.ExpectExec(`INSERT INTO organization_members`).
		WithArgs("org-1", "user-uuid-1", "member").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// fetchSCIMUser
	mock.ExpectQuery(`SELECT u.id, u.email, u.name FROM users u`).
		WithArgs("user-uuid-1", "org-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name"}).AddRow("user-uuid-1", "jane@example.com", "Jane Doe"))

	body := `{
		"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
		"userName": "jane@example.com",
		"name": {"formatted": "Jane Doe"},
		"externalId": "ext-123",
		"active": true
	}`

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/scim+json")
	req = withOrgID(req, "org-1")
	rec := httptest.NewRecorder()

	handler.CreateUser(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("got status %d, want 201; body: %s", rec.Code, rec.Body.String())
	}

	var user SCIMUser
	json.NewDecoder(rec.Body).Decode(&user)
	if user.ID != "user-uuid-1" {
		t.Errorf("ID = %q, want user-uuid-1", user.ID)
	}
	if user.UserName != "jane@example.com" {
		t.Errorf("UserName = %q, want jane@example.com", user.UserName)
	}
}

func TestCreateUser_ExistingUser(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	// resolveUser step 1: external identity found
	mock.ExpectQuery(`SELECT user_id FROM external_identities WHERE provider = \$1 AND external_id = \$2`).
		WithArgs("org-1", "ext-123").
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow("existing-user"))

	// Already a member — ON CONFLICT DO NOTHING
	mock.ExpectExec(`INSERT INTO organization_members`).
		WithArgs("org-1", "existing-user", "member").
		WillReturnResult(pgxmock.NewResult("INSERT", 0))

	// Fetch user details for response
	mock.ExpectQuery(`SELECT u.id, u.email, u.name FROM users u`).
		WithArgs("existing-user", "org-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name"}).AddRow("existing-user", "jane@example.com", "Jane Doe"))

	body := `{
		"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
		"userName": "jane@example.com",
		"externalId": "ext-123",
		"active": true
	}`

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/scim+json")
	req = withOrgID(req, "org-1")
	rec := httptest.NewRecorder()

	handler.CreateUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetUser(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT u.id, u.email, u.name FROM users u`).
		WithArgs("user-1", "org-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name"}).
			AddRow("user-1", "jane@example.com", "Jane Doe"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	rctx.URLParams.Add("id", "user-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.GetUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var user SCIMUser
	json.NewDecoder(rec.Body).Decode(&user)
	if user.UserName != "jane@example.com" {
		t.Errorf("UserName = %q, want jane@example.com", user.UserName)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT u.id, u.email, u.name FROM users u`).
		WithArgs("no-such-user", "org-1").
		WillReturnError(pgx.ErrNoRows)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	rctx.URLParams.Add("id", "no-such-user")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.GetUser(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want 404", rec.Code)
	}
}

func TestListUsers(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users u JOIN organization_members`).
		WithArgs("org-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT u.id, u.email, u.name FROM users u JOIN organization_members`).
		WithArgs("org-1", 100, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name"}).
			AddRow("user-1", "jane@example.com", "Jane").
			AddRow("user-2", "bob@example.com", "Bob"))

	req := httptest.NewRequest(http.MethodGet, "/?count=100&startIndex=1", nil)
	req = withOrgID(req, "org-1")
	rec := httptest.NewRecorder()

	handler.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp SCIMListResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.TotalResults != 2 {
		t.Errorf("TotalResults = %d, want 2", resp.TotalResults)
	}
	if len(resp.Resources) != 2 {
		t.Errorf("Resources length = %d, want 2", len(resp.Resources))
	}
}

func TestListUsers_FilterByUserName(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users u JOIN organization_members`).
		WithArgs("org-1", "jane@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT u.id, u.email, u.name FROM users u JOIN organization_members`).
		WithArgs("org-1", "jane@example.com", 100, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name"}).
			AddRow("user-1", "jane@example.com", "Jane"))

	filterURL := "/?filter=" + url.QueryEscape(`userName eq "jane@example.com"`)
	req := httptest.NewRequest(http.MethodGet, filterURL, nil)
	req = withOrgID(req, "org-1")
	rec := httptest.NewRecorder()

	handler.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}

	var resp SCIMListResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.TotalResults != 1 {
		t.Errorf("TotalResults = %d, want 1", resp.TotalResults)
	}
}
