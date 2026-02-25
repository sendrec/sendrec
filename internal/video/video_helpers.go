package video

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sendrec/sendrec/internal/webhook"
)

const defaultPageSize = 50
const maxPageSize = 100

type analyticsSummary struct {
	TotalViews        int64   `json:"totalViews"`
	UniqueViews       int64   `json:"uniqueViews"`
	ViewsToday        int64   `json:"viewsToday"`
	AverageDailyViews float64 `json:"averageDailyViews"`
	PeakDay           string  `json:"peakDay"`
	PeakDayViews      int64   `json:"peakDayViews"`
	TotalCtaClicks    int64   `json:"totalCtaClicks"`
	CtaClickRate      float64 `json:"ctaClickRate"`
}

type dailyViews struct {
	Date        string `json:"date"`
	Views       int64  `json:"views"`
	UniqueViews int64  `json:"uniqueViews"`
}

type milestoneCounts struct {
	Reached25  int64 `json:"reached25"`
	Reached50  int64 `json:"reached50"`
	Reached75  int64 `json:"reached75"`
	Reached100 int64 `json:"reached100"`
}

type viewerInfo struct {
	Email         string `json:"email"`
	FirstViewedAt string `json:"firstViewedAt"`
	ViewCount     int64  `json:"viewCount"`
	Completion    int    `json:"completion"`
}

type segmentData struct {
	Segment    int     `json:"segment"`
	WatchCount int64   `json:"watchCount"`
	Intensity  float64 `json:"intensity"`
}

type analyticsResponse struct {
	Summary    analyticsSummary `json:"summary"`
	Daily      []dailyViews     `json:"daily"`
	Milestones milestoneCounts  `json:"milestones"`
	Viewers    []viewerInfo     `json:"viewers"`
	Heatmap    []segmentData    `json:"heatmap"`
}

func (h *Handler) dispatchWebhook(userID string, event webhook.Event) {
	if h.webhookClient == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		webhookURL, secret, err := h.webhookClient.LookupConfigByUserID(ctx, userID)
		if err != nil {
			return
		}
		if err := h.webhookClient.Dispatch(ctx, userID, webhookURL, secret, event); err != nil {
			slog.Error("webhook: dispatch failed", "user_id", userID, "event", event.Name, "error", err)
		}
	}()
}

func deleteWithRetry(ctx context.Context, storage ObjectStorage, key string, maxAttempts int) error {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
		lastErr = storage.DeleteObject(ctx, key)
		if lastErr == nil {
			return nil
		}
		slog.Error("storage: delete attempt failed", "attempt", attempt+1, "max_attempts", maxAttempts, "key", key, "error", lastErr)
	}
	return fmt.Errorf("all %d delete attempts failed for %s: %w", maxAttempts, key, lastErr)
}

func viewerHash(ip, userAgent string) string {
	h := sha256.Sum256([]byte(ip + "|" + userAgent))
	return fmt.Sprintf("%x", h[:8])
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if first, _, ok := strings.Cut(forwarded, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(forwarded)
	}
	return r.RemoteAddr
}
