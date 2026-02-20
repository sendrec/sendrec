package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/sendrec/sendrec/internal/email"
)

type mockViewNotifier struct {
	called     bool
	viewEmail  string
	viewName   string
	viewTitle  string
	viewURL    string
	viewCount  int
	digestArgs []email.DigestVideoSummary
	err        error
}

func (m *mockViewNotifier) SendViewNotification(_ context.Context, toEmail, toName, videoTitle, watchURL string, viewCount int) error {
	m.called = true
	m.viewEmail = toEmail
	m.viewName = toName
	m.viewTitle = videoTitle
	m.viewURL = watchURL
	m.viewCount = viewCount
	return m.err
}

func (m *mockViewNotifier) SendDigestNotification(_ context.Context, toEmail, toName string, videos []email.DigestVideoSummary) error {
	m.called = true
	m.viewEmail = toEmail
	m.viewName = toName
	m.digestArgs = videos
	return m.err
}

type mockCommentNotifier struct {
	called        bool
	commentEmail  string
	commentName   string
	commentTitle  string
	commentAuthor string
	commentBody   string
	commentURL    string
	err           error
}

func (m *mockCommentNotifier) SendCommentNotification(_ context.Context, toEmail, toName, videoTitle, commentAuthor, commentBody, watchURL string) error {
	m.called = true
	m.commentEmail = toEmail
	m.commentName = toName
	m.commentTitle = videoTitle
	m.commentAuthor = commentAuthor
	m.commentBody = commentBody
	m.commentURL = watchURL
	return m.err
}

func TestMultiViewNotifier_CallsAllNotifiers(t *testing.T) {
	n1 := &mockViewNotifier{}
	n2 := &mockViewNotifier{}
	multi := NewMultiViewNotifier(n1, n2)

	err := multi.SendViewNotification(context.Background(), "user@example.com", "Alice", "Demo Video", "https://app.sendrec.eu/watch/abc", 5)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !n1.called {
		t.Error("first notifier was not called")
	}
	if !n2.called {
		t.Error("second notifier was not called")
	}
	if n1.viewEmail != "user@example.com" {
		t.Errorf("expected email user@example.com, got %s", n1.viewEmail)
	}
	if n2.viewCount != 5 {
		t.Errorf("expected viewCount 5, got %d", n2.viewCount)
	}
}

func TestMultiViewNotifier_OneFailure_OthersContinue(t *testing.T) {
	n1 := &mockViewNotifier{err: errors.New("email service down")}
	n2 := &mockViewNotifier{}
	multi := NewMultiViewNotifier(n1, n2)

	err := multi.SendViewNotification(context.Background(), "user@example.com", "Alice", "Demo Video", "https://app.sendrec.eu/watch/abc", 3)

	if err != nil {
		t.Fatalf("expected nil error despite notifier failure, got %v", err)
	}
	if !n1.called {
		t.Error("first notifier was not called")
	}
	if !n2.called {
		t.Error("second notifier was not called after first failed")
	}
}

func TestMultiCommentNotifier_CallsAllNotifiers(t *testing.T) {
	n1 := &mockCommentNotifier{}
	n2 := &mockCommentNotifier{}
	multi := NewMultiCommentNotifier(n1, n2)

	err := multi.SendCommentNotification(context.Background(), "owner@example.com", "Bob", "My Video", "Jane", "Great video!", "https://app.sendrec.eu/watch/xyz")

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !n1.called {
		t.Error("first notifier was not called")
	}
	if !n2.called {
		t.Error("second notifier was not called")
	}
	if n1.commentAuthor != "Jane" {
		t.Errorf("expected commentAuthor Jane, got %s", n1.commentAuthor)
	}
	if n2.commentBody != "Great video!" {
		t.Errorf("expected commentBody 'Great video!', got %s", n2.commentBody)
	}
}

func TestMultiCommentNotifier_OneFailure_OthersContinue(t *testing.T) {
	n1 := &mockCommentNotifier{err: errors.New("slack webhook down")}
	n2 := &mockCommentNotifier{}
	multi := NewMultiCommentNotifier(n1, n2)

	err := multi.SendCommentNotification(context.Background(), "owner@example.com", "Bob", "My Video", "Jane", "Great video!", "https://app.sendrec.eu/watch/xyz")

	if err != nil {
		t.Fatalf("expected nil error despite notifier failure, got %v", err)
	}
	if !n1.called {
		t.Error("first notifier was not called")
	}
	if !n2.called {
		t.Error("second notifier was not called after first failed")
	}
}

func TestMultiViewNotifier_DigestCallsAllNotifiers(t *testing.T) {
	n1 := &mockViewNotifier{}
	n2 := &mockViewNotifier{}
	multi := NewMultiViewNotifier(n1, n2)

	videos := []email.DigestVideoSummary{
		{Title: "Video 1", ViewCount: 10, CommentCount: 2, WatchURL: "https://app.sendrec.eu/watch/aaa"},
		{Title: "Video 2", ViewCount: 5, CommentCount: 0, WatchURL: "https://app.sendrec.eu/watch/bbb"},
	}

	err := multi.SendDigestNotification(context.Background(), "user@example.com", "Alice", videos)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !n1.called {
		t.Error("first notifier was not called")
	}
	if !n2.called {
		t.Error("second notifier was not called")
	}
	if len(n1.digestArgs) != 2 {
		t.Errorf("expected 2 digest videos, got %d", len(n1.digestArgs))
	}
	if len(n2.digestArgs) != 2 {
		t.Errorf("expected 2 digest videos, got %d", len(n2.digestArgs))
	}
}

func TestMultiViewNotifier_DigestOneFailure_OthersContinue(t *testing.T) {
	n1 := &mockViewNotifier{err: errors.New("email service down")}
	n2 := &mockViewNotifier{}
	multi := NewMultiViewNotifier(n1, n2)

	videos := []email.DigestVideoSummary{
		{Title: "Video 1", ViewCount: 10, CommentCount: 2, WatchURL: "https://app.sendrec.eu/watch/aaa"},
	}

	err := multi.SendDigestNotification(context.Background(), "user@example.com", "Alice", videos)

	if err != nil {
		t.Fatalf("expected nil error despite notifier failure, got %v", err)
	}
	if !n1.called {
		t.Error("first notifier was not called")
	}
	if !n2.called {
		t.Error("second notifier was not called after first failed")
	}
}
