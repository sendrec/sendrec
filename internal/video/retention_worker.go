package video

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/email"
)

// RetentionSender sends retention warning emails.
type RetentionSender interface {
	SendRetentionWarning(ctx context.Context, toEmail string, videos []email.RetentionVideoSummary, expiryDate string) error
}

func processRetentionWarnings(ctx context.Context, db database.DBTX, sender RetentionSender, baseURL string) {
	rows, err := db.Query(ctx,
		`SELECT v.id, v.title, v.share_id, u.email,
		        COALESCE(o.retention_days, u.retention_days) AS retention_days
		 FROM videos v
		 JOIN users u ON u.id = v.user_id
		 LEFT JOIN organizations o ON o.id = v.organization_id
		 WHERE v.status = 'ready'
		   AND v.pinned = false
		   AND v.retention_warned_at IS NULL
		   AND COALESCE(o.retention_days, u.retention_days) > 0
		   AND v.created_at < now() - make_interval(days => COALESCE(o.retention_days, u.retention_days) - 7)
		 LIMIT 100`)
	if err != nil {
		slog.Error("retention-worker: warnings query failed", "error", err)
		return
	}
	defer rows.Close()

	type videoEntry struct {
		id       string
		title    string
		shareID  string
		days     int
	}

	grouped := make(map[string][]videoEntry)
	retentionDays := make(map[string]int)

	for rows.Next() {
		var id, title, shareID, userEmail string
		var days int
		if err := rows.Scan(&id, &title, &shareID, &userEmail, &days); err != nil {
			slog.Error("retention-worker: warnings scan failed", "error", err)
			continue
		}
		grouped[userEmail] = append(grouped[userEmail], videoEntry{
			id:      id,
			title:   title,
			shareID: shareID,
			days:    days,
		})
		retentionDays[userEmail] = days
	}
	if err := rows.Err(); err != nil {
		slog.Error("retention-worker: warnings row iteration error", "error", err)
	}

	warned := 0
	for userEmail, entries := range grouped {
		videos := make([]email.RetentionVideoSummary, len(entries))
		var videoIDs []string
		for i, e := range entries {
			videos[i] = email.RetentionVideoSummary{
				Title:    e.title,
				WatchURL: fmt.Sprintf("%s/watch/%s", baseURL, e.shareID),
			}
			videoIDs = append(videoIDs, e.id)
		}

		expiryDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")

		if err := sender.SendRetentionWarning(ctx, userEmail, videos, expiryDate); err != nil {
			slog.Error("retention-worker: send warning failed", "email", userEmail, "error", err)
			continue
		}

		if _, err := db.Exec(ctx,
			"UPDATE videos SET retention_warned_at = now() WHERE id = ANY($1)",
			videoIDs,
		); err != nil {
			slog.Error("retention-worker: mark warned failed", "email", userEmail, "error", err)
		}
		warned += len(entries)
	}

	if warned > 0 {
		slog.Info("retention-worker: sent warning emails", "videos_warned", warned)
	}
}

func processRetentionDeletions(ctx context.Context, db database.DBTX) {
	rows, err := db.Query(ctx,
		`SELECT id FROM videos
		 WHERE retention_warned_at IS NOT NULL
		   AND retention_warned_at < now() - interval '7 days'
		   AND status = 'ready'
		   AND pinned = false
		 LIMIT 100`)
	if err != nil {
		slog.Error("retention-worker: deletions query failed", "error", err)
		return
	}
	defer rows.Close()

	var videoIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			slog.Error("retention-worker: deletions scan failed", "error", err)
			continue
		}
		videoIDs = append(videoIDs, id)
	}
	if err := rows.Err(); err != nil {
		slog.Error("retention-worker: deletions row iteration error", "error", err)
	}

	if len(videoIDs) == 0 {
		return
	}

	if _, err := db.Exec(ctx,
		"DELETE FROM playlist_videos WHERE video_id = ANY($1)",
		videoIDs,
	); err != nil {
		slog.Error("retention-worker: remove from playlists failed", "error", err)
	}

	result, err := db.Exec(ctx,
		"UPDATE videos SET status = 'deleted' WHERE id = ANY($1)",
		videoIDs,
	)
	if err != nil {
		slog.Error("retention-worker: soft delete failed", "error", err)
		return
	}

	slog.Info("retention-worker: soft-deleted expired videos", "count", result.RowsAffected())
}

// StartRetentionWorker runs the data retention worker on a daily ticker.
func StartRetentionWorker(ctx context.Context, db database.DBTX, sender RetentionSender, baseURL string) {
	if sender == nil {
		return
	}
	go func() {
		slog.Info("retention-worker: started")

		processRetentionWarnings(ctx, db, sender, baseURL)
		processRetentionDeletions(ctx, db)

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("retention-worker: shutting down")
				return
			case <-ticker.C:
				processRetentionWarnings(ctx, db, sender, baseURL)
				processRetentionDeletions(ctx, db)
			}
		}
	}()
}
