package video

import (
	"context"
	"log/slog"
	"time"

	"github.com/sendrec/sendrec/internal/database"
)

// stuckProcessingAfter is how long a video may sit in status='processing'
// before the owning job is presumed dead. Composite, trim and remove-segments
// all run under a 10 minute context, so anything past this bound belongs to a
// process that can no longer finish it.
const stuckProcessingAfter = "15 minutes"

// resetStuckProcessing does what the in-process setReadyFallback would have
// done for rows whose owner never got the chance.
//
// Every editing job flips the row to 'processing', works, then flips it back on
// success or via setReadyFallback on error. That covers Go error returns and
// nothing else: a SIGKILL, an OOM, an evicted pod or a node drain skips the
// fallback entirely and strands the row. Nothing else looks at 'processing'
// rows — the transcode and normalize workers both filter on 'ready' — so
// without this sweep the video shows the processing overlay forever and both
// edit endpoints reject it with 409.
//
// Resetting is safe because all three jobs upload only after ffmpeg succeeds:
// a row abandoned mid-job still has its original object at file_key and its
// original duration. The row is the only thing that is wrong.
//
// The deadline is keyed on processing_started_at rather than updated_at because
// updated_at moves for unrelated writes (a title edit, say), which would push
// the deadline out indefinitely on exactly the rows that need sweeping. A NULL
// processing_started_at means the row was stranded before this column existed,
// so those are swept on the first pass.
//
// webcam_key is cleared to match composite's own fallback: an abandoned overlay
// leaves a webcam object that will never be composited, and leaving the key set
// would keep the watch page waiting on it.
func resetStuckProcessing(ctx context.Context, db database.DBTX) {
	tag, err := db.Exec(ctx,
		`UPDATE videos SET status = 'ready', webcam_key = NULL, processing_started_at = NULL, updated_at = now()
		 WHERE status = 'processing'
		   AND (processing_started_at < now() - INTERVAL '`+stuckProcessingAfter+`' OR processing_started_at IS NULL)`,
	)
	if err != nil {
		slog.Error("stuck-processing: failed to reset abandoned jobs", "error", err)
		return
	}
	if n := tag.RowsAffected(); n > 0 {
		slog.Warn("stuck-processing: reset abandoned jobs to ready", "count", n, "older_than", stuckProcessingAfter)
	}
}

// StartStuckProcessingWorker sweeps once at startup — the pod that died is
// usually the pod that comes back — and then on every tick, which also covers
// node-level failures where a different pod inherits the work.
func StartStuckProcessingWorker(ctx context.Context, db database.DBTX, interval time.Duration) {
	go func() {
		resetStuckProcessing(ctx, db)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("stuck-processing: shutting down")
				return
			case <-ticker.C:
				resetStuckProcessing(ctx, db)
			}
		}
	}()
}
