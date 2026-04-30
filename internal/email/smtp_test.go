package email

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTPServer is a minimal in-memory SMTP server for tests.
// Captures every message it receives and exposes them via Captured().
type fakeSMTPServer struct {
	listener net.Listener
	addr     string

	mu       sync.Mutex
	captured []capturedMessage

	// authConfig (optional) — when set, AUTH PLAIN is required
	authUser, authPass string
	requireAuth        bool

	// responses — flip to inject failure cases
	authShouldFail bool

	wg     sync.WaitGroup
	closed chan struct{}
}

type capturedMessage struct {
	from     string
	rcpts    []string
	data     string // raw DATA payload (headers + body)
	authUser string // username from PLAIN auth
}

func newFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &fakeSMTPServer{
		listener: ln,
		addr:     ln.Addr().String(),
		closed:   make(chan struct{}),
	}
	s.wg.Add(1)
	go s.acceptLoop()
	t.Cleanup(s.Close)
	return s
}

func (s *fakeSMTPServer) Addr() string { return s.addr }

func (s *fakeSMTPServer) Captured() []capturedMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]capturedMessage, len(s.captured))
	copy(out, s.captured)
	return out
}

func (s *fakeSMTPServer) RequireAuth(user, pass string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requireAuth = true
	s.authUser = user
	s.authPass = pass
}

func (s *fakeSMTPServer) FailAuth() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authShouldFail = true
}

func (s *fakeSMTPServer) Close() {
	select {
	case <-s.closed:
		return
	default:
	}
	close(s.closed)
	_ = s.listener.Close()
	s.wg.Wait()
}

func (s *fakeSMTPServer) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(5 * time.Second))
			s.handle(c)
		}(conn)
	}
}

func (s *fakeSMTPServer) handle(c net.Conn) {
	r := bufio.NewReader(c)
	w := bufio.NewWriter(c)
	write := func(line string) {
		_, _ = w.WriteString(line + "\r\n")
		_ = w.Flush()
	}

	write("220 fake.smtp ESMTP")

	msg := capturedMessage{}

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(strings.ToUpper(line), "EHLO"), strings.HasPrefix(strings.ToUpper(line), "HELO"):
			write("250-fake.smtp")
			write("250-AUTH PLAIN LOGIN")
			write("250 8BITMIME")
		case strings.HasPrefix(strings.ToUpper(line), "AUTH PLAIN"):
			s.mu.Lock()
			fail := s.authShouldFail
			s.mu.Unlock()
			if fail {
				write("535 auth failed")
				continue
			}
			// AUTH PLAIN <base64> — accept any creds in fake unless RequireAuth set
			parts := strings.SplitN(line, " ", 3)
			if len(parts) == 3 {
				msg.authUser = parts[2]
			}
			write("235 ok")
		case strings.HasPrefix(strings.ToUpper(line), "MAIL FROM:"):
			msg.from = strings.TrimSpace(line[len("MAIL FROM:"):])
			write("250 ok")
		case strings.HasPrefix(strings.ToUpper(line), "RCPT TO:"):
			msg.rcpts = append(msg.rcpts, strings.TrimSpace(line[len("RCPT TO:"):]))
			write("250 ok")
		case strings.EqualFold(line, "DATA"):
			write("354 send data")
			var b strings.Builder
			for {
				dl, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(dl, "\r\n") == "." {
					break
				}
				b.WriteString(dl)
			}
			msg.data = b.String()
			write("250 ok")
		case strings.EqualFold(line, "QUIT"):
			write("221 bye")
			s.mu.Lock()
			s.captured = append(s.captured, msg)
			s.mu.Unlock()
			return
		case strings.EqualFold(line, "RSET"):
			msg = capturedMessage{}
			write("250 ok")
		default:
			write("500 unknown command")
		}
	}
}

// --- tests ---

func TestSendTx_SMTP_Success(t *testing.T) {
	s := newFakeSMTPServer(t)
	host, port := splitHostPort(t, s.Addr())

	client := New(Config{
		SMTPHost:     host,
		SMTPPort:     port,
		SMTPUsername: "user",
		SMTPPassword: "pass",
		SMTPTLS:      "none",
		FromAddress:  "noreply@sendrec.eu",
	})

	err := client.SendConfirmation(context.Background(), "alice@example.com", "Alice", "https://app.sendrec.eu/confirm?token=abc")
	if err != nil {
		t.Fatalf("SendConfirmation: %v", err)
	}

	msgs := waitForMessages(t, s, 1)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	got := msgs[0]
	if !strings.Contains(got.from, "noreply@sendrec.eu") {
		t.Errorf("unexpected from: %q", got.from)
	}
	if len(got.rcpts) != 1 || !strings.Contains(got.rcpts[0], "alice@example.com") {
		t.Errorf("unexpected rcpts: %v", got.rcpts)
	}
	if !strings.Contains(got.data, "Subject: Confirm your email") {
		t.Errorf("missing subject header in data: %q", got.data)
	}
	if !strings.Contains(got.data, "https://app.sendrec.eu/confirm?token=abc") {
		t.Errorf("missing confirm link in body: %q", got.data)
	}
}

func TestSendTx_SMTP_AuthFailure_ReturnsError(t *testing.T) {
	s := newFakeSMTPServer(t)
	s.FailAuth()
	host, port := splitHostPort(t, s.Addr())

	client := New(Config{
		SMTPHost:     host,
		SMTPPort:     port,
		SMTPUsername: "user",
		SMTPPassword: "wrong",
		SMTPTLS:      "none",
		FromAddress:  "noreply@sendrec.eu",
	})

	err := client.SendConfirmation(context.Background(), "alice@example.com", "Alice", "https://example.com/confirm")
	if err == nil {
		t.Fatalf("expected SMTP auth error, got nil")
	}
}

