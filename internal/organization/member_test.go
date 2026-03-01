package organization

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

const testTargetUserID = "660e8400-e29b-41d4-a716-446655440001"

func TestListMembers(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	orgID := "org-1"
	now := time.Now().UTC().Truncate(time.Second)

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("owner"))

	mock.ExpectQuery(`SELECT u\.id, u\.name, u\.email, om\.role, om\.joined_at`).
		WithArgs(orgID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "email", "role", "joined_at"}).
			AddRow(testUserID, "Alice", "alice@example.com", "owner", now).
			AddRow(testTargetUserID, "Bob", "bob@example.com", "member", now))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/organizations/{orgId}/members", handler.ListMembers)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/organizations/"+orgID+"/members", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var members []memberResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &members); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	if members[0].Name != "Alice" {
		t.Errorf("expected first member name %q, got %q", "Alice", members[0].Name)
	}
	if members[0].Role != "owner" {
		t.Errorf("expected first member role %q, got %q", "owner", members[0].Role)
	}
	if members[1].Name != "Bob" {
		t.Errorf("expected second member name %q, got %q", "Bob", members[1].Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestListMembers_NonMember(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	orgID := "org-1"

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/organizations/{orgId}/members", handler.ListMembers)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/organizations/"+orgID+"/members", nil))

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

func TestRemoveMember_AsOwner(t *testing.T) {
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

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testTargetUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("member"))

	mock.ExpectExec(`DELETE FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testTargetUserID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/organizations/{orgId}/members/{userId}", handler.RemoveMember)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/organizations/"+orgID+"/members/"+testTargetUserID, nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRemoveMember_AsMember(t *testing.T) {
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

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/organizations/{orgId}/members/{userId}", handler.RemoveMember)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/organizations/"+orgID+"/members/"+testTargetUserID, nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "only owners and admins can remove members" {
		t.Errorf("expected error %q, got %q", "only owners and admins can remove members", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRemoveMember_Self(t *testing.T) {
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

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/organizations/{orgId}/members/{userId}", handler.RemoveMember)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/organizations/"+orgID+"/members/"+testUserID, nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "cannot remove yourself" {
		t.Errorf("expected error %q, got %q", "cannot remove yourself", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRemoveMember_SoleOwner(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testBaseURL)
	orgID := "org-1"
	otherOwnerID := "770e8400-e29b-41d4-a716-446655440002"

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("owner"))

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs(orgID, otherOwnerID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("owner"))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM organization_members WHERE organization_id = \$1 AND role = 'owner'`).
		WithArgs(orgID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/organizations/{orgId}/members/{userId}", handler.RemoveMember)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/organizations/"+orgID+"/members/"+otherOwnerID, nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "cannot remove the only owner" {
		t.Errorf("expected error %q, got %q", "cannot remove the only owner", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdateMemberRole_AsOwner(t *testing.T) {
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

	mock.ExpectExec(`UPDATE organization_members SET role = \$1 WHERE organization_id = \$2 AND user_id = \$3`).
		WithArgs("admin", orgID, testTargetUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(updateRoleRequest{Role: "admin"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/organizations/{orgId}/members/{userId}/role", handler.UpdateMemberRole)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/organizations/"+orgID+"/members/"+testTargetUserID+"/role", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["role"] != "admin" {
		t.Errorf("expected role %q, got %q", "admin", resp["role"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdateMemberRole_AsAdmin(t *testing.T) {
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

	body, _ := json.Marshal(updateRoleRequest{Role: "member"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/organizations/{orgId}/members/{userId}/role", handler.UpdateMemberRole)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/organizations/"+orgID+"/members/"+testTargetUserID+"/role", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "only owners can change roles" {
		t.Errorf("expected error %q, got %q", "only owners can change roles", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdateMemberRole_Self(t *testing.T) {
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

	body, _ := json.Marshal(updateRoleRequest{Role: "admin"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/organizations/{orgId}/members/{userId}/role", handler.UpdateMemberRole)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/organizations/"+orgID+"/members/"+testUserID+"/role", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "cannot change your own role" {
		t.Errorf("expected error %q, got %q", "cannot change your own role", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
