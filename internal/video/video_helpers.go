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
