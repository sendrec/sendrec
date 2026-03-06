package sso

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/integration"
)

const testJWTSecret = "test-sso-jwt-secret"
const testBaseURL = "http://localhost:3000"

// mockProvider is a test double that satisfies the Provider interface.
type mockProvider struct {
	authURL  string
	userInfo *UserInfo
	err      error
}

func (m *mockProvider) AuthURL(state string) string {
	return m.authURL + "?state=" + state
}

func (m *mockProvider) Exchange(_ context.Context, _ string) (*UserInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.userInfo, nil
}

func newTestHandler(t *testing.T) (*Handler, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	handler := NewHandler(mock, testJWTSecret, testBaseURL, false, nil)
	return handler, mock
}

// callWithChiParam makes a request through a chi router so that
// chi.URLParam("provider") is populated.
func callWithChiParam(handler http.HandlerFunc, method, path, paramName, paramValue string, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(paramName, paramValue)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	handler.ServeHTTP(rec, r)
	return rec
}

func TestProviders_ReturnsEnabledList(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	handler.RegisterProvider("github", &mockProvider{authURL: "https://github.com/login"})
	handler.RegisterProvider("google", &mockProvider{authURL: "https://accounts.google.com"})

	req := httptest.NewRequest(http.MethodGet, "/api/sso/providers", nil)
	rec := httptest.NewRecorder()
	handler.Providers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp providersResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Providers) != 2 {
		t.Fatalf("providers count = %d, want 2", len(resp.Providers))
	}

	providers := make(map[string]bool)
	for _, p := range resp.Providers {
		providers[p] = true
	}
	if !providers["github"] || !providers["google"] {
		t.Errorf("providers = %v, want github and google", resp.Providers)
	}
}

func TestInitiateSSO_RedirectsToProvider(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	handler.RegisterProvider("github", &mockProvider{authURL: "https://github.com/login/oauth/authorize"})

	req := httptest.NewRequest(http.MethodGet, "/api/sso/github/initiate", nil)
	rec := callWithChiParam(handler.Initiate, http.MethodGet, "/api/sso/{provider}/initiate", "provider", "github", req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}
	if len(location) < 10 {
		t.Fatalf("Location = %q, want a valid URL", location)
	}

	// Verify the sso_state cookie was set.
	cookies := rec.Result().Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "sso_state" {
			stateCookie = c
			break
		}
	}
	if stateCookie == nil {
		t.Fatal("sso_state cookie not set")
	}
	if stateCookie.MaxAge != 300 {
		t.Errorf("sso_state MaxAge = %d, want 300", stateCookie.MaxAge)
	}
	if !stateCookie.HttpOnly {
		t.Error("sso_state cookie should be HttpOnly")
	}
}

