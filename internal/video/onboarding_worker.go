package video

import (
	"context"
	"log/slog"
	"time"

	"github.com/sendrec/sendrec/internal/database"
)

// OnboardingSender sends onboarding emails.
type OnboardingSender interface {
	SendOnboardingDay2(ctx context.Context, toEmail, toName, dashboardURL string) error
	SendOnboardingDay7(ctx context.Context, toEmail, toName, dashboardURL string) error
}

func processOnboardingDay2(ctx context.Context, db database.DBTX, sender OnboardingSender, baseURL string) {
	rows, err := db.Query(ctx,
		`SELECT id, email, name FROM users
		 WHERE email_verified = true
		   AND created_at < now() - interval '2 days'
		   AND onboarding_day2_sent_at IS NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM video_views WHERE video_id IN (
		       SELECT id FROM videos WHERE user_id = users.id
		     )
		   )
		 LIMIT 50`)
	if err != nil {
		slog.Error("onboarding-worker: day2 query failed", "error", err)
		return
	}
	defer rows.Close()

	sent := 0
	for rows.Next() {
		var userID, userEmail, name string
		if err := rows.Scan(&userID, &userEmail, &name); err != nil {
			slog.Error("onboarding-worker: day2 scan failed", "error", err)
			continue
		}
		if err := sender.SendOnboardingDay2(ctx, userEmail, name, baseURL); err != nil {
			slog.Error("onboarding-worker: day2 send failed", "user_id", userID, "error", err)
			continue
		}
		if _, err := db.Exec(ctx,
			"UPDATE users SET onboarding_day2_sent_at = now() WHERE id = $1", userID,
		); err != nil {
			slog.Error("onboarding-worker: day2 mark sent failed", "user_id", userID, "error", err)
		}
		sent++
	}
	if err := rows.Err(); err != nil {
		slog.Error("onboarding-worker: day2 row iteration error", "error", err)
	}
	if sent > 0 {
		slog.Info("onboarding-worker: sent day 2 emails", "count", sent)
	}
}

func processOnboardingDay7(ctx context.Context, db database.DBTX, sender OnboardingSender, baseURL string) {
	rows, err := db.Query(ctx,
		`SELECT id, email, name FROM users
		 WHERE email_verified = true
		   AND created_at < now() - interval '7 days'
		   AND onboarding_day7_sent_at IS NULL
		   AND subscription_plan = 'free'
		 LIMIT 50`)
	if err != nil {
		slog.Error("onboarding-worker: day7 query failed", "error", err)
		return
	}
	defer rows.Close()

	sent := 0
	for rows.Next() {
		var userID, userEmail, name string
		if err := rows.Scan(&userID, &userEmail, &name); err != nil {
			slog.Error("onboarding-worker: day7 scan failed", "error", err)
			continue
		}
		if err := sender.SendOnboardingDay7(ctx, userEmail, name, baseURL); err != nil {
			slog.Error("onboarding-worker: day7 send failed", "user_id", userID, "error", err)
			continue
		}
		if _, err := db.Exec(ctx,
			"UPDATE users SET onboarding_day7_sent_at = now() WHERE id = $1", userID,
		); err != nil {
			slog.Error("onboarding-worker: day7 mark sent failed", "user_id", userID, "error", err)
		}
		sent++
	}
	if err := rows.Err(); err != nil {
		slog.Error("onboarding-worker: day7 row iteration error", "error", err)
	}
	if sent > 0 {
		slog.Info("onboarding-worker: sent day 7 emails", "count", sent)
	}
}

// StartOnboardingWorker runs the onboarding email worker on an hourly ticker.
func StartOnboardingWorker(ctx context.Context, db database.DBTX, sender OnboardingSender, baseURL string) {
	if sender == nil {
		return
	}
	go func() {
		slog.Info("onboarding-worker: started")
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("onboarding-worker: shutting down")
				return
			case <-ticker.C:
				processOnboardingDay2(ctx, db, sender, baseURL)
				processOnboardingDay7(ctx, db, sender, baseURL)
			}
		}
	}()
}
