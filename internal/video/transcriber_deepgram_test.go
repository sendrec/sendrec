package video

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const deepgramSampleResponse = `{
  "results": {
    "channels": [{
      "alternatives": [{
        "transcript": "Hello world how are you",
        "paragraphs": {
          "paragraphs": [{
            "sentences": [
              {"text": "Hello world.", "start": 0.1, "end": 1.2},
              {"text": "How are you?", "start": 1.5, "end": 2.8}
            ]
          }]
        }
      }]
    }]
  }
}`

func TestDeepgram_Transcribe(t *testing.T) {
	var receivedAuth, receivedQuery, receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedQuery = r.URL.RawQuery
		receivedContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, deepgramSampleResponse)
	}))
	defer server.Close()

	tr := newDeepgram("test-key", "nova-3", 0)
	// Point to our test server by swapping baseURL via direct field — there's no setter,
	// so this test verifies request shape against a server reachable at api.deepgram.com.
	// Use a transport-level override instead.
	tr.httpClient.Transport = redirectTransport(server.URL)

	audio := writeTempWav(t, "audio bytes")

	segments, err := tr.Transcribe(context.Background(), audio, "ro")
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}

	if receivedAuth != "Token test-key" {
		t.Errorf("Authorization = %q, want Token test-key", receivedAuth)
	}
	if receivedContentType != "audio/wav" {
		t.Errorf("Content-Type = %q, want audio/wav", receivedContentType)
	}
	if !strings.Contains(receivedQuery, "model=nova-3") {
		t.Errorf("query missing model: %s", receivedQuery)
	}
	if !strings.Contains(receivedQuery, "language=ro") {
		t.Errorf("query missing language=ro: %s", receivedQuery)
	}
	if strings.Contains(receivedQuery, "detect_language=true") {
		t.Errorf("query should not include detect_language when explicit language given: %s", receivedQuery)
	}

	if len(segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(segments))
	}
	if segments[0].Text != "Hello world." || segments[1].Text != "How are you?" {
		t.Errorf("segment text mismatch: %+v", segments)
	}
}

func TestDeepgram_AutoLanguageEnablesDetect(t *testing.T) {
	var receivedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, deepgramSampleResponse)
	}))
	defer server.Close()

	tr := newDeepgram("key", "nova-3", 0)
	tr.httpClient.Transport = redirectTransport(server.URL)
	audio := writeTempWav(t, "audio")

	_, err := tr.Transcribe(context.Background(), audio, "auto")
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}

	if !strings.Contains(receivedQuery, "detect_language=true") {
		t.Errorf("auto language should set detect_language=true, got: %s", receivedQuery)
	}
	if strings.Contains(receivedQuery, "language=") && !strings.Contains(receivedQuery, "detect_language=") {
		t.Errorf("auto language should not set explicit language param: %s", receivedQuery)
	}
}

func TestDeepgram_EmptyTranscriptReturnsErrNoAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"results":{"channels":[{"alternatives":[{"transcript":""}]}]}}`)
	}))
	defer server.Close()

	tr := newDeepgram("k", "nova-3", 0)
	tr.httpClient.Transport = redirectTransport(server.URL)
	audio := writeTempWav(t, "audio")

	_, err := tr.Transcribe(context.Background(), audio, "en")
	if err != ErrNoAudio {
		t.Errorf("expected ErrNoAudio, got: %v", err)
	}
}

func TestDeepgram_NotAvailableWithoutKey(t *testing.T) {
	tr := newDeepgram("", "", 0)
	if tr.Available() {
		t.Error("deepgram should not be available without API key")
	}
}

// redirectTransport rewrites every outgoing request to go to baseURL instead of its
// original Host. Used so we can test the Deepgram client (which has a hard-coded
// api.deepgram.com URL) against an httptest server.
type redirectTransport string

func (r redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target, err := http.NewRequestWithContext(req.Context(), req.Method, string(r)+req.URL.Path+"?"+req.URL.RawQuery, req.Body)
	if err != nil {
		return nil, err
	}
	target.Header = req.Header
	target.ContentLength = req.ContentLength
	return http.DefaultTransport.RoundTrip(target)
}
