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
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
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

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["schemas"] == nil {
		t.Fatal("schemas missing from response")
	}
	resources, ok := body["Resources"].([]interface{})
	if !ok || len(resources) != 1 {
		t.Fatalf("Resources = %#v, want one schema resource", body["Resources"])
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
	mock.ExpectQuery(`SELECT`).
		WithArgs("user-uuid-1", "org-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name", "external_id", "active"}).
			AddRow("user-uuid-1", "jane@example.com", "Jane Doe", "ext-123", true))

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
	if err := json.NewDecoder(rec.Body).Decode(&user); err != nil {
		t.Fatal(err)
	}
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
	mock.ExpectQuery(`SELECT`).
		WithArgs("existing-user", "org-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name", "external_id", "active"}).
			AddRow("existing-user", "jane@example.com", "Jane Doe", "ext-123", true))

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

	mock.ExpectQuery(`SELECT`).
		WithArgs("user-1", "org-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name", "external_id", "active"}).
			AddRow("user-1", "jane@example.com", "Jane Doe", "ext-123", true))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	rctx.URLParams.Add("id", "user-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.GetUser(rec, req)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var user SCIMUser
	if err := json.NewDecoder(rec.Body).Decode(&user); err != nil {
		t.Fatal(err)
	}
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

func TestGetUser_Inactive(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT`).
		WithArgs("user-1", "org-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name", "external_id", "active"}).
			AddRow("user-1", "jane@example.com", "Jane Doe", "ext-123", false))

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
	if err := json.NewDecoder(rec.Body).Decode(&user); err != nil {
		t.Fatal(err)
	}
	if user.Active {
		t.Error("expected inactive user to serialize with active=false")
	}
	if user.ExternalID != "ext-123" {
		t.Errorf("ExternalID = %q, want ext-123", user.ExternalID)
	}
}

func TestListUsers(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT`).
		WithArgs("org-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT`).
		WithArgs("org-1", 100, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name", "external_id", "active"}).
			AddRow("user-1", "jane@example.com", "Jane", "ext-123", true).
			AddRow("user-2", "bob@example.com", "Bob", "ext-456", true))

	req := httptest.NewRequest(http.MethodGet, "/?count=100&startIndex=1", nil)
	req = withOrgID(req, "org-1")
	rec := httptest.NewRecorder()

	handler.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp SCIMListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
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

	mock.ExpectQuery(`SELECT`).
		WithArgs("org-1", "jane@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT`).
		WithArgs("org-1", "jane@example.com", 100, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name", "external_id", "active"}).
			AddRow("user-1", "jane@example.com", "Jane", "ext-123", true))

	filterURL := "/?filter=" + url.QueryEscape(`userName eq "jane@example.com"`)
	req := httptest.NewRequest(http.MethodGet, filterURL, nil)
	req = withOrgID(req, "org-1")
	rec := httptest.NewRecorder()

	handler.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}

	var resp SCIMListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.TotalResults != 1 {
		t.Errorf("TotalResults = %d, want 1", resp.TotalResults)
	}
}

func TestListUsers_IncludesInactiveProvisionedUsers(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT`).
		WithArgs("org-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT`).
		WithArgs("org-1", 100, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name", "external_id", "active"}).
			AddRow("user-1", "jane@example.com", "Jane", "ext-123", false))

	req := httptest.NewRequest(http.MethodGet, "/?count=100&startIndex=1", nil)
	req = withOrgID(req, "org-1")
	rec := httptest.NewRecorder()

	handler.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp SCIMListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Resources) != 1 {
		t.Fatalf("Resources length = %d, want 1", len(resp.Resources))
	}
	if resp.Resources[0].Active {
		t.Error("expected inactive SCIM user to remain listed with active=false")
	}
}

