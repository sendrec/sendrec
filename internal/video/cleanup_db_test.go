package video

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sendrec/sendrec/internal/webhook"
)

// The webcam of a composite abandoned with its process is deleted, and the
// key cleared, once the sweep resets the row. #296.
func TestResetStuckProcessing_DB_DeletesAbandonedWebcam(t *testing.T) {
	pool := accountDB(t)
	name := uniqueName(t)
	user := mustID(t, pool, `INSERT INTO users (email, password, name) VALUES ($1, 'x', 'U') RETURNING id`, name+"@example.com")
	id := seedVideo(t, pool, user, nil, name)
	mustExec(t, pool, `UPDATE videos SET status = 'processing', processing_started_at = now() - interval '3 hours' WHERE id = $1`, id)

	storage := &mockStorage{deleteCalled: make(chan string, 4)}
	resetStuckProcessing(context.Background(), pool, storage, videoReadyHook{})

	if !exists(t, pool, `SELECT 1 FROM videos WHERE id = $1 AND status = 'ready' AND webcam_key IS NULL AND capture_warning = $2`, id, webcamDroppedWarning) {
		t.Error("want the row ready, warned and without its webcam key")
	}
	if got := drainDeletes(storage); !slices.Equal(got, []string{name + "/webcam.webm"}) {
		t.Errorf("want only the webcam deleted, got %v", got)
	}
}

// The sweep removes every object a deleted video references, webcam included,
// before marking it purged. #296.
func TestPurgeOrphanedFiles_DB_DeletesEveryObject(t *testing.T) {
	pool := accountDB(t)
	name := uniqueName(t)
	user := mustID(t, pool, `INSERT INTO users (email, password, name) VALUES ($1, 'x', 'U') RETURNING id`, name+"@example.com")
	id := seedVideo(t, pool, user, nil, name)
	mustExec(t, pool, `UPDATE videos SET status = 'deleted' WHERE id = $1`, id)

	storage := &mockStorage{deleteCalled: make(chan string, 8)}
	PurgeOrphanedFiles(context.Background(), pool, storage)

	if !exists(t, pool, `SELECT 1 FROM videos WHERE id = $1 AND file_purged_at IS NOT NULL`, id) {
		t.Error("want the row marked purged")
	}
	got := drainDeletes(storage)
	for _, want := range []string{"/file.mp4", "/thumb.jpg", "/transcript.vtt", "/webcam.webm"} {
		if !slices.Contains(got, name+want) {
			t.Errorf("want %s deleted, got %v", name+want, got)
		}
	}
}

// seedWebcamVideo returns a video in status with every object key set, the
// webcam at name/webcam.webm.
func seedWebcamVideo(t *testing.T, status string) (*pgxpool.Pool, string, string) {
	t.Helper()
	pool := accountDB(t)
	name := uniqueName(t)
	user := mustID(t, pool, `INSERT INTO users (email, password, name) VALUES ($1, 'x', 'U') RETURNING id`, name+"@example.com")
	id := seedVideo(t, pool, user, nil, name)
	mustExec(t, pool, `UPDATE videos SET status = $2, processing_started_at = now() - interval '3 hours', updated_at = now() - interval '1 day' WHERE id = $1`, id, status)
	return pool, id, name
}

// A row is purged only once every object it references is gone. Whichever
// delete fails, file_purged_at stays NULL so the next sweep tries again.
// #296.
func TestPurgeOrphanedFiles_DB_FailedDeleteLeavesRowUnpurged(t *testing.T) {
	for _, failing := range []string{"/file.mp4", "/thumb.jpg", "/transcript.vtt", "/webcam.webm"} {
		t.Run(failing[1:], func(t *testing.T) {
			t.Parallel()
			pool, id, name := seedWebcamVideo(t, "deleted")

			PurgeOrphanedFiles(context.Background(), pool, &mockStorage{deleteFailKey: name + failing})

			if exists(t, pool, `SELECT 1 FROM videos WHERE id = $1 AND file_purged_at IS NOT NULL`, id) {
				t.Errorf("marked purged although %s was not deleted", failing)
			}
			// Sent to the back of the queue, behind rows not yet tried.
			if !exists(t, pool, `SELECT 1 FROM videos WHERE id = $1 AND updated_at > now() - interval '1 hour'`, id) {
				t.Error("want the failed row requeued behind the others")
			}
		})
	}
}

