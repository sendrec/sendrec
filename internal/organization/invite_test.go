package organization

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

type mockEmailSender struct {
	called    bool
	toEmail   string
	orgName   string
	inviter   string
	link      string
	returnErr error
}

func (m *mockEmailSender) SendOrgInvite(_ context.Context, toEmail, orgName, inviterName, acceptLink string) error {
	m.called = true
	m.toEmail = toEmail
	m.orgName = orgName
	m.inviter = inviterName
	m.link = acceptLink
	return m.returnErr
}

func TestSendInvite(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	emailMock := &mockEmailSender{}
	handler := NewHandler(mock, testBaseURL)
	handler.SetEmailSender(emailMock)
	orgID := "org-1"
	inviteeEmail := "bob@example.com"
	now := time.Now().UTC().Truncate(time.Second)

	// Caller role check
	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("owner"))

	// Already-member check
	mock.ExpectQuery(`SELECT 1 FROM organization_members om JOIN users u ON u\.id = om\.user_id WHERE om\.organization_id = \$1 AND u\.email = \$2`).
		WithArgs(orgID, inviteeEmail).
		WillReturnError(pgx.ErrNoRows)

	// Pending invite check
	mock.ExpectQuery(`SELECT 1 FROM organization_invites WHERE organization_id = \$1 AND email = \$2 AND accepted_at IS NULL AND expires_at > now\(\)`).
		WithArgs(orgID, inviteeEmail).
		WillReturnError(pgx.ErrNoRows)

	// Insert invite
	mock.ExpectQuery(`INSERT INTO organization_invites`).
		WithArgs(orgID, inviteeEmail, "member", testUserID, pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow("invite-1", now))

	// Org name for email
	mock.ExpectQuery(`SELECT name FROM organizations WHERE id = \$1`).
		WithArgs(orgID).
		WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow("Acme Corp"))

	// Inviter name for email
	mock.ExpectQuery(`SELECT name FROM users WHERE id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow("Alice"))

	body, _ := json.Marshal(sendInviteRequest{Email: inviteeEmail, Role: "member"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/organizations/{orgId}/invites", handler.SendInvite)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/organizations/"+orgID+"/invites", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp inviteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != "invite-1" {
		t.Errorf("expected ID %q, got %q", "invite-1", resp.ID)
	}
	if resp.Email != inviteeEmail {
		t.Errorf("expected email %q, got %q", inviteeEmail, resp.Email)
	}
	if resp.Role != "member" {
		t.Errorf("expected role %q, got %q", "member", resp.Role)
	}
	if !emailMock.called {
		t.Error("expected email sender to be called")
	}
	if emailMock.toEmail != inviteeEmail {
		t.Errorf("expected email sent to %q, got %q", inviteeEmail, emailMock.toEmail)
	}
	if emailMock.orgName != "Acme Corp" {
		t.Errorf("expected org name %q, got %q", "Acme Corp", emailMock.orgName)
	}
	if emailMock.inviter != "Alice" {
		t.Errorf("expected inviter name %q, got %q", "Alice", emailMock.inviter)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestSendInvite_AlreadyMember(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	orgID := "org-1"
	inviteeEmail := "bob@example.com"

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("owner"))

	mock.ExpectQuery(`SELECT 1 FROM organization_members om JOIN users u ON u\.id = om\.user_id WHERE om\.organization_id = \$1 AND u\.email = \$2`).
		WithArgs(orgID, inviteeEmail).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(1))

	body, _ := json.Marshal(sendInviteRequest{Email: inviteeEmail, Role: "member"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/organizations/{orgId}/invites", handler.SendInvite)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/organizations/"+orgID+"/invites", body))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "user is already a member" {
		t.Errorf("expected error %q, got %q", "user is already a member", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestSendInvite_PendingExists(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	orgID := "org-1"
	inviteeEmail := "bob@example.com"

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("owner"))

	mock.ExpectQuery(`SELECT 1 FROM organization_members om JOIN users u ON u\.id = om\.user_id WHERE om\.organization_id = \$1 AND u\.email = \$2`).
		WithArgs(orgID, inviteeEmail).
		WillReturnError(pgx.ErrNoRows)

	mock.ExpectQuery(`SELECT 1 FROM organization_invites WHERE organization_id = \$1 AND email = \$2 AND accepted_at IS NULL AND expires_at > now\(\)`).
		WithArgs(orgID, inviteeEmail).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(1))

	body, _ := json.Marshal(sendInviteRequest{Email: inviteeEmail, Role: "member"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/organizations/{orgId}/invites", handler.SendInvite)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/organizations/"+orgID+"/invites", body))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "invite already pending" {
		t.Errorf("expected error %q, got %q", "invite already pending", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestSendInvite_AsMember(t *testing.T) {
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

	body, _ := json.Marshal(sendInviteRequest{Email: "bob@example.com", Role: "member"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/organizations/{orgId}/invites", handler.SendInvite)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/organizations/"+orgID+"/invites", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "only owners and admins can send invites" {
		t.Errorf("expected error %q, got %q", "only owners and admins can send invites", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestListInvites(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	orgID := "org-1"
	now := time.Now().UTC().Truncate(time.Second)
	expiresAt := now.Add(inviteTokenExpiry)

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("admin"))

	mock.ExpectQuery(`SELECT i\.id, i\.email, i\.role, u\.name AS invited_by_name, i\.expires_at, i\.created_at`).
		WithArgs(orgID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "role", "invited_by_name", "expires_at", "created_at"}).
			AddRow("invite-1", "bob@example.com", "member", "Alice", expiresAt, now).
			AddRow("invite-2", "carol@example.com", "admin", "Alice", expiresAt, now))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/organizations/{orgId}/invites", handler.ListInvites)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/organizations/"+orgID+"/invites", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var items []inviteListItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 invites, got %d", len(items))
	}
	if items[0].Email != "bob@example.com" {
		t.Errorf("expected first invite email %q, got %q", "bob@example.com", items[0].Email)
	}
	if items[0].InvitedByName != "Alice" {
		t.Errorf("expected inviter name %q, got %q", "Alice", items[0].InvitedByName)
	}
	if items[1].Role != "admin" {
		t.Errorf("expected second invite role %q, got %q", "admin", items[1].Role)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRevokeInvite(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	orgID := "org-1"
	inviteID := "invite-1"

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("owner"))

	mock.ExpectExec(`DELETE FROM organization_invites WHERE id = \$1 AND organization_id = \$2 AND accepted_at IS NULL`).
		WithArgs(inviteID, orgID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/organizations/{orgId}/invites/{inviteId}", handler.RevokeInvite)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/organizations/"+orgID+"/invites/"+inviteID, nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestAcceptInvite(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	rawToken := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	tokenHash := hashInviteToken(rawToken)

	// Look up invite by token hash
	mock.ExpectQuery(`SELECT i\.id, i\.organization_id, i\.role`).
		WithArgs(tokenHash).
		WillReturnRows(pgxmock.NewRows([]string{"id", "organization_id", "role"}).
			AddRow("invite-1", "org-1", "member"))

	// Get user email
	mock.ExpectQuery(`SELECT email FROM users WHERE id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"email"}).AddRow("alice@example.com"))

	// Get invite email
	mock.ExpectQuery(`SELECT email FROM organization_invites WHERE id = \$1`).
		WithArgs("invite-1").
		WillReturnRows(pgxmock.NewRows([]string{"email"}).AddRow("alice@example.com"))

	// Mark accepted
	mock.ExpectExec(`UPDATE organization_invites SET accepted_at = now\(\) WHERE id = \$1`).
		WithArgs("invite-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// Insert membership
	mock.ExpectExec(`INSERT INTO organization_members .+ ON CONFLICT DO NOTHING`).
		WithArgs("org-1", testUserID, "member").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// Get org details
	mock.ExpectQuery(`SELECT id, name, slug FROM organizations WHERE id = \$1`).
		WithArgs("org-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "slug"}).
			AddRow("org-1", "Acme Corp", "acme-corp"))

	body, _ := json.Marshal(acceptInviteRequest{Token: rawToken})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/invites/accept", handler.AcceptInvite)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/invites/accept", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp acceptInviteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.OrganizationID != "org-1" {
		t.Errorf("expected organization ID %q, got %q", "org-1", resp.OrganizationID)
	}
	if resp.Name != "Acme Corp" {
		t.Errorf("expected name %q, got %q", "Acme Corp", resp.Name)
	}
	if resp.Slug != "acme-corp" {
		t.Errorf("expected slug %q, got %q", "acme-corp", resp.Slug)
	}
	if resp.Role != "member" {
		t.Errorf("expected role %q, got %q", "member", resp.Role)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestAcceptInvite_Expired(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	rawToken := "expired0000000000000000000000000000000000000000000000000000000000"
	tokenHash := hashInviteToken(rawToken)

	mock.ExpectQuery(`SELECT i\.id, i\.organization_id, i\.role`).
		WithArgs(tokenHash).
		WillReturnError(pgx.ErrNoRows)

	body, _ := json.Marshal(acceptInviteRequest{Token: rawToken})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/invites/accept", handler.AcceptInvite)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/invites/accept", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "invalid or expired invite" {
		t.Errorf("expected error %q, got %q", "invalid or expired invite", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestAcceptInvite_WrongEmail(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	rawToken := "wrongemail00000000000000000000000000000000000000000000000000000000"
	tokenHash := hashInviteToken(rawToken)

	mock.ExpectQuery(`SELECT i\.id, i\.organization_id, i\.role`).
		WithArgs(tokenHash).
		WillReturnRows(pgxmock.NewRows([]string{"id", "organization_id", "role"}).
			AddRow("invite-1", "org-1", "member"))

	mock.ExpectQuery(`SELECT email FROM users WHERE id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"email"}).AddRow("alice@example.com"))

	mock.ExpectQuery(`SELECT email FROM organization_invites WHERE id = \$1`).
		WithArgs("invite-1").
		WillReturnRows(pgxmock.NewRows([]string{"email"}).AddRow("different@example.com"))

	body, _ := json.Marshal(acceptInviteRequest{Token: rawToken})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/invites/accept", handler.AcceptInvite)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/invites/accept", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "invite was sent to a different email" {
		t.Errorf("expected error %q, got %q", "invite was sent to a different email", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