func TestPatchUser_Deactivate(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectExec(`DELETE FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs("org-1", "user-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "active", "value": false}]
	}`

	req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	rctx.URLParams.Add("id", "user-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.PatchUser(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("got status %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
}

func TestPatchUser_DeactivateWithValueObject(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectExec(`DELETE FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs("org-1", "user-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "value": {"active": false}}]
	}`

	req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	rctx.URLParams.Add("id", "user-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.PatchUser(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("got status %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
}

func TestPatchUser_Reactivate(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO organization_members`).
		WithArgs("org-1", "user-1", "member").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectQuery(`SELECT`).
		WithArgs("user-1", "org-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name", "external_id", "active"}).
			AddRow("user-1", "jane@example.com", "Jane", "ext-123", true))

	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "active", "value": true}]
	}`

	req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	rctx.URLParams.Add("id", "user-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.PatchUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestPatchUser_UpdateName(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectExec(`UPDATE users SET name = \$1`).
		WithArgs("Jane Updated", "user-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectQuery(`SELECT`).
		WithArgs("user-1", "org-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name", "external_id", "active"}).
			AddRow("user-1", "jane@example.com", "Jane Updated", "ext-123", true))

	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "name.formatted", "value": "Jane Updated"}]
	}`

	req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	rctx.URLParams.Add("id", "user-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.PatchUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestReplaceUser(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectExec(`UPDATE users SET email = \$1, name = \$2 WHERE id = \$3`).
		WithArgs("jane.updated@example.com", "Jane Updated", "user-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec(`INSERT INTO organization_members`).
		WithArgs("org-1", "user-1", "member").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectExec(`INSERT INTO external_identities`).
		WithArgs("user-1", "org-1", "ext-123", "jane.updated@example.com").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectQuery(`SELECT`).
		WithArgs("user-1", "org-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name", "external_id", "active"}).
			AddRow("user-1", "jane.updated@example.com", "Jane Updated", "ext-123", true))

	body := `{
		"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
		"userName": "jane.updated@example.com",
		"name": {"formatted": "Jane Updated"},
		"externalId": "ext-123",
		"active": true
	}`

	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	rctx.URLParams.Add("id", "user-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.ReplaceUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestPatchUser_UpdateNameErrorReturns500(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectExec(`UPDATE users SET name = \$1`).
		WithArgs("Jane Updated", "user-1").
		WillReturnError(pgx.ErrTxClosed)

	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "name.formatted", "value": "Jane Updated"}]
	}`

	req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	rctx.URLParams.Add("id", "user-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.PatchUser(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want 500; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateUser_IdentityLookupErrorReturns500(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT user_id FROM external_identities WHERE provider = \$1 AND external_id = \$2`).
		WithArgs("org-1", "ext-123").
		WillReturnError(pgx.ErrTxClosed)

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

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want 500; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateUser_IdentityInsertErrorReturns500(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT user_id FROM external_identities WHERE provider = \$1 AND external_id = \$2`).
		WithArgs("org-1", "ext-123").
		WillReturnError(pgx.ErrNoRows)

	mock.ExpectQuery(`SELECT id, email_verified FROM users WHERE email = \$1`).
		WithArgs("jane@example.com").
		WillReturnError(pgx.ErrNoRows)

	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs("jane@example.com", "", "Jane Doe").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("user-uuid-1"))

	mock.ExpectExec(`INSERT INTO external_identities`).
		WithArgs("user-uuid-1", "org-1", "ext-123", "jane@example.com").
		WillReturnError(pgx.ErrTxClosed)

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

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want 500; body: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteUser(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectExec(`DELETE FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs("org-1", "user-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectExec(`DELETE FROM external_identities WHERE provider = \$1 AND user_id = \$2`).
		WithArgs("org-1", "user-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	rctx.URLParams.Add("id", "user-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.DeleteUser(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("got status %d, want 204", rec.Code)
	}
}

func TestDeleteUser_DBErrorReturns500(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectExec(`DELETE FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs("org-1", "user-1").
		WillReturnError(pgx.ErrTxClosed)

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	rctx.URLParams.Add("id", "user-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.DeleteUser(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want 500; body: %s", rec.Code, rec.Body.String())
	}
}
