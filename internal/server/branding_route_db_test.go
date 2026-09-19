package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/server"
)

// The reported failure in #224 was a routing gap, not a SQL one: the handler and
// its statements were correct, but no real request ever carried the workspace
// context. This walks the reporter's steps end to end — real router, real
// database, the header the browser sends — and checks the row that lands.
func TestBrandingRouteDB_WorkspaceHeaderWritesOrgScopedRow(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	db, err := database.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(db.Close)

	if _, err := db.Pool.Exec(ctx, `DELETE FROM user_branding`); err != nil {
		t.Fatalf("reset branding: %v", err)
	}

	var userID, orgID string
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO users (email, password, name) VALUES ('route-branding@example.com', 'x', 'Owner')
		 ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme-route-branding')
		 ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
	).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'owner')
		 ON CONFLICT (organization_id, user_id) DO UPDATE SET role = EXCLUDED.role`, orgID, userID,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	srv := server.New(server.Config{
		DB:              db.Pool,
		Pinger:          &mockPinger{err: nil},
		Storage:         &mockStorage{},
		JWTSecret:       "test-secret",
		BaseURL:         "https://localhost:8080",
		BrandingEnabled: true,
	})
	token, err := auth.GenerateAccessToken("test-secret", userID)
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/settings/branding",
		strings.NewReader(`{"companyName":"Acme Inc","colorAccent":"#ff0000"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-Id", orgID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("save: expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Step 3 of the report: the row must carry organization_id, not user_id.
	var gotUserID, gotOrgID *string
	var company *string
	if err := db.Pool.QueryRow(ctx,
		`SELECT user_id, organization_id, company_name FROM user_branding`,
	).Scan(&gotUserID, &gotOrgID, &company); err != nil {
		t.Fatalf("read back branding row: %v", err)
	}
	if gotOrgID == nil || *gotOrgID != orgID {
		t.Errorf("branding saved outside the workspace: user_id=%v organization_id=%v", gotUserID, gotOrgID)
	}
	if gotUserID != nil {
		t.Errorf("workspace branding row also carries a user_id: %v", *gotUserID)
	}
	if company == nil || *company != "Acme Inc" {
		t.Errorf("company name not stored: %v", company)
	}
}