func TestSendTx_PrefersListmonkOverSMTP(t *testing.T) {
	listmonkHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/api/tx") {
			listmonkHit = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	smtpSrv := newFakeSMTPServer(t)
	host, port := splitHostPort(t, smtpSrv.Addr())

	client := New(Config{
		BaseURL:           srv.URL,
		Username:          "admin",
		Password:          "secret",
		ConfirmTemplateID: 7,
		SMTPHost:          host,
		SMTPPort:          port,
		SMTPUsername:      "u",
		SMTPPassword:      "p",
		SMTPTLS:           "none",
		FromAddress:       "noreply@sendrec.eu",
	})

	if err := client.SendConfirmation(context.Background(), "alice@example.com", "Alice", "https://example.com/confirm"); err != nil {
		t.Fatalf("send: %v", err)
	}

	if !listmonkHit {
		t.Error("expected listmonk to be hit when both backends configured")
	}
	if got := smtpSrv.Captured(); len(got) != 0 {
		t.Errorf("SMTP unexpectedly received message: %+v", got)
	}
}

func TestSendTx_SMTP_ConnectionError_ReturnsError(t *testing.T) {
	client := New(Config{
		SMTPHost:    "127.0.0.1",
		SMTPPort:    1, // closed port
		SMTPTLS:     "none",
		FromAddress: "noreply@sendrec.eu",
	})

	err := client.SendPasswordReset(context.Background(), "alice@example.com", "Alice", "https://example.com/reset")
	if err == nil {
		t.Fatal("expected SMTP connection error, got nil")
	}
}

// fakeNoSTARTTLSServer behaves like fakeSMTPServer but does not advertise STARTTLS in EHLO.
type fakeNoSTARTTLSServer struct {
	listener net.Listener
	addr     string
	wg       sync.WaitGroup
	closed   chan struct{}
}

func newFakeNoSTARTTLSServer(t *testing.T) *fakeNoSTARTTLSServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &fakeNoSTARTTLSServer{listener: ln, addr: ln.Addr().String(), closed: make(chan struct{})}
	s.wg.Add(1)
	go s.acceptLoop()
	t.Cleanup(s.Close)
	return s
}

func (s *fakeNoSTARTTLSServer) Addr() string { return s.addr }

func (s *fakeNoSTARTTLSServer) Close() {
	select {
	case <-s.closed:
		return
	default:
	}
	close(s.closed)
	_ = s.listener.Close()
	s.wg.Wait()
}

func (s *fakeNoSTARTTLSServer) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(5 * time.Second))
			r := bufio.NewReader(c)
			w := bufio.NewWriter(c)
			write := func(line string) {
				_, _ = w.WriteString(line + "\r\n")
				_ = w.Flush()
			}
			write("220 fake.smtp ESMTP")
			for {
				line, err := r.ReadString('\n')
				if err != nil {
					return
				}
				line = strings.TrimRight(line, "\r\n")
				up := strings.ToUpper(line)
				switch {
				case strings.HasPrefix(up, "EHLO"), strings.HasPrefix(up, "HELO"):
					write("250-fake.smtp")
					write("250 8BITMIME") // intentionally NO STARTTLS
				case strings.EqualFold(line, "QUIT"):
					write("221 bye")
					return
				default:
					write("500 not supported in this fake")
				}
			}
		}(conn)
	}
}

func TestSendTx_SMTP_StartTLSRequired_FailsIfServerDoesNotOffer(t *testing.T) {
	s := newFakeNoSTARTTLSServer(t)
	host, port := splitHostPort(t, s.Addr())

	client := New(Config{
		SMTPHost:    host,
		SMTPPort:    port,
		SMTPTLS:     "starttls",
		FromAddress: "noreply@sendrec.eu",
	})

	err := client.SendPasswordReset(context.Background(), "alice@example.com", "Alice", "https://example.com/reset")
	if err == nil {
		t.Fatal("expected error when STARTTLS required but server does not offer it")
	}
	if !strings.Contains(err.Error(), "STARTTLS") {
		t.Errorf("expected STARTTLS error message, got: %v", err)
	}
}

func TestSendTx_SMTP_StartTLSDefaultIsRequired(t *testing.T) {
	// Empty SMTPTLS should default to "starttls" (S1 fix).
	s := newFakeNoSTARTTLSServer(t)
	host, port := splitHostPort(t, s.Addr())

	client := New(Config{
		SMTPHost:    host,
		SMTPPort:    port,
		// SMTPTLS intentionally unset
		FromAddress: "noreply@sendrec.eu",
	})

	err := client.SendPasswordReset(context.Background(), "alice@example.com", "Alice", "https://example.com/reset")
	if err == nil {
		t.Fatal("expected default mode to require STARTTLS and error out when server lacks it")
	}
}

func TestHasBackend(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   bool
	}{
		{"no backend", Config{}, false},
		{"listmonk only", Config{BaseURL: "https://listmonk.example"}, true},
		{"smtp only", Config{SMTPHost: "smtp.example", SMTPPort: 587}, true},
		{"both", Config{BaseURL: "https://listmonk.example", SMTPHost: "smtp.example"}, true},
		{"smtp host without port", Config{SMTPHost: "smtp.example"}, true}, // host alone counts; port has default
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.config)
			if got := c.HasBackend(); got != tt.want {
				t.Errorf("HasBackend() = %v, want %v", got, tt.want)
			}
		})
	}
}

func splitHostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("port parse: %v", err)
	}
	return host, port
}

func waitForMessages(t *testing.T, s *fakeSMTPServer, n int) []capturedMessage {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msgs := s.Captured()
		if len(msgs) >= n {
			return msgs
		}
		time.Sleep(10 * time.Millisecond)
	}
	return s.Captured()
}
