package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
)

func TestSignPayload(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"event":"video.viewed","data":{}}`)

	signature := SignPayload(secret, payload)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if signature != expected {
		t.Errorf("expected signature %s, got %s", expected, signature)
	}

	if !strings.HasPrefix(signature, "sha256=") {
		t.Errorf("signature should start with sha256= prefix, got %s", signature)
	}
}

func TestSignPayloadDifferentSecrets(t *testing.T) {
	payload := []byte(`{"event":"video.viewed"}`)

	sig1 := SignPayload("secret-one", payload)
	sig2 := SignPayload("secret-two", payload)

	if sig1 == sig2 {
		t.Errorf("different secrets should produce different signatures, both got %s", sig1)
	}
}

func expectDeliveryLog(mock pgxmock.PgxPoolIface, eventName string, attempt int) {
	mock.ExpectExec("INSERT INTO webhook_deliveries").
		WithArgs(pgxmock.AnyArg(), eventName, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), attempt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
}

func TestDispatchSuccess(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	var receivedSignature string
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSignature = r.Header.Get("X-Webhook-Signature")
		receivedBody = make([]byte, r.ContentLength)
		r.Body.Read(receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	event := Event{
		Name:      "video.viewed",
		Timestamp: time.Date(2026, 2, 21, 12, 0, 0, 0, time.UTC),
		Data:      map[string]any{"videoId": "abc123"},
	}

	eventJSON, _ := json.Marshal(event)
	expectedSignature := SignPayload("my-secret", eventJSON)

	expectDeliveryLog(mock, "video.viewed", 1)

	client := New(mock)
	client.retryDelays = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond}

	err = client.Dispatch(context.Background(), "user-1", server.URL, "my-secret", event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if receivedSignature != expectedSignature {
		t.Errorf("expected signature %s, got %s", expectedSignature, receivedSignature)
	}

	var receivedEvent Event
	if err := json.Unmarshal(receivedBody, &receivedEvent); err != nil {
		t.Fatalf("failed to unmarshal received body: %v", err)
	}
	if receivedEvent.Name != "video.viewed" {
		t.Errorf("expected event name video.viewed, got %s", receivedEvent.Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestDispatchRetryOnServerError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	var attemptCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attemptCount.Add(1)
		if attempt <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	event := Event{
		Name:      "video.commented",
		Timestamp: time.Now(),
		Data:      map[string]any{"videoId": "v1"},
	}

	expectDeliveryLog(mock, "video.commented", 1)
	expectDeliveryLog(mock, "video.commented", 2)
	expectDeliveryLog(mock, "video.commented", 3)

	client := New(mock)
	client.retryDelays = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond}

	err = client.Dispatch(context.Background(), "user-1", server.URL, "secret", event)
	if err != nil {
		t.Fatalf("expected no error after successful retry, got %v", err)
	}

	if attemptCount.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attemptCount.Load())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestDispatchAllRetriesFail(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	var attemptCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("bad gateway"))
	}))
	defer server.Close()

	event := Event{
		Name:      "video.viewed",
		Timestamp: time.Now(),
		Data:      map[string]any{"videoId": "v2"},
	}

	expectDeliveryLog(mock, "video.viewed", 1)
	expectDeliveryLog(mock, "video.viewed", 2)
	expectDeliveryLog(mock, "video.viewed", 3)

	client := New(mock)
	client.retryDelays = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond}

	err = client.Dispatch(context.Background(), "user-1", server.URL, "secret", event)
	if err == nil {
		t.Fatal("expected error after all retries failed, got nil")
	}

	if !strings.Contains(err.Error(), "502") {
		t.Errorf("expected error to mention status 502, got: %s", err.Error())
	}

	if attemptCount.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attemptCount.Load())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestDispatchConnectionError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Start a server then immediately close it to get a guaranteed connection-refused error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := server.URL
	server.Close()

	event := Event{
		Name:      "video.viewed",
		Timestamp: time.Now(),
		Data:      map[string]any{"videoId": "v3"},
	}

	expectDeliveryLog(mock, "video.viewed", 1)
	expectDeliveryLog(mock, "video.viewed", 2)
	expectDeliveryLog(mock, "video.viewed", 3)

	client := New(mock)
	client.retryDelays = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond}

	err = client.Dispatch(context.Background(), "user-1", unreachableURL, "secret", event)
	if err == nil {
		t.Fatal("expected error for unreachable URL, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestResponseBodyTruncation(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	longBody := strings.Repeat("x", 2000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(longBody))
	}))
	defer server.Close()

	event := Event{
		Name:      "video.viewed",
		Timestamp: time.Now(),
		Data:      map[string]any{},
	}

	expectDeliveryLog(mock, "video.viewed", 1)

	client := New(mock)
	client.retryDelays = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond}

	err = client.Dispatch(context.Background(), "user-1", server.URL, "secret", event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify the constant is set correctly
	if maxResponseBodyBytes != 1024 {
		t.Errorf("expected maxResponseBodyBytes to be 1024, got %d", maxResponseBodyBytes)
	}

	// Directly test doPost to verify truncation behavior
	statusCode, respBody, postErr := client.doPost(context.Background(), server.URL, []byte("{}"), "sha256=test")
	if postErr != nil {
		t.Fatalf("doPost error: %v", postErr)
	}
	if statusCode == nil || *statusCode != 200 {
		t.Fatalf("expected status 200, got %v", statusCode)
	}
	if len(respBody) != maxResponseBodyBytes {
		t.Errorf("expected response body truncated to %d bytes, got %d bytes", maxResponseBodyBytes, len(respBody))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}
