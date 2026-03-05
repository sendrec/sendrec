package video

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/auth"
)

const testTargetOrgID = "org-target-550e8400-e29b-41d4-a716-446655440001"

func newTransferHandler(mock pgxmock.PgxPoolIface) *Handler {
	return NewHandler(mock, &mockStorage{}, testBaseURL, 0, 25, 300, 3, testJWTSecret, false)
}

func strPtr(s string) *string {
	return &s
}

func TestTransfer_PersonalToOrg(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newTransferHandler(mock)

	mock.ExpectQuery("SELECT user_id, organization_id, status, title FROM videos").
		WithArgs("vid-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "organization_id", "status", "title"}).
			AddRow(testUserID, nil, "ready", "Test Video"))

	mock.ExpectQuery("SELECT role FROM organization_members").
		WithArgs(testTargetOrgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("member"))

	mock.ExpectExec("UPDATE videos SET organization_id").
		WithArgs(strPtr(testTargetOrgID), "vid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec("DELETE FROM video_tags").
		WithArgs("vid-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	body, _ := json.Marshal(transferRequest{OrganizationID: strPtr(testTargetOrgID)})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/transfer", handler.Transfer)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/vid-1/transfer", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp transferResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != "vid-1" || resp.Title != "Test Video" || resp.OrganizationID == nil || *resp.OrganizationID != testTargetOrgID {
		t.Errorf("unexpected response: %+v", resp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTransfer_OrgToPersonal(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newTransferHandler(mock)

	mock.ExpectQuery("SELECT user_id, organization_id, status, title FROM videos").
		WithArgs("vid-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "organization_id", "status", "title"}).
			AddRow(testUserID, strPtr(testOrgID), "ready", "Org Video"))

	mock.ExpectExec("UPDATE videos SET organization_id").
		WithArgs((*string)(nil), "vid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec("DELETE FROM video_tags").
		WithArgs("vid-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 2))

	body, _ := json.Marshal(transferRequest{OrganizationID: nil})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/transfer", handler.Transfer)

	rec := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodPost, "/api/videos/vid-1/transfer", body)
	ctx := auth.ContextWithOrg(req.Context(), testOrgID, "member")
	r.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTransfer_OrgToOtherOrg(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newTransferHandler(mock)

	mock.ExpectQuery("SELECT user_id, organization_id, status, title FROM videos").
		WithArgs("vid-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "organization_id", "status", "title"}).
			AddRow(testUserID, strPtr(testOrgID), "ready", "Cross-Org Video"))

	mock.ExpectQuery("SELECT role FROM organization_members").
		WithArgs(testTargetOrgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("admin"))

	mock.ExpectExec("UPDATE videos SET organization_id").
		WithArgs(strPtr(testTargetOrgID), "vid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec("DELETE FROM video_tags").
		WithArgs("vid-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	body, _ := json.Marshal(transferRequest{OrganizationID: strPtr(testTargetOrgID)})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/transfer", handler.Transfer)

	rec := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodPost, "/api/videos/vid-1/transfer", body)
	ctx := auth.ContextWithOrg(req.Context(), testOrgID, "member")
	r.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTransfer_OrgAdminTransfersOtherVideo(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newTransferHandler(mock)
	otherUserID := "other-user-550e8400-e29b-41d4-a716-446655440099"

	mock.ExpectQuery("SELECT user_id, organization_id, status, title FROM videos").
		WithArgs("vid-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "organization_id", "status", "title"}).
			AddRow(otherUserID, strPtr(testOrgID), "ready", "Other User Video"))

	mock.ExpectExec("UPDATE videos SET organization_id").
		WithArgs((*string)(nil), "vid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec("DELETE FROM video_tags").
		WithArgs("vid-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	body, _ := json.Marshal(transferRequest{OrganizationID: nil})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/transfer", handler.Transfer)

	rec := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodPost, "/api/videos/vid-1/transfer", body)
	ctx := auth.ContextWithOrg(req.Context(), testOrgID, "admin")
	r.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTransfer_MemberCannotTransferOthersVideo(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newTransferHandler(mock)
	otherUserID := "other-user-550e8400-e29b-41d4-a716-446655440099"

	mock.ExpectQuery("SELECT user_id, organization_id, status, title FROM videos").
		WithArgs("vid-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "organization_id", "status", "title"}).
			AddRow(otherUserID, strPtr(testOrgID), "ready", "Other User Video"))

	body, _ := json.Marshal(transferRequest{OrganizationID: nil})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/transfer", handler.Transfer)

	rec := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodPost, "/api/videos/vid-1/transfer", body)
	ctx := auth.ContextWithOrg(req.Context(), testOrgID, "member")
	r.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTransfer_NonMemberOfTargetBlocked(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newTransferHandler(mock)

	mock.ExpectQuery("SELECT user_id, organization_id, status, title FROM videos").
		WithArgs("vid-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "organization_id", "status", "title"}).
			AddRow(testUserID, nil, "ready", "My Video"))

	mock.ExpectQuery("SELECT role FROM organization_members").
		WithArgs(testTargetOrgID, testUserID).
		WillReturnError(pgx.ErrNoRows)

	body, _ := json.Marshal(transferRequest{OrganizationID: strPtr(testTargetOrgID)})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/transfer", handler.Transfer)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/vid-1/transfer", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTransfer_ViewerOfTargetBlocked(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newTransferHandler(mock)

	mock.ExpectQuery("SELECT user_id, organization_id, status, title FROM videos").
		WithArgs("vid-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "organization_id", "status", "title"}).
			AddRow(testUserID, nil, "ready", "My Video"))

	mock.ExpectQuery("SELECT role FROM organization_members").
		WithArgs(testTargetOrgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("viewer"))

	body, _ := json.Marshal(transferRequest{OrganizationID: strPtr(testTargetOrgID)})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/transfer", handler.Transfer)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/vid-1/transfer", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTransfer_ProcessingVideoBlocked(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newTransferHandler(mock)

	mock.ExpectQuery("SELECT user_id, organization_id, status, title FROM videos").
		WithArgs("vid-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "organization_id", "status", "title"}).
			AddRow(testUserID, nil, "processing", "Processing Video"))

	body, _ := json.Marshal(transferRequest{OrganizationID: strPtr(testTargetOrgID)})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/transfer", handler.Transfer)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/vid-1/transfer", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTransfer_ClearsFolderAndTags(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newTransferHandler(mock)

	mock.ExpectQuery("SELECT user_id, organization_id, status, title FROM videos").
		WithArgs("vid-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "organization_id", "status", "title"}).
			AddRow(testUserID, nil, "ready", "Tagged Video"))

	mock.ExpectQuery("SELECT role FROM organization_members").
		WithArgs(testTargetOrgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("member"))

	mock.ExpectExec("UPDATE videos SET organization_id").
		WithArgs(strPtr(testTargetOrgID), "vid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec("DELETE FROM video_tags WHERE video_id").
		WithArgs("vid-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 3))

	body, _ := json.Marshal(transferRequest{OrganizationID: strPtr(testTargetOrgID)})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/transfer", handler.Transfer)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/vid-1/transfer", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTransfer_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newTransferHandler(mock)

	mock.ExpectQuery("SELECT user_id, organization_id, status, title FROM videos").
		WithArgs("vid-nonexistent").
		WillReturnError(pgx.ErrNoRows)

	body, _ := json.Marshal(transferRequest{OrganizationID: strPtr(testTargetOrgID)})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/transfer", handler.Transfer)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/vid-nonexistent/transfer", body))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTransfer_AlreadyInTargetScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newTransferHandler(mock)

	mock.ExpectQuery("SELECT user_id, organization_id, status, title FROM videos").
		WithArgs("vid-1").
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "organization_id", "status", "title"}).
			AddRow(testUserID, strPtr(testOrgID), "ready", "Already There"))

	mock.ExpectQuery("SELECT role FROM organization_members").
		WithArgs(testOrgID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("member"))

	body, _ := json.Marshal(transferRequest{OrganizationID: strPtr(testOrgID)})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/transfer", handler.Transfer)

	rec := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodPost, "/api/videos/vid-1/transfer", body)
	ctx := auth.ContextWithOrg(req.Context(), testOrgID, "member")
	r.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