func TestInitiateSSO_UnknownProvider(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/sso/unknown/initiate", nil)
	rec := callWithChiParam(handler.Initiate, http.MethodGet, "/api/sso/{provider}/initiate", "provider", "unknown", req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestCallback_NewUser_CreatesAccount(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	handler.RegisterProvider("github", &mockProvider{
		authURL: "https://github.com/login",
		userInfo: &UserInfo{
			ExternalID: "gh-12345",
			Email:      "newuser@example.com",
			Name:       "New User",
		},
	})

	// Step 1: external identity lookup -- not found
	mock.ExpectQuery(`SELECT user_id FROM external_identities WHERE provider = \$1 AND external_id = \$2`).
		WithArgs("github", "gh-12345").
		WillReturnError(pgx.ErrNoRows)

	// Step 2: user lookup by email -- not found
	mock.ExpectQuery(`SELECT id, email_verified FROM users WHERE email = \$1`).
		WithArgs("newuser@example.com").
		WillReturnError(pgx.ErrNoRows)

	// Step 3: create user
	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs("newuser@example.com", "", "New User").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("user-uuid-1"))

	// Step 4: create external identity
	mock.ExpectExec(`INSERT INTO external_identities`).
		WithArgs("user-uuid-1", "github", "gh-12345", "newuser@example.com").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// Step 5: issue tokens (insert refresh token)
	mock.ExpectExec(`INSERT INTO refresh_tokens`).
		WithArgs(pgxmock.AnyArg(), "user-uuid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	state := "valid-state-12345"
	req := httptest.NewRequest(http.MethodGet, "/api/sso/github/callback?code=auth-code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "sso_state", Value: state})
	rec := callWithChiParam(handler.Callback, http.MethodGet, "/api/sso/{provider}/callback", "provider", "github", req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}

	// Verify it redirects to /login?sso_token=...
	if !strings.Contains(location, "/login?sso_token=") {
		t.Fatalf("Location = %q, want to contain /login?sso_token=", location)
	}

	// Verify refresh token cookie was set.
	var refreshCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh_token" && c.Value != "" {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatal("refresh_token cookie not set")
	}

	// Verify the access token in the redirect URL is valid.
	parsed, parseErr := url.Parse(location)
	if parseErr != nil {
		t.Fatalf("parse location URL: %v", parseErr)
	}
	accessToken := parsed.Query().Get("sso_token")
	claims, err := auth.ValidateToken(testJWTSecret, accessToken)
	if err != nil {
		t.Fatalf("access token validation failed: %v", err)
	}
	if claims.UserID != "user-uuid-1" {
		t.Errorf("claims.UserID = %q, want %q", claims.UserID, "user-uuid-1")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCallback_ExistingIdentity_SignsIn(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	handler.RegisterProvider("github", &mockProvider{
		authURL: "https://github.com/login",
		userInfo: &UserInfo{
			ExternalID: "gh-existing",
			Email:      "existing@example.com",
			Name:       "Existing User",
		},
	})

	// External identity found
	mock.ExpectQuery(`SELECT user_id FROM external_identities WHERE provider = \$1 AND external_id = \$2`).
		WithArgs("github", "gh-existing").
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow("user-uuid-existing"))

	// Issue tokens
	mock.ExpectExec(`INSERT INTO refresh_tokens`).
		WithArgs(pgxmock.AnyArg(), "user-uuid-existing", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	state := "valid-state-existing"
	req := httptest.NewRequest(http.MethodGet, "/api/sso/github/callback?code=auth-code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "sso_state", Value: state})
	rec := callWithChiParam(handler.Callback, http.MethodGet, "/api/sso/{provider}/callback", "provider", "github", req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "/login?sso_token=") {
		t.Fatalf("Location = %q, want /login?sso_token=", location)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCallback_ExistingVerifiedUser_LinksIdentity(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	handler.RegisterProvider("google", &mockProvider{
		authURL: "https://accounts.google.com",
		userInfo: &UserInfo{
			ExternalID: "google-456",
			Email:      "verified@example.com",
			Name:       "Verified User",
		},
	})

	// External identity not found
	mock.ExpectQuery(`SELECT user_id FROM external_identities WHERE provider = \$1 AND external_id = \$2`).
		WithArgs("google", "google-456").
		WillReturnError(pgx.ErrNoRows)

	// User exists and is verified
	mock.ExpectQuery(`SELECT id, email_verified FROM users WHERE email = \$1`).
		WithArgs("verified@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email_verified"}).AddRow("user-uuid-verified", true))

	// Link identity
	mock.ExpectExec(`INSERT INTO external_identities`).
		WithArgs("user-uuid-verified", "google", "google-456", "verified@example.com").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// Issue tokens
	mock.ExpectExec(`INSERT INTO refresh_tokens`).
		WithArgs(pgxmock.AnyArg(), "user-uuid-verified", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	state := "valid-state-link"
	req := httptest.NewRequest(http.MethodGet, "/api/sso/google/callback?code=auth-code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "sso_state", Value: state})
	rec := callWithChiParam(handler.Callback, http.MethodGet, "/api/sso/{provider}/callback", "provider", "google", req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "/login?sso_token=") {
		t.Fatalf("Location = %q, want /login?sso_token=", location)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCallback_UnverifiedUser_RejectsLink(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	handler.RegisterProvider("github", &mockProvider{
		authURL: "https://github.com/login",
		userInfo: &UserInfo{
			ExternalID: "gh-unverified",
			Email:      "unverified@example.com",
			Name:       "Unverified User",
		},
	})

	// External identity not found
	mock.ExpectQuery(`SELECT user_id FROM external_identities WHERE provider = \$1 AND external_id = \$2`).
		WithArgs("github", "gh-unverified").
		WillReturnError(pgx.ErrNoRows)

	// User exists but is NOT verified
	mock.ExpectQuery(`SELECT id, email_verified FROM users WHERE email = \$1`).
		WithArgs("unverified@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email_verified"}).AddRow("user-uuid-unverified", false))

	state := "valid-state-unverified"
	req := httptest.NewRequest(http.MethodGet, "/api/sso/github/callback?code=auth-code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "sso_state", Value: state})
	rec := callWithChiParam(handler.Callback, http.MethodGet, "/api/sso/{provider}/callback", "provider", "github", req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "sso_error=") {
		t.Fatalf("Location = %q, want sso_error= in redirect", location)
	}
	if !strings.Contains(location, "email+not+verified") && !strings.Contains(location, "email%20not%20verified") {
		t.Fatalf("Location = %q, want 'email not verified' error", location)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCallback_StateMismatch_RejectsRequest(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	handler.RegisterProvider("github", &mockProvider{authURL: "https://github.com/login"})

	req := httptest.NewRequest(http.MethodGet, "/api/sso/github/callback?code=auth-code&state=wrong-state", nil)
	req.AddCookie(&http.Cookie{Name: "sso_state", Value: "correct-state"})
	rec := callWithChiParam(handler.Callback, http.MethodGet, "/api/sso/{provider}/callback", "provider", "github", req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "sso_error=") {
		t.Fatalf("Location = %q, want sso_error= in redirect", location)
	}
}

func TestCallback_MissingStateCookie_RejectsRequest(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	handler.RegisterProvider("github", &mockProvider{authURL: "https://github.com/login"})

	req := httptest.NewRequest(http.MethodGet, "/api/sso/github/callback?code=auth-code&state=some-state", nil)
	rec := callWithChiParam(handler.Callback, http.MethodGet, "/api/sso/{provider}/callback", "provider", "github", req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "sso_error=") {
		t.Fatalf("Location = %q, want sso_error= in redirect", location)
	}
}

// --- Task 6: Workspace SSO Config CRUD Tests ---

var testEncryptionKey = integration.DeriveKey("test-encryption-secret")

func newTestHandlerWithKey(t *testing.T) (*Handler, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	handler := NewHandler(mock, testJWTSecret, testBaseURL, false, testEncryptionKey)
	return handler, mock
}

func requestWithOrg(r *http.Request, orgID, role string) *http.Request {
	ctx := auth.ContextWithUserID(r.Context(), "user-1")
	ctx = auth.ContextWithOrg(ctx, orgID, role)
	// Set chi URL params for handlers that use chi.URLParam
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", orgID)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

func TestSaveConfig_Success(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT role FROM organization_members`).
		WithArgs("org-1", "user-1").
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("admin"))

	mock.ExpectExec(`INSERT INTO organization_sso_configs`).
		WithArgs("org-1", "https://accounts.google.com", "client-123", pgxmock.AnyArg(), false).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := `{"issuerUrl":"https://accounts.google.com","clientId":"client-123","clientSecret":"secret-456","enforceSso":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/sso/config", strings.NewReader(body))
	req = requestWithOrg(req, "org-1", "admin")

	rec := httptest.NewRecorder()
	handler.SaveConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSaveConfig_RequiresOrgContext(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT role FROM organization_members`).
		WithArgs("", "").
		WillReturnError(pgx.ErrNoRows)

	body := `{"issuerUrl":"https://example.com","clientId":"id","clientSecret":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sso/config", strings.NewReader(body))
	// No org context set.

	rec := httptest.NewRecorder()
	handler.SaveConfig(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestSaveConfig_RequiresAdmin(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT role FROM organization_members`).
		WithArgs("org-1", "user-1").
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("member"))

	body := `{"issuerUrl":"https://example.com","clientId":"id","clientSecret":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sso/config", strings.NewReader(body))
	req = requestWithOrg(req, "org-1", "member")

	rec := httptest.NewRecorder()
	handler.SaveConfig(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGetConfig_Success(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT provider, issuer_url, client_id, enforce_sso FROM organization_sso_configs`).
		WithArgs("org-1").
		WillReturnRows(pgxmock.NewRows([]string{"provider", "issuer_url", "client_id", "enforce_sso"}).
			AddRow("oidc", "https://accounts.google.com", "client-123", true))

	req := httptest.NewRequest(http.MethodGet, "/api/sso/config", nil)
	req = requestWithOrg(req, "org-1", "admin")

	rec := httptest.NewRecorder()
	handler.GetConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp ssoConfigResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !resp.Configured {
		t.Error("configured = false, want true")
	}
	if resp.Provider != "oidc" {
		t.Errorf("provider = %q, want %q", resp.Provider, "oidc")
	}
	if resp.IssuerURL != "https://accounts.google.com" {
		t.Errorf("issuer_url = %q, want %q", resp.IssuerURL, "https://accounts.google.com")
	}
	if resp.ClientID != "client-123" {
		t.Errorf("client_id = %q, want %q", resp.ClientID, "client-123")
	}
	if resp.ClientSecret != "******" {
		t.Errorf("client_secret = %q, want %q", resp.ClientSecret, "******")
	}
	if !resp.EnforceSSO {
		t.Error("enforce_sso = false, want true")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetConfig_NotConfigured(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT provider, issuer_url, client_id, enforce_sso FROM organization_sso_configs`).
		WithArgs("org-1").
		WillReturnError(pgx.ErrNoRows)

	req := httptest.NewRequest(http.MethodGet, "/api/sso/config", nil)
	req = requestWithOrg(req, "org-1", "admin")

	rec := httptest.NewRecorder()
	handler.GetConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp ssoConfigResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Configured {
		t.Error("configured = true, want false")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDeleteConfig_Success(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT role FROM organization_members`).
		WithArgs("org-1", "user-1").
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("owner"))

	mock.ExpectExec(`DELETE FROM organization_sso_configs WHERE organization_id`).
		WithArgs("org-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	req := httptest.NewRequest(http.MethodDelete, "/api/sso/config", nil)
	req = requestWithOrg(req, "org-1", "owner")

	rec := httptest.NewRecorder()
	handler.DeleteConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDeleteConfig_RequiresOwner(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT role FROM organization_members`).
		WithArgs("org-1", "user-1").
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("admin"))

	req := httptest.NewRequest(http.MethodDelete, "/api/sso/config", nil)
	req = requestWithOrg(req, "org-1", "admin")

	rec := httptest.NewRecorder()
	handler.DeleteConfig(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestInitiateOrgSSO_NoEmail(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/sso/org/initiate", nil)

	rec := httptest.NewRecorder()
	handler.InitiateOrgSSO(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestInitiateOrgSSO_NoConfig(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT o.id, c.issuer_url, c.client_id, c.client_secret_encrypted`).
		WithArgs("user@example.com").
		WillReturnError(pgx.ErrNoRows)

	req := httptest.NewRequest(http.MethodGet, "/api/sso/org/initiate?email=user@example.com", nil)

	rec := httptest.NewRecorder()
	handler.InitiateOrgSSO(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- Task 9: Connected Accounts API Tests ---

func TestListIdentities_Success(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT password FROM users WHERE id = \$1`).
		WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows([]string{"password"}).AddRow("$2a$10$hashed"))

	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT provider, external_id, email, created_at FROM external_identities WHERE user_id = \$1 ORDER BY created_at`).
		WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows([]string{"provider", "external_id", "email", "created_at"}).
			AddRow("github", "gh-123", "user@example.com", createdAt).
			AddRow("google", "ggl-456", "user@example.com", createdAt))

	req := httptest.NewRequest(http.MethodGet, "/api/sso/identities", nil)
	ctx := auth.ContextWithUserID(req.Context(), "user-1")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ListIdentities(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Identities  []identityResponse `json:"identities"`
		HasPassword bool               `json:"hasPassword"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Identities) != 2 {
		t.Fatalf("identities count = %d, want 2", len(resp.Identities))
	}
	if resp.Identities[0].Provider != "github" {
		t.Errorf("first provider = %q, want %q", resp.Identities[0].Provider, "github")
	}
	if resp.Identities[1].Provider != "google" {
		t.Errorf("second provider = %q, want %q", resp.Identities[1].Provider, "google")
	}
	if !resp.HasPassword {
		t.Error("expected hasPassword = true")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUnlinkIdentity_Success(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	// User has a password set.
	mock.ExpectQuery(`SELECT password FROM users WHERE id = \$1`).
		WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows([]string{"password"}).AddRow("$2a$10$hashed"))

	// Count identities = 2.
	mock.ExpectQuery(`SELECT count\(\*\) FROM external_identities WHERE user_id = \$1`).
		WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	// Delete the identity.
	mock.ExpectExec(`DELETE FROM external_identities WHERE user_id = \$1 AND provider = \$2`).
		WithArgs("user-1", "github").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	req := httptest.NewRequest(http.MethodDelete, "/api/sso/identities/github", nil)
	ctx := auth.ContextWithUserID(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rec := callWithChiParam(handler.UnlinkIdentity, http.MethodDelete, "/api/sso/identities/{provider}", "provider", "github", req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUnlinkIdentity_BlocksLastIdentityNoPassword(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	// User has NO password set (empty string).
	mock.ExpectQuery(`SELECT password FROM users WHERE id = \$1`).
		WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows([]string{"password"}).AddRow(""))

	// Count identities = 1 (last one).
	mock.ExpectQuery(`SELECT count\(\*\) FROM external_identities WHERE user_id = \$1`).
		WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	req := httptest.NewRequest(http.MethodDelete, "/api/sso/identities/github", nil)
	ctx := auth.ContextWithUserID(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rec := callWithChiParam(handler.UnlinkIdentity, http.MethodDelete, "/api/sso/identities/{provider}", "provider", "github", req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	// Verify the error message mentions the constraint.
	var errResp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(errResp["error"], "cannot unlink last identity") {
		t.Errorf("error = %q, want to contain %q", errResp["error"], "cannot unlink last identity")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- OrgCallback Tests ---

func TestOrgCallback_ErrorParam_Redirects(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/org/callback?error=access_denied&error_description=User+cancelled", nil)
	rec := httptest.NewRecorder()
	handler.OrgCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "sso_error=") {
		t.Fatalf("Location = %q, want sso_error= in redirect", location)
	}
	if !strings.Contains(location, "cancelled") {
		t.Fatalf("Location = %q, want error description containing 'cancelled'", location)
	}
}

func TestOrgCallback_ErrorParam_DefaultDescription(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/org/callback?error=access_denied", nil)
	rec := httptest.NewRecorder()
	handler.OrgCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "denied") {
		t.Fatalf("Location = %q, want 'denied' in error", location)
	}
}

func TestOrgCallback_MissingStateCookie(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/org/callback?code=auth-code&state=some-state", nil)
	rec := httptest.NewRecorder()
	handler.OrgCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "sso_error=") {
		t.Fatalf("Location = %q, want sso_error=", location)
	}
}

func TestOrgCallback_MissingOrgCookie(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/org/callback?code=auth-code&state=valid-state", nil)
	req.AddCookie(&http.Cookie{Name: "sso_state", Value: "valid-state"})
	rec := httptest.NewRecorder()
	handler.OrgCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "sso_error=") {
		t.Fatalf("Location = %q, want sso_error=", location)
	}
}

func TestOrgCallback_StateMismatch(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/org/callback?code=auth-code&state=wrong-state", nil)
	req.AddCookie(&http.Cookie{Name: "sso_state", Value: "correct-state"})
	req.AddCookie(&http.Cookie{Name: "sso_org", Value: "org-1"})
	rec := httptest.NewRecorder()
	handler.OrgCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "sso_error=") {
		t.Fatalf("Location = %q, want sso_error=", location)
	}
}

func TestOrgCallback_MissingCode(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	state := "valid-state-no-code"
	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/org/callback?state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "sso_state", Value: state})
	req.AddCookie(&http.Cookie{Name: "sso_org", Value: "org-1"})
	rec := httptest.NewRecorder()
	handler.OrgCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "sso_error=") {
		t.Fatalf("Location = %q, want sso_error=", location)
	}
}

func TestOrgCallback_ConfigNotFound(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT issuer_url, client_id, client_secret_encrypted FROM organization_sso_configs`).
		WithArgs("org-1").
		WillReturnError(pgx.ErrNoRows)

	state := "valid-state-no-config"
	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/org/callback?code=auth-code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "sso_state", Value: state})
	req.AddCookie(&http.Cookie{Name: "sso_org", Value: "org-1"})
	rec := httptest.NewRecorder()
	handler.OrgCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "sso_error=") {
		t.Fatalf("Location = %q, want sso_error=", location)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOrgCallback_ClearsStateCookies(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT issuer_url, client_id, client_secret_encrypted FROM organization_sso_configs`).
		WithArgs("org-1").
		WillReturnError(pgx.ErrNoRows)

	state := "valid-state-cookies"
	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/org/callback?code=auth-code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "sso_state", Value: state})
	req.AddCookie(&http.Cookie{Name: "sso_org", Value: "org-1"})
	rec := httptest.NewRecorder()
	handler.OrgCallback(rec, req)

	var stateCleared, orgCleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == "sso_state" && c.MaxAge < 0 {
			stateCleared = true
		}
		if c.Name == "sso_org" && c.MaxAge < 0 {
			orgCleared = true
		}
	}
	if !stateCleared {
		t.Error("sso_state cookie was not cleared")
	}
	if !orgCleared {
		t.Error("sso_org cookie was not cleared")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// --- Additional SSO Config Tests ---

func TestSaveConfig_WithEnforcement(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT role FROM organization_members`).
		WithArgs("org-1", "user-1").
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("owner"))

	mock.ExpectExec(`INSERT INTO organization_sso_configs`).
		WithArgs("org-1", "https://login.microsoftonline.com/tenant/v2.0", "ms-client", pgxmock.AnyArg(), true).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := `{"issuerUrl":"https://login.microsoftonline.com/tenant/v2.0","clientId":"ms-client","clientSecret":"ms-secret","enforceSso":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/sso/config", strings.NewReader(body))
	req = requestWithOrg(req, "org-1", "owner")

	rec := httptest.NewRecorder()
	handler.SaveConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSaveConfig_ViewerBlocked(t *testing.T) {
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT role FROM organization_members`).
		WithArgs("org-1", "user-1").
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("viewer"))

	body := `{"issuerUrl":"https://example.com","clientId":"id","clientSecret":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sso/config", strings.NewReader(body))
	req = requestWithOrg(req, "org-1", "viewer")

	rec := httptest.NewRecorder()
	handler.SaveConfig(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestOrgCallback_Success(t *testing.T) {
	server, _ := newOIDCTestServer(t)
	defer server.Close()

	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	encryptedSecret, err := integration.Encrypt(testEncryptionKey, "test-client-secret")
	if err != nil {
		t.Fatalf("encrypt client secret: %v", err)
	}

	// Load SSO config for the organization.
	mock.ExpectQuery(`SELECT issuer_url, client_id, client_secret_encrypted FROM organization_sso_configs`).
		WithArgs("org-1").
		WillReturnRows(pgxmock.NewRows([]string{"issuer_url", "client_id", "client_secret_encrypted"}).
			AddRow(server.URL, "test-client-id", encryptedSecret))

	// resolveUser: external identity lookup -- not found.
	mock.ExpectQuery(`SELECT user_id FROM external_identities WHERE provider = \$1 AND external_id = \$2`).
		WithArgs("org-1", "oidc-user-123").
		WillReturnError(pgx.ErrNoRows)

	// resolveUser: user lookup by email -- not found.
	mock.ExpectQuery(`SELECT id, email_verified FROM users WHERE email = \$1`).
		WithArgs("oidc@example.com").
		WillReturnError(pgx.ErrNoRows)

	// resolveUser: create user.
	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs("oidc@example.com", "", "OIDC User").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("user-uuid-sso"))

	// resolveUser: create external identity.
	mock.ExpectExec(`INSERT INTO external_identities`).
		WithArgs("user-uuid-sso", "org-1", "oidc-user-123", "oidc@example.com").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// Auto-add user as organization member.
	mock.ExpectExec(`INSERT INTO organization_members`).
		WithArgs("org-1", "user-uuid-sso").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// issueTokens: insert refresh token.
	mock.ExpectExec(`INSERT INTO refresh_tokens`).
		WithArgs(pgxmock.AnyArg(), "user-uuid-sso", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	state := "valid-org-state-12345"
	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/org/callback?code=auth-code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "sso_state", Value: state})
	req.AddCookie(&http.Cookie{Name: "sso_org", Value: "org-1"})

	rec := httptest.NewRecorder()
	handler.OrgCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusFound, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}

	if !strings.Contains(location, "/login?sso_token=") {
		t.Fatalf("Location = %q, want to contain /login?sso_token=", location)
	}

	// Verify access token is valid.
	parsed, parseErr := url.Parse(location)
	if parseErr != nil {
		t.Fatalf("parse location URL: %v", parseErr)
	}
	accessToken := parsed.Query().Get("sso_token")
	claims, claimsErr := auth.ValidateToken(testJWTSecret, accessToken)
	if claimsErr != nil {
		t.Fatalf("access token validation failed: %v", claimsErr)
	}
	if claims.UserID != "user-uuid-sso" {
		t.Errorf("claims.UserID = %q, want %q", claims.UserID, "user-uuid-sso")
	}

	// Verify refresh token cookie was set.
	var refreshCookie *http.Cookie
	var stateCleared, orgCleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh_token" && c.Value != "" {
			refreshCookie = c
		}
		if c.Name == "sso_state" && c.MaxAge < 0 {
			stateCleared = true
		}
		if c.Name == "sso_org" && c.MaxAge < 0 {
			orgCleared = true
		}
	}
	if refreshCookie == nil {
		t.Fatal("refresh_token cookie not set")
	}
	if !stateCleared {
		t.Error("sso_state cookie was not cleared")
	}
	if !orgCleared {
		t.Error("sso_org cookie was not cleared")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUnlinkIdentity_AllowedWithPassword(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT password FROM users WHERE id = \$1`).
		WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows([]string{"password"}).AddRow("$2a$10$hashed"))

	mock.ExpectQuery(`SELECT count\(\*\) FROM external_identities WHERE user_id = \$1`).
		WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectExec(`DELETE FROM external_identities WHERE user_id = \$1 AND provider = \$2`).
		WithArgs("user-1", "google").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	req := httptest.NewRequest(http.MethodDelete, "/api/sso/identities/google", nil)
	ctx := auth.ContextWithUserID(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rec := callWithChiParam(handler.UnlinkIdentity, http.MethodDelete, "/api/sso/identities/{provider}", "provider", "google", req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

