package video

import (
	"context"
	"log"
	"time"

	"github.com/sendrec/sendrec/internal/database"
)

const digestHourUTC = 9

func durationUntilNextRun(now time.Time) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day(), digestHourUTC, 0, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now)
}

func processDigest(ctx context.Context, db database.DBTX, notifier ViewNotifier, baseURL string) {
	rows, err := db.Query(ctx,
		`SELECT v.id, v.title, v.share_token, v.user_id, u.email, u.name,
		        COUNT(vv.id) as view_count
		 FROM video_views vv
		 JOIN videos v ON v.id = vv.video_id
		 JOIN users u ON u.id = v.user_id
		 LEFT JOIN notification_preferences np ON np.user_id = v.user_id
		 WHERE vv.created_at >= NOW() - INTERVAL '24 hours'
		   AND COALESCE(v.view_notification, np.view_notification, 'off') = 'digest'
		   AND v.status != 'deleted'
		 GROUP BY v.id, v.title, v.share_token, v.user_id, u.email, u.name`)
	if err != nil {
		log.Printf("digest-worker: query failed: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var videoID, title, shareToken, userID, email, name string
		var viewCount int64
		if err := rows.Scan(&videoID, &title, &shareToken, &userID, &email, &name, &viewCount); err != nil {
			log.Printf("digest-worker: scan failed: %v", err)
			continue
		}
		watchURL := baseURL + "/watch/" + shareToken
		if err := notifier.SendViewNotification(ctx, email, name, title, watchURL, int(viewCount), true); err != nil {
			log.Printf("digest-worker: failed to send digest for %s: %v", videoID, err)
		}
	}
}

func StartDigestWorker(ctx context.Context, db database.DBTX, notifier ViewNotifier, baseURL string) {
	if notifier == nil {
		return
	}
	go func() {
		log.Println("digest-worker: started")
		for {
			d := durationUntilNextRun(time.Now().UTC())
			log.Printf("digest-worker: next run in %v", d)
			select {
			case <-ctx.Done():
				log.Println("digest-worker: shutting down")
				return
			case <-time.After(d):
				processDigest(ctx, db, notifier, baseURL)
			}
		}
	}()
}
