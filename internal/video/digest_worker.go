package video

import (
	"context"
	"log/slog"
	"time"

	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/email"
)

const digestHourUTC = 9

func durationUntilNextRun(now time.Time) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day(), digestHourUTC, 0, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now)
}

type userDigest struct {
	email  string
	name   string
	videos []email.DigestVideoSummary
}

func processDigest(ctx context.Context, db database.DBTX, notifier ViewNotifier, baseURL string) {
	rows, err := db.Query(ctx,
		`WITH recent_views AS (
		     SELECT video_id, COUNT(*) AS view_count
		     FROM video_views
		     WHERE created_at >= NOW() - INTERVAL '24 hours'
		     GROUP BY video_id
		 ),
		 recent_comments AS (
		     SELECT video_id, COUNT(*) AS comment_count
		     FROM video_comments
		     WHERE created_at >= NOW() - INTERVAL '24 hours'
		       AND is_private = false
		     GROUP BY video_id
		 )
		 SELECT v.id, v.title, v.share_token, v.user_id, u.email, u.name,
		        CASE
		          WHEN COALESCE(v.view_notification, 'digest') = 'digest' THEN COALESCE(rv.view_count, 0)
		          ELSE 0
		        END AS view_count,
		        COALESCE(rc.comment_count, 0) AS comment_count
		 FROM videos v
		 JOIN users u ON u.id = v.user_id
		 LEFT JOIN notification_preferences np ON np.user_id = v.user_id
		 LEFT JOIN recent_views rv ON rv.video_id = v.id
		 LEFT JOIN recent_comments rc ON rc.video_id = v.id
		 WHERE v.status != 'deleted'
		   AND COALESCE(np.view_notification, 'off') = 'digest'
		   AND (
		     CASE
		       WHEN COALESCE(v.view_notification, 'digest') = 'digest' THEN COALESCE(rv.view_count, 0)
		       ELSE 0
		     END > 0
		     OR COALESCE(rc.comment_count, 0) > 0
		   )
		 ORDER BY v.user_id, view_count DESC, comment_count DESC`)
	if err != nil {
		slog.Error("digest-worker: query failed", "error", err)
		return
	}
	defer rows.Close()

	digests := make(map[string]*userDigest)
	for rows.Next() {
		var videoID, title, shareToken, userID, ownerEmail, name string
		var viewCount, commentCount int64
		if err := rows.Scan(&videoID, &title, &shareToken, &userID, &ownerEmail, &name, &viewCount, &commentCount); err != nil {
			slog.Error("digest-worker: scan failed", "error", err)
			continue
		}
		d, ok := digests[userID]
		if !ok {
			d = &userDigest{email: ownerEmail, name: name}
			digests[userID] = d
		}
		d.videos = append(d.videos, email.DigestVideoSummary{
			Title:        title,
			ViewCount:    int(viewCount),
			CommentCount: int(commentCount),
			WatchURL:     baseURL + "/watch/" + shareToken,
		})
	}
	if err := rows.Err(); err != nil {
		slog.Error("digest-worker: row iteration error", "error", err)
	}

	sent := 0
	totalVideos := 0
	for userID, d := range digests {
		if err := notifier.SendDigestNotification(ctx, d.email, d.name, d.videos); err != nil {
			slog.Error("digest-worker: failed to send digest", "user_id", userID, "error", err)
			continue
		}
		sent++
		totalVideos += len(d.videos)
	}
	if sent > 0 {
		slog.Info("digest-worker: sent digest emails", "emails", sent, "videos", totalVideos)
	}
}

func StartDigestWorker(ctx context.Context, db database.DBTX, notifier ViewNotifier, baseURL string) {
	if notifier == nil {
		return
	}
	go func() {
		slog.Info("digest-worker: started")
		for {
			d := durationUntilNextRun(time.Now().UTC())
			slog.Info("digest-worker: next run scheduled", "duration", d)
			select {
			case <-ctx.Done():
				slog.Info("digest-worker: shutting down")
				return
			case <-time.After(d):
				processDigest(ctx, db, notifier, baseURL)
			}
		}
	}()
}
