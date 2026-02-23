package notify

import (
	"context"
	"log/slog"

	"github.com/sendrec/sendrec/internal/email"
	"github.com/sendrec/sendrec/internal/video"
)

var (
	_ video.ViewNotifier    = (*MultiViewNotifier)(nil)
	_ video.CommentNotifier = (*MultiCommentNotifier)(nil)
)

// MultiViewNotifier fans out view notifications to all registered notifiers.
type MultiViewNotifier struct {
	notifiers []video.ViewNotifier
}

// NewMultiViewNotifier creates a notifier that delegates to all provided view notifiers.
func NewMultiViewNotifier(notifiers ...video.ViewNotifier) *MultiViewNotifier {
	return &MultiViewNotifier{notifiers: notifiers}
}

func (m *MultiViewNotifier) SendViewNotification(ctx context.Context, toEmail, toName, videoTitle, watchURL string, viewCount int) error {
	for _, n := range m.notifiers {
		if err := n.SendViewNotification(ctx, toEmail, toName, videoTitle, watchURL, viewCount); err != nil {
			slog.Error("multi-notifier: view notification failed", "error", err)
		}
	}
	return nil
}

func (m *MultiViewNotifier) SendDigestNotification(ctx context.Context, toEmail, toName string, videos []email.DigestVideoSummary) error {
	for _, n := range m.notifiers {
		if err := n.SendDigestNotification(ctx, toEmail, toName, videos); err != nil {
			slog.Error("multi-notifier: digest notification failed", "error", err)
		}
	}
	return nil
}

// MultiCommentNotifier fans out comment notifications to all registered notifiers.
type MultiCommentNotifier struct {
	notifiers []video.CommentNotifier
}

// NewMultiCommentNotifier creates a notifier that delegates to all provided comment notifiers.
func NewMultiCommentNotifier(notifiers ...video.CommentNotifier) *MultiCommentNotifier {
	return &MultiCommentNotifier{notifiers: notifiers}
}

func (m *MultiCommentNotifier) SendCommentNotification(ctx context.Context, toEmail, toName, videoTitle, commentAuthor, commentBody, watchURL string) error {
	for _, n := range m.notifiers {
		if err := n.SendCommentNotification(ctx, toEmail, toName, videoTitle, commentAuthor, commentBody, watchURL); err != nil {
			slog.Error("multi-notifier: comment notification failed", "error", err)
		}
	}
	return nil
}