// A webcam object that would not delete keeps its key, so deleting the video
// later still finds it: after a fallback, after an overlay that succeeded, and
// after the stuck-processing sweep. #296.
func TestDropWebcam_DB_FailedDeleteKeepsWebcamKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(pool *pgxpool.Pool, id, prefix string, storage *mockStorage)
	}{
		{"composite fallback", func(pool *pgxpool.Pool, id, prefix string, storage *mockStorage) {
			storage.downloadToFileErr = errors.New("s3 down")
			CompositeWithWebcam(context.Background(), pool, storage, id, prefix+"/file.mp4", prefix+"/webcam.webm", prefix+"/thumb.jpg", "video/mp4")
		}},
		{"composite success", func(pool *pgxpool.Pool, id, prefix string, storage *mockStorage) {
			CompositeWithWebcam(context.Background(), pool, storage, id, prefix+"/file.mp4", prefix+"/webcam.webm", prefix+"/thumb.jpg", "video/mp4")
		}},
		{"stuck-processing reset", func(pool *pgxpool.Pool, _, _ string, storage *mockStorage) {
			resetStuckProcessing(context.Background(), pool, storage, videoReadyHook{})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubCompositeTools(t)
			pool, id, prefix := seedWebcamVideo(t, "processing")

			tc.run(pool, id, prefix, &mockStorage{deleteFailKey: prefix + "/webcam.webm"})

			if !exists(t, pool, `SELECT 1 FROM videos WHERE id = $1 AND status = 'ready' AND webcam_key = $2`, id, prefix+"/webcam.webm") {
				t.Error("want the row ready and still pointing at the webcam it could not delete")
			}
		})
	}
}

// Migration 000068 hands every deleted video back to the sweep, including
// rows marked purged before #296 although some of their objects survived.
func TestRepurgeMigration_DB_ReturnsDeletedRowsToTheSweep(t *testing.T) {
	pool, id, name := seedWebcamVideo(t, "deleted")
	live := seedVideo(t, pool, mustID(t, pool, `SELECT user_id FROM videos WHERE id = $1`, id), nil, name+"-live")
	mustExec(t, pool, `UPDATE videos SET file_purged_at = now() WHERE id = ANY($1)`, []string{id, live})

	sql, err := os.ReadFile("../../migrations/000068_repurge_deleted_videos.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, pool, string(sql))

	if !exists(t, pool, `SELECT 1 FROM videos WHERE id = $1 AND file_purged_at IS NULL`, id) {
		t.Error("want the deleted row unmarked")
	}
	if !exists(t, pool, `SELECT 1 FROM videos WHERE id = $1 AND file_purged_at IS NOT NULL`, live) {
		t.Error("want rows that are not deleted left alone")
	}

	storage := &mockStorage{deleteCalled: make(chan string, 8)}
	PurgeOrphanedFiles(context.Background(), pool, storage)
	if got := drainDeletes(storage); len(got) != 4 {
		t.Errorf("want the sweep to delete all four objects again, got %v", got)
	}
	if !exists(t, pool, `SELECT 1 FROM videos WHERE id = $1 AND file_purged_at IS NOT NULL`, id) {
		t.Error("want the row purged again")
	}
}

// A composite abandoned with its process becomes watchable when the sweep
// publishes the screen recording, so the sweep sends video.ready for it. An
// edit reset the same way was watchable all along and sends nothing. BG-13.
func TestResetStuckProcessing_DB_SendsVideoReadyForComposites(t *testing.T) {
	pool, composite, name := seedWebcamVideo(t, "processing")
	user := mustID(t, pool, `SELECT user_id FROM videos WHERE id = $1`, composite)
	trim := seedVideo(t, pool, user, nil, name+"-trim")
	mustExec(t, pool, `UPDATE videos SET status = 'processing', webcam_key = NULL, processing_started_at = now() - interval '3 hours' WHERE id = $1`, trim)
	// Dispatch to this host fails, but every attempt is logged by event name.
	mustExec(t, pool, `INSERT INTO notification_preferences (user_id, webhook_url, webhook_secret) VALUES ($1, 'https://hooks.invalid/x', 's') ON CONFLICT (user_id) DO UPDATE SET webhook_url = EXCLUDED.webhook_url, webhook_secret = EXCLUDED.webhook_secret`, user)

	resetStuckProcessing(context.Background(), pool, &mockStorage{}, videoReadyHook{webhook.New(pool), "https://app.example"})

	deadline := time.Now().Add(10 * time.Second)
	for !exists(t, pool, `SELECT 1 FROM webhook_deliveries WHERE user_id = $1 AND event = 'video.ready' AND payload->'data'->>'videoId' = $2`, user, composite) {
		if time.Now().After(deadline) {
			t.Fatal("want video.ready for the reset composite")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if exists(t, pool, `SELECT 1 FROM webhook_deliveries WHERE payload->'data'->>'videoId' = $1`, trim) {
		t.Error("video.ready sent for an edit that was already watchable")
	}
}
