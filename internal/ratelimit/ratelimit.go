package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

type visitor struct {
	tokens    float64
	lastSeen  time.Time
}

type Limiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     float64
	burst    float64
}

func NewLimiter(requestsPerSecond float64, burst int) *Limiter {
	l := &Limiter{
		visitors: make(map[string]*visitor),
		rate:     requestsPerSecond,
		burst:    float64(burst),
	}
	go l.cleanup()
	return l
}

func (l *Limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, exists := l.visitors[ip]
	if !exists {
		l.visitors[ip] = &visitor{tokens: l.burst - 1, lastSeen: time.Now()}
		return true
	}

	elapsed := time.Since(v.lastSeen).Seconds()
	v.lastSeen = time.Now()
	v.tokens += elapsed * l.rate
	if v.tokens > l.burst {
		v.tokens = l.burst
	}

	if v.tokens < 1 {
		return false
	}

	v.tokens--
	return true
}

func (l *Limiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		l.mu.Lock()
		for ip, v := range l.visitors {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}

		if !l.allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "10")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"too many requests"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
