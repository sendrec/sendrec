package video

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
)

// Account deletion crosses every table that points at a user, a workspace or
// a video, and several of those references do not cascade. Only the real
// schema can show what blocks it, so these run against TEST_DATABASE_URL.

type fakeCanceler struct {
	canceled []string
	err      error
}

func (f *fakeCanceler) CancelSubscription(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	f.canceled = append(f.canceled, id)
	return nil
}

// accountDB gives each test a database of its own. CI runs every package's
// tests at once against one Postgres, and other suites reset by deleting
// every video, which these tests would trip over, and the reverse.
func accountDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	dbName := "account_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		admin.Close()
		t.Fatalf("create database: %v", err)
	}

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	u.Path = "/" + dbName
	db, err := database.Connect(ctx, u.String())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.Migrate(u.String()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		if _, err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)"); err != nil {
			t.Logf("drop database %s: %v", dbName, err)
		}
		admin.Close()
	})
	return db.Pool
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func mustID(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return id
}

func exists(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) bool {
	t.Helper()
	var ok bool
	if err := pool.QueryRow(context.Background(), "SELECT EXISTS ("+sql+")", args...).Scan(&ok); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return ok
}

func deleteAccountAs(h *Handler, userID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, "/api/user", nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	h.DeleteAccount(rec, req)
	return rec
}

func seedVideo(t *testing.T, pool *pgxpool.Pool, userID string, orgID *string, prefix string) string {
	t.Helper()
	return mustID(t, pool,
		`INSERT INTO videos (user_id, organization_id, title, file_key, share_token, status,
		                     thumbnail_key, transcript_key, webcam_key, branding_logo_key)
		 VALUES ($1, $2, 'v', $3 || '/file.mp4', $3 || '-tok', 'ready',
		         $3 || '/thumb.jpg', $3 || '/transcript.vtt', $3 || '/webcam.webm', $3 || '/logo.png')
		 RETURNING id`, userID, orgID, prefix)
}

