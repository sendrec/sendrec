package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/server"
)

// Workspace scoping on the playlist routes, driven through the real router the
// way the browser does. The middleware gap this guards against reached
// production twice — once for branding, once here — because handler tests
// injected the workspace context that was never actually arriving.
func TestPlaylistRouteDB_WorkspaceScopeEndToEnd(t *testing.T) {
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
	for _, stmt := range []string{`DELETE FROM playlist_videos`, `DELETE FROM playlists`, `DELETE FROM videos`, `DELETE FROM user_branding`} {
		if _, err := db.Pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("reset: %v", err)
		}
	}

	scan := func(label, stmt string, dest *string, args ...any) {
		t.Helper()
		if err := db.Pool.QueryRow(ctx, stmt, args...).Scan(dest); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
	}
	exec := func(label, stmt string, args ...any) {
		t.Helper()
		if _, err := db.Pool.Exec(ctx, stmt, args...); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
	}

	var ownerID, viewerID, orgID, videoID string
	scan("seed owner", `INSERT INTO users (email, password, name) VALUES ('fin-owner@example.com','x','Owner') ON CONFLICT (email) DO UPDATE SET name='Owner' RETURNING id`, &ownerID)
	scan("seed viewer", `INSERT INTO users (email, password, name) VALUES ('fin-viewer@example.com','x','Viewer') ON CONFLICT (email) DO UPDATE SET name='Viewer' RETURNING id`, &viewerID)
	scan("seed org", `INSERT INTO organizations (name, slug) VALUES ('Acme','acme-final') ON CONFLICT (slug) DO UPDATE SET name='Acme' RETURNING id`, &orgID)
	exec("seed owner membership", `INSERT INTO organization_members (organization_id,user_id,role) VALUES ($1,$2,'owner') ON CONFLICT (organization_id,user_id) DO UPDATE SET role='owner'`, orgID, ownerID)
	exec("seed viewer membership", `INSERT INTO organization_members (organization_id,user_id,role) VALUES ($1,$2,'viewer') ON CONFLICT (organization_id,user_id) DO UPDATE SET role='viewer'`, orgID, viewerID)
	scan("seed video", `INSERT INTO videos (user_id, organization_id, title, file_key, share_token, status) VALUES ($1,$2,'Clip','k','fin-vid','ready') RETURNING id`, &videoID, ownerID, orgID)

	srv := server.New(server.Config{DB: db.Pool, Pinger: &mockPinger{}, Storage: &mockStorage{}, JWTSecret: "s", BaseURL: "https://x", BrandingEnabled: true})
	ownerTok, _ := auth.GenerateAccessToken("s", ownerID)
	viewerTok, _ := auth.GenerateAccessToken("s", viewerID)

	do := func(method, path, body, tok, org string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		if org != "" {
			req.Header.Set("X-Organization-Id", org)
		}
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}

	// 1. workspace playlist lands in workspace scope
	rec := do(http.MethodPost, "/api/playlists", `{"title":"WS"}`, ownerTok, orgID)
	if rec.Code != http.StatusCreated {
		t.Fatalf("workspace create: %d %s", rec.Code, rec.Body.String())
	}
	var created struct{ ID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created playlist: %v", err)
	}
	var plOrg *string
	scanOrg := func(id string) *string {
		t.Helper()
		var got *string
		if err := db.Pool.QueryRow(ctx, `SELECT organization_id FROM playlists WHERE id=$1`, id).Scan(&got); err != nil {
			t.Fatalf("read playlist scope: %v", err)
		}
		return got
	}
	plOrg = scanOrg(created.ID)
	if plOrg == nil || *plOrg != orgID {
		t.Errorf("workspace playlist stored outside the workspace: %v", plOrg)
	}

	// 2. a workspace video can join it — the reviewer's 404
	rec = do(http.MethodPost, "/api/playlists/"+created.ID+"/videos", `{"videoIds":["`+videoID+`"]}`, ownerTok, orgID)
	if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Errorf("adding a workspace video: %d %s", rec.Code, rec.Body.String())
	}

	// 3. viewer refused
	if rec := do(http.MethodPost, "/api/playlists", `{"title":"V"}`, viewerTok, orgID); rec.Code != http.StatusForbidden {
		t.Errorf("viewer create: expected 403, got %d %s", rec.Code, rec.Body.String())
	}

	// 4. no header stays personal
	rec = do(http.MethodPost, "/api/playlists", `{"title":"Personal"}`, ownerTok, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("personal create: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode personal playlist: %v", err)
	}
	plOrg = scanOrg(created.ID)
	if plOrg != nil {
		t.Errorf("personal playlist picked up a workspace: %v", *plOrg)
	}

	// 5. stale workspace id is refused rather than silently personal
	if rec := do(http.MethodGet, "/api/settings/branding", "", ownerTok, "00000000-0000-0000-0000-000000000000"); rec.Code != http.StatusForbidden {
		t.Errorf("stale org header: expected 403, got %d %s", rec.Code, rec.Body.String())
	}
}
