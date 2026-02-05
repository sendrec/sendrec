package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewLimiterAllowsFirstRequest(t *testing.T) {
	limiter := NewLimiter(10, 5)

	allowed := limiter.allow("192.168.1.1")

	if !allowed {
		t.Error("expected first request from new IP to be allowed")
	}
}

func TestRequestsWithinBurstAreAllowed(t *testing.T) {
	burst := 5
	limiter := NewLimiter(1, burst)

	for i := 0; i < burst; i++ {
		allowed := limiter.allow("192.168.1.1")
		if !allowed {
			t.Errorf("request %d within burst of %d should be allowed", i+1, burst)
		}
	}
}

func TestRequestsExceedingBurstAreDenied(t *testing.T) {
	burst := 3
	limiter := NewLimiter(1, burst)

	for i := 0; i < burst; i++ {
		limiter.allow("192.168.1.1")
	}

	allowed := limiter.allow("192.168.1.1")
	if allowed {
		t.Error("request exceeding burst should be denied")
	}
}

func TestTokensReplenishOverTime(t *testing.T) {
	limiter := NewLimiter(10, 2)

	// Exhaust all tokens.
	limiter.allow("192.168.1.1")
	limiter.allow("192.168.1.1")

	denied := limiter.allow("192.168.1.1")
	if denied {
		t.Error("expected request to be denied after exhausting burst")
	}

	// Wait long enough for at least one token to replenish.
	// At 10 tokens/sec, 150ms gives ~1.5 tokens.
	time.Sleep(150 * time.Millisecond)

	allowed := limiter.allow("192.168.1.1")
	if !allowed {
		t.Error("expected request to be allowed after token replenishment")
	}
}

func TestDifferentIPsHaveIndependentLimits(t *testing.T) {
	limiter := NewLimiter(1, 2)

	// Exhaust tokens for first IP.
	limiter.allow("10.0.0.1")
	limiter.allow("10.0.0.1")
	denied := limiter.allow("10.0.0.1")
	if denied {
		t.Error("expected third request from first IP to be denied")
	}

	// Second IP should still be allowed.
	allowed := limiter.allow("10.0.0.2")
	if !allowed {
		t.Error("expected first request from second IP to be allowed despite first IP being exhausted")
	}
}

func TestTokensDoNotExceedBurst(t *testing.T) {
	burst := 3
	limiter := NewLimiter(100, burst)

	// Use one token.
	limiter.allow("192.168.1.1")

	// Wait for tokens to refill well beyond burst.
	time.Sleep(200 * time.Millisecond)

	// Should get at most burst number of requests through.
	allowed := 0
	for i := 0; i < burst+2; i++ {
		if limiter.allow("192.168.1.1") {
			allowed++
		}
	}

	if allowed > burst {
		t.Errorf("expected at most %d requests allowed, got %d", burst, allowed)
	}
}

func TestMiddlewareReturns200WhenAllowed(t *testing.T) {
	limiter := NewLimiter(10, 5)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.168.1.1:12345"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}
	if recorder.Body.String() != "ok" {
		t.Errorf("expected body ok, got %s", recorder.Body.String())
	}
}

func TestMiddlewareReturns429WhenRateLimited(t *testing.T) {
	limiter := NewLimiter(1, 1)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request exhausts the single token.
	firstRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	firstRequest.RemoteAddr = "192.168.1.1:12345"
	firstRecorder := httptest.NewRecorder()
	handler.ServeHTTP(firstRecorder, firstRequest)

	// Second request should be rate limited.
	secondRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	secondRequest.RemoteAddr = "192.168.1.1:12345"
	secondRecorder := httptest.NewRecorder()
	handler.ServeHTTP(secondRecorder, secondRequest)

	if secondRecorder.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", secondRecorder.Code)
	}
}

func TestMiddlewareSetsRetryAfterHeader(t *testing.T) {
	limiter := NewLimiter(1, 1)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust the token.
	firstRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	firstRequest.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(httptest.NewRecorder(), firstRequest)

	// Trigger rate limit.
	secondRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	secondRequest.RemoteAddr = "10.0.0.1:1234"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, secondRequest)

	retryAfter := recorder.Header().Get("Retry-After")
	if retryAfter != "10" {
		t.Errorf("expected Retry-After=10, got %s", retryAfter)
	}
}

func TestMiddlewareSetsContentTypeOn429(t *testing.T) {
	limiter := NewLimiter(1, 1)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	firstRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	firstRequest.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(httptest.NewRecorder(), firstRequest)

	secondRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	secondRequest.RemoteAddr = "10.0.0.1:1234"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, secondRequest)

	contentType := recorder.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

func TestMiddlewareReturnsErrorBodyOn429(t *testing.T) {
	limiter := NewLimiter(1, 1)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	firstRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	firstRequest.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(httptest.NewRecorder(), firstRequest)

	secondRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	secondRequest.RemoteAddr = "10.0.0.1:1234"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, secondRequest)

	expected := `{"error":"too many requests"}`
	if recorder.Body.String() != expected {
		t.Errorf("expected body %s, got %s", expected, recorder.Body.String())
	}
}

func TestMiddlewareRespectsXForwardedForHeader(t *testing.T) {
	limiter := NewLimiter(1, 1)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust the token for the forwarded IP.
	firstRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	firstRequest.RemoteAddr = "10.0.0.99:1234"
	firstRequest.Header.Set("X-Forwarded-For", "203.0.113.50")
	handler.ServeHTTP(httptest.NewRecorder(), firstRequest)

	// Same forwarded IP from different RemoteAddr should be rate limited.
	secondRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	secondRequest.RemoteAddr = "10.0.0.100:5678"
	secondRequest.Header.Set("X-Forwarded-For", "203.0.113.50")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, secondRequest)

	if recorder.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for same X-Forwarded-For IP, got %d", recorder.Code)
	}
}

func TestMiddlewareXForwardedForDifferentIPsAreIndependent(t *testing.T) {
	limiter := NewLimiter(1, 1)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust for one forwarded IP.
	firstRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	firstRequest.RemoteAddr = "10.0.0.1:1234"
	firstRequest.Header.Set("X-Forwarded-For", "203.0.113.1")
	handler.ServeHTTP(httptest.NewRecorder(), firstRequest)

	// Different forwarded IP should still be allowed.
	secondRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	secondRequest.RemoteAddr = "10.0.0.1:1234"
	secondRequest.Header.Set("X-Forwarded-For", "203.0.113.2")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, secondRequest)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected 200 for different X-Forwarded-For IP, got %d", recorder.Code)
	}
}

func TestMiddlewarePassesRequestToNextHandler(t *testing.T) {
	limiter := NewLimiter(10, 5)
	called := false

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.RemoteAddr = "192.168.1.1:12345"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if !called {
		t.Error("expected next handler to be called when request is allowed")
	}
}

func TestMiddlewareDoesNotCallNextHandlerWhenRateLimited(t *testing.T) {
	limiter := NewLimiter(1, 1)
	callCount := 0

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 3; i++ {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "10.0.0.1:1234"
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}

	if callCount != 1 {
		t.Errorf("expected next handler called 1 time, got %d", callCount)
	}
}
