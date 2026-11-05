package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/sendrec/sendrec/internal/webhook"
)

func allowPrivateTargets(t *testing.T) {
	t.Helper()
	previous := webhook.AllowPrivateTargets
	webhook.AllowPrivateTargets = true
	t.Cleanup(func() { webhook.AllowPrivateTargets = previous })
}

// newInternalJira stands in for a service on the private network. httptest
// listens on loopback, so any request it counts got past the guard.
func newInternalJira(t *testing.T) (*JiraClient, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"key":"PROJ-1"}`))
	}))
	t.Cleanup(server.Close)
	return NewJiraClient(server.URL, "user@example.com", "token", "PROJ"), &hits
}

func TestJiraValidateConfig_RefusesInternalAddress(t *testing.T) {
	client, hits := newInternalJira(t)

	if err := client.ValidateConfig(context.Background()); err == nil {
		t.Fatal("expected an error connecting to an internal address")
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("internal server received %d requests, want 0", got)
	}
}

func TestJiraCreateIssue_RefusesInternalAddress(t *testing.T) {
	client, hits := newInternalJira(t)

	if _, err := client.CreateIssue(context.Background(), CreateIssueRequest{Title: "Bug"}); err == nil {
		t.Fatal("expected an error connecting to an internal address")
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("internal server received %d requests, want 0", got)
	}
}