// uniqueName keys every seeded row to this run, so reruns against the same
// database never collide.
func uniqueName(t *testing.T) string {
	return strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-")) + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

func drainDeletes(s *mockStorage) []string {
	close(s.deleteCalled)
	var keys []string
	for k := range s.deleteCalled {
		keys = append(keys, k)
	}
	return keys
}

func TestDeleteAccount_RemovesEverythingTheUserOwns(t *testing.T) {
	pool := accountDB(t)
	name := uniqueName(t)
	alice := mustID(t, pool, `INSERT INTO users (email, password, name, creem_subscription_id) VALUES ($1, 'x', 'Alice', 'sub_alice') RETURNING id`, name+"-alice@example.com")
	bob := mustID(t, pool, `INSERT INTO users (email, password, name) VALUES ($1, 'x', 'Bob') RETURNING id`, name+"-bob@example.com")

	// Alice's own workspace, nobody else in it, on a paid plan.
	solo := mustID(t, pool, `INSERT INTO organizations (name, slug, creem_subscription_id) VALUES ('Solo', $1, 'sub_solo') RETURNING id`, name+"-solo")
	mustExec(t, pool, `INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'owner')`, solo, alice)
	// Bob's workspace, which Alice belongs to and invited people to.
	shared := mustID(t, pool, `INSERT INTO organizations (name, slug) VALUES ('Shared', $1) RETURNING id`, name+"-shared")
	mustExec(t, pool, `INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'owner'), ($1, $3, 'admin')`, shared, bob, alice)
	mustExec(t, pool, `INSERT INTO organization_invites (organization_id, email, role, invited_by, token_hash, expires_at) VALUES ($1, 'x@example.com', 'member', $2, $3, now() + interval '1 day')`, shared, alice, name+"-invite")

	personal := seedVideo(t, pool, alice, nil, name+"/alice-personal")
	inSolo := seedVideo(t, pool, alice, &solo, name+"/alice-solo")
	inShared := seedVideo(t, pool, alice, &shared, name+"/alice-shared")
	bobs := seedVideo(t, pool, bob, &shared, name+"/bob")

	// The analytics rows whose references to videos do not cascade.
	for _, v := range []string{personal, inSolo, inShared} {
		mustExec(t, pool, `INSERT INTO video_views (video_id, viewer_hash) VALUES ($1, 'h')`, v)
		mustExec(t, pool, `INSERT INTO view_milestones (video_id, viewer_hash, milestone) VALUES ($1, 'h', 25)`, v)
		mustExec(t, pool, `INSERT INTO segment_engagement (video_id, segment_index) VALUES ($1, 0)`, v)
		mustExec(t, pool, `INSERT INTO cta_clicks (video_id, viewer_hash) VALUES ($1, 'h')`, v)
	}
	// Bob commented on Alice's video; Alice commented on Bob's.
	mustExec(t, pool, `INSERT INTO video_comments (video_id, user_id, body) VALUES ($1, $2, 'nice')`, personal, bob)
	mustExec(t, pool, `INSERT INTO video_comments (video_id, user_id, body) VALUES ($1, $2, 'thanks')`, bobs, alice)
	mustExec(t, pool, `INSERT INTO notification_preferences (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, alice)
	mustExec(t, pool, `INSERT INTO user_branding (user_id, logo_key) VALUES ($1, $2)`, alice, name+"/alice-brand.png")

	storage := &mockStorage{deleteCalled: make(chan string, 64)}
	canceler := &fakeCanceler{}
	h := NewHandler(pool, storage, "https://example.com", 0, 0, 0, 0, "secret", false)
	h.SetSubscriptionCanceler(canceler)

	rec := deleteAccountAs(h, alice)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if exists(t, pool, `SELECT 1 FROM users WHERE id = $1`, alice) {
		t.Error("the user still exists")
	}
	if exists(t, pool, `SELECT 1 FROM videos WHERE id = ANY($1)`, []string{personal, inSolo, inShared}) {
		t.Error("the user's videos still exist")
	}
	if exists(t, pool, `SELECT 1 FROM organizations WHERE id = $1`, solo) {
		t.Error("the user's own workspace still exists")
	}
	if !exists(t, pool, `SELECT 1 FROM organizations WHERE id = $1`, shared) {
		t.Error("the shared workspace should survive")
	}
	if !exists(t, pool, `SELECT 1 FROM videos WHERE id = $1`, bobs) {
		t.Error("another member's video in the shared workspace was deleted")
	}
	if !exists(t, pool, `SELECT 1 FROM video_comments WHERE video_id = $1 AND user_id IS NULL AND body = 'thanks'`, bobs) {
		t.Error("the user's comment on someone else's video should stay, anonymised")
	}

	slices.Sort(canceler.canceled)
	if !slices.Equal(canceler.canceled, []string{"sub_alice", "sub_solo"}) {
		t.Errorf("want both of the user's subscriptions canceled, got %v", canceler.canceled)
	}

	deleted := drainDeletes(storage)
	for _, prefix := range []string{name + "/alice-personal", name + "/alice-solo", name + "/alice-shared"} {
		for _, suffix := range []string{"/file.mp4", "/thumb.jpg", "/transcript.vtt", "/webcam.webm", "/logo.png"} {
			if !slices.Contains(deleted, prefix+suffix) {
				t.Errorf("object %s was not deleted", prefix+suffix)
			}
		}
	}
	if !slices.Contains(deleted, name+"/alice-brand.png") {
		t.Error("the user's branding logo was not deleted")
	}
	for _, k := range deleted {
		if strings.HasPrefix(k, name+"/bob") {
			t.Errorf("deleted another user's object %s", k)
		}
	}
}

// Deleting would take other people's work with it: they must hand the
// workspace over first, and nothing is touched until they do.
func TestDeleteAccount_RefusesWhileOwningASharedWorkspace(t *testing.T) {
	pool := accountDB(t)
	name := uniqueName(t)
	alice := mustID(t, pool, `INSERT INTO users (email, password, name) VALUES ($1, 'x', 'Alice') RETURNING id`, name+"-alice@example.com")
	bob := mustID(t, pool, `INSERT INTO users (email, password, name) VALUES ($1, 'x', 'Bob') RETURNING id`, name+"-bob@example.com")
	team := mustID(t, pool, `INSERT INTO organizations (name, slug) VALUES ('Team Rocket', $1) RETURNING id`, name+"-team")
	mustExec(t, pool, `INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'owner'), ($1, $3, 'member')`, team, alice, bob)
	video := seedVideo(t, pool, alice, nil, name+"/alice")

	storage := &mockStorage{deleteCalled: make(chan string, 64)}
	rec := deleteAccountAs(NewHandler(pool, storage, "https://example.com", 0, 0, 0, 0, "secret", false), alice)

	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "Team Rocket") {
		t.Fatalf("want 409 naming the workspace, got %d: %s", rec.Code, rec.Body.String())
	}
	if !exists(t, pool, `SELECT 1 FROM users WHERE id = $1`, alice) || !exists(t, pool, `SELECT 1 FROM videos WHERE id = $1`, video) {
		t.Error("something was deleted although the request was refused")
	}
	if keys := drainDeletes(storage); len(keys) != 0 {
		t.Errorf("objects were deleted although the request was refused: %v", keys)
	}
}

// A paying customer must not be charged after leaving, so a cancellation that
// fails stops the deletion before anything is removed.
func TestDeleteAccount_StopsWhenTheSubscriptionCannotBeCanceled(t *testing.T) {
	pool := accountDB(t)
	name := uniqueName(t)
	alice := mustID(t, pool, `INSERT INTO users (email, password, name, creem_subscription_id) VALUES ($1, 'x', 'Alice', 'sub_alice') RETURNING id`, name+"-alice@example.com")
	video := seedVideo(t, pool, alice, nil, name+"/alice")

	storage := &mockStorage{deleteCalled: make(chan string, 64)}
	h := NewHandler(pool, storage, "https://example.com", 0, 0, 0, 0, "secret", false)
	h.SetSubscriptionCanceler(&fakeCanceler{err: errors.New("creem down")})

	rec := deleteAccountAs(h, alice)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d: %s", rec.Code, rec.Body.String())
	}
	if !exists(t, pool, `SELECT 1 FROM users WHERE id = $1`, alice) || !exists(t, pool, `SELECT 1 FROM videos WHERE id = $1`, video) {
		t.Error("data was deleted although the subscription is still active")
	}
	if keys := drainDeletes(storage); len(keys) != 0 {
		t.Errorf("objects were deleted although the subscription is still active: %v", keys)
	}
}

// A stored file that will not delete must not be forgotten when the account's
// rows go: it is handed to the retired-objects sweep, which retries it.
func TestDeleteAccount_HandsUndeletedFilesToTheSweep(t *testing.T) {
	pool := accountDB(t)
	name := uniqueName(t)
	alice := mustID(t, pool, `INSERT INTO users (email, password, name) VALUES ($1, 'x', 'Alice') RETURNING id`, name+"-alice@example.com")
	seedVideo(t, pool, alice, nil, name+"/alice")

	storage := &mockStorage{deleteCalled: make(chan string, 64), deleteFailUntil: 1000, deleteErr: errors.New("storage down")}
	rec := deleteAccountAs(NewHandler(pool, storage, "https://example.com", 0, 0, 0, 0, "secret", false), alice)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", rec.Code, rec.Body.String())
	}
	drainDeletes(storage)

	for _, suffix := range []string{"/file.mp4", "/thumb.jpg", "/transcript.vtt", "/webcam.webm", "/logo.png"} {
		if !exists(t, pool, `SELECT 1 FROM retired_objects WHERE key = $1`, name+"/alice"+suffix) {
			t.Errorf("%s failed to delete and was not handed to the sweep", suffix)
		}
	}
}
