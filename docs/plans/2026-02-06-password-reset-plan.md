# Password Reset Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow users to reset their password via email using Listmonk transactional API.

**Architecture:** Two new API endpoints (`forgot-password`, `reset-password`) in the auth package. A new `internal/email/` package wraps Listmonk's transactional API. Reset tokens are stored as SHA-256 hashes with 1-hour expiry. Two new React pages for the frontend.

**Tech Stack:** Go, PostgreSQL, pgxmock, Listmonk API, React, TypeScript

---

### Task 1: Migration — `password_resets` table

**Files:**
- Create: `migrations/000008_create_password_resets.up.sql`
- Create: `migrations/000008_create_password_resets.down.sql`

**Step 1: Create up migration**

```sql
CREATE TABLE password_resets (
    token_hash  TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_password_resets_user_id ON password_resets(user_id);
```

**Step 2: Create down migration**

```sql
DROP INDEX IF EXISTS idx_password_resets_user_id;
DROP TABLE IF EXISTS password_resets;
```

**Step 3: Verify build**

Run: `go build ./...`
Expected: clean build (migrations are embedded via golang-migrate at runtime)

**Step 4: Commit**

```
feat: add password_resets migration (000008)
```

---

### Task 2: Email client — Listmonk integration

**Files:**
- Create: `internal/email/email.go`
- Create: `internal/email/email_test.go`

**Step 1: Write the tests**

Test file `internal/email/email_test.go`:

```go
package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendPasswordReset_Success(t *testing.T) {
	var receivedBody txRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tx" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			t.Errorf("unexpected auth: %s:%s", user, pass)
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": true}`))
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:    srv.URL,
		Username:   "admin",
		Password:   "secret",
		TemplateID: 5,
	})

	err := client.SendPasswordReset(context.Background(), "alice@example.com", "Alice", "https://app.sendrec.eu/reset-password?token=abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedBody.SubscriberEmail != "alice@example.com" {
		t.Errorf("expected subscriber email %q, got %q", "alice@example.com", receivedBody.SubscriberEmail)
	}
	if receivedBody.TemplateID != 5 {
		t.Errorf("expected template ID 5, got %d", receivedBody.TemplateID)
	}
	resetLink, ok := receivedBody.Data["resetLink"]
	if !ok || resetLink != "https://app.sendrec.eu/reset-password?token=abc123" {
		t.Errorf("expected resetLink in data, got %v", receivedBody.Data)
	}
}

func TestSendPasswordReset_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:    srv.URL,
		Username:   "admin",
		Password:   "secret",
		TemplateID: 5,
	})

	err := client.SendPasswordReset(context.Background(), "alice@example.com", "Alice", "https://example.com/reset")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestSendPasswordReset_NoBaseURL(t *testing.T) {
	client := New(Config{})

	// Should not error — just logs to stdout
	err := client.SendPasswordReset(context.Background(), "alice@example.com", "Alice", "https://example.com/reset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/email/ -v`
Expected: FAIL — package doesn't exist yet

**Step 3: Write the implementation**

File `internal/email/email.go`:

```go
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Config struct {
	BaseURL    string
	Username   string
	Password   string
	TemplateID int
}

type Client struct {
	config Config
	http   *http.Client
}

func New(cfg Config) *Client {
	return &Client{
		config: cfg,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

type txRequest struct {
	SubscriberEmail string            `json:"subscriber_email"`
	TemplateID      int               `json:"template_id"`
	Data            map[string]string `json:"data"`
	ContentType     string            `json:"content_type"`
}

func (c *Client) SendPasswordReset(ctx context.Context, toEmail, toName, resetLink string) error {
	if c.config.BaseURL == "" {
		log.Printf("email not configured — reset link for %s: %s", toEmail, resetLink)
		return nil
	}

	body := txRequest{
		SubscriberEmail: toEmail,
		TemplateID:      c.config.TemplateID,
		Data: map[string]string{
			"resetLink": resetLink,
			"name":      toName,
		},
		ContentType: "html",
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal email request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/api/tx", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create email request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Username, c.config.Password)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("listmonk returned status %d", resp.StatusCode)
	}

	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/email/ -v`
Expected: PASS

**Step 5: Commit**

```
feat: add email client for Listmonk transactional API
```

---

### Task 3: Auth handler — `EmailSender` interface and `ForgotPassword`

**Files:**
- Modify: `internal/auth/auth.go`
- Modify: `internal/auth/auth_test.go`

**Step 1: Write the tests**

Add to `internal/auth/auth_test.go`:

```go
// --- mock email sender ---

type mockEmailSender struct {
	lastEmail     string
	lastName      string
	lastResetLink string
	sendErr       error
}

func (m *mockEmailSender) SendPasswordReset(_ context.Context, toEmail, toName, resetLink string) error {
	m.lastEmail = toEmail
	m.lastName = toName
	m.lastResetLink = resetLink
	return m.sendErr
}

// Update newTestHandler to include nil email sender:
// (This will be updated in the implementation step)

// --- ForgotPassword ---

func TestForgotPassword_Success(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	emailSender := &mockEmailSender{}
	handler.emailSender = emailSender

	mock.ExpectQuery(`SELECT id, name FROM users WHERE email`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).AddRow("user-uuid-1", "Alice"))

	mock.ExpectExec(`UPDATE password_resets SET used_at`).
		WithArgs("user-uuid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	mock.ExpectExec(`INSERT INTO password_resets`).
		WithArgs(pgxmock.AnyArg(), "user-uuid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := `{"email":"alice@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ForgotPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if emailSender.lastEmail != "alice@example.com" {
		t.Errorf("expected email sent to alice@example.com, got %q", emailSender.lastEmail)
	}
	if emailSender.lastResetLink == "" {
		t.Error("expected non-empty reset link")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestForgotPassword_UnknownEmail_StillReturns200(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	emailSender := &mockEmailSender{}
	handler.emailSender = emailSender

	mock.ExpectQuery(`SELECT id, name FROM users WHERE email`).
		WithArgs("nobody@example.com").
		WillReturnError(pgx.ErrNoRows)

	body := `{"email":"nobody@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ForgotPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if emailSender.lastEmail != "" {
		t.Error("should not send email for unknown user")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestForgotPassword_MissingEmail(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	body := `{"email":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ForgotPassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestForgotPassword_EmailSendFailure_StillReturns200(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	emailSender := &mockEmailSender{sendErr: errors.New("smtp down")}
	handler.emailSender = emailSender

	mock.ExpectQuery(`SELECT id, name FROM users WHERE email`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).AddRow("user-uuid-1", "Alice"))

	mock.ExpectExec(`UPDATE password_resets SET used_at`).
		WithArgs("user-uuid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	mock.ExpectExec(`INSERT INTO password_resets`).
		WithArgs(pgxmock.AnyArg(), "user-uuid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := `{"email":"alice@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ForgotPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/ -run ForgotPassword -v`
Expected: FAIL — `ForgotPassword` method doesn't exist

**Step 3: Write the implementation**

Add to `internal/auth/auth.go`:

1. Add `EmailSender` interface and update `Handler`:

```go
type EmailSender interface {
	SendPasswordReset(ctx context.Context, toEmail, toName, resetLink string) error
}

type Handler struct {
	db            database.DBTX
	jwtSecret     string
	secureCookies bool
	emailSender   EmailSender
	baseURL       string
}
```

2. Update `NewHandler`:

```go
func NewHandler(db database.DBTX, jwtSecret string, secureCookies bool) *Handler {
	return &Handler{db: db, jwtSecret: jwtSecret, secureCookies: secureCookies}
}

func (h *Handler) SetEmailSender(sender EmailSender, baseURL string) {
	h.emailSender = sender
	h.baseURL = baseURL
}
```

3. Add imports: `"crypto/sha256"`, `"encoding/base64"`, `"fmt"` (most already present). Add `"log"`.

4. Add `ForgotPassword` handler and helpers:

```go
type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type messageResponse struct {
	Message string `json:"message"`
}

const resetTokenExpiry = 1 * time.Hour

func generateResetToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	return raw, hash, nil
}

func hashResetToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" {
		httputil.WriteError(w, http.StatusBadRequest, "email is required")
		return
	}

	response := messageResponse{Message: "If an account with that email exists, we've sent a password reset link"}

	var userID, userName string
	err := h.db.QueryRow(r.Context(),
		"SELECT id, name FROM users WHERE email = $1", req.Email,
	).Scan(&userID, &userName)
	if err != nil {
		// User not found — return 200 anyway (no enumeration)
		httputil.WriteJSON(w, http.StatusOK, response)
		return
	}

	// Invalidate existing unused reset tokens
	if _, err := h.db.Exec(r.Context(),
		"UPDATE password_resets SET used_at = now() WHERE user_id = $1 AND used_at IS NULL",
		userID,
	); err != nil {
		log.Printf("forgot-password: failed to invalidate old tokens: %v", err)
	}

	rawToken, tokenHash, err := generateResetToken()
	if err != nil {
		log.Printf("forgot-password: failed to generate token: %v", err)
		httputil.WriteJSON(w, http.StatusOK, response)
		return
	}

	if _, err := h.db.Exec(r.Context(),
		"INSERT INTO password_resets (token_hash, user_id, expires_at) VALUES ($1, $2, $3)",
		tokenHash, userID, time.Now().Add(resetTokenExpiry),
	); err != nil {
		log.Printf("forgot-password: failed to store token: %v", err)
		httputil.WriteJSON(w, http.StatusOK, response)
		return
	}

	resetLink := h.baseURL + "/reset-password?token=" + rawToken
	if h.emailSender != nil {
		if err := h.emailSender.SendPasswordReset(r.Context(), req.Email, userName, resetLink); err != nil {
			log.Printf("forgot-password: failed to send email: %v", err)
		}
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/ -v`
Expected: PASS (all existing + new tests)

**Step 5: Commit**

```
feat: add forgot-password endpoint with email sender interface
```

---

### Task 4: Auth handler — `ResetPassword`

**Files:**
- Modify: `internal/auth/auth.go`
- Modify: `internal/auth/auth_test.go`

**Step 1: Write the tests**

Add to `internal/auth/auth_test.go`:

```go
// --- ResetPassword ---

func TestResetPassword_Success(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	rawToken, tokenHash, _ := generateResetToken()

	mock.ExpectQuery(`SELECT user_id FROM password_resets`).
		WithArgs(tokenHash).
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow("user-uuid-1"))

	mock.ExpectExec(`UPDATE password_resets SET used_at`).
		WithArgs(tokenHash).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec(`UPDATE users SET password`).
		WithArgs(pgxmock.AnyArg(), "user-uuid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec(`UPDATE refresh_tokens SET revoked`).
		WithArgs("user-uuid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 2))

	body := `{"token":"` + rawToken + `","password":"newpassword123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ResetPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestResetPassword_InvalidToken(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT user_id FROM password_resets`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	body := `{"token":"invalid-token","password":"newpassword123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ResetPassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "invalid or expired reset link" {
		t.Errorf("expected invalid token error, got %q", errMsg)
	}
}

func TestResetPassword_PasswordTooShort(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	body := `{"token":"sometoken","password":"short"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ResetPassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestResetPassword_MissingFields(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	body := `{"token":"","password":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ResetPassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/ -run ResetPassword -v`
Expected: FAIL — `ResetPassword` method doesn't exist

**Step 3: Write the implementation**

Add to `internal/auth/auth.go`:

```go
type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Token == "" || req.Password == "" {
		httputil.WriteError(w, http.StatusBadRequest, "token and password are required")
		return
	}

	if len(req.Password) < 8 {
		httputil.WriteError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	if len(req.Password) > 72 {
		httputil.WriteError(w, http.StatusBadRequest, "password must be at most 72 characters")
		return
	}

	tokenHash := hashResetToken(req.Token)

	var userID string
	err := h.db.QueryRow(r.Context(),
		"SELECT user_id FROM password_resets WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()",
		tokenHash,
	).Scan(&userID)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid or expired reset link")
		return
	}

	// Mark token as used
	if _, err := h.db.Exec(r.Context(),
		"UPDATE password_resets SET used_at = now() WHERE token_hash = $1",
		tokenHash,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to process reset")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if _, err := h.db.Exec(r.Context(),
		"UPDATE users SET password = $1, updated_at = now() WHERE id = $2",
		string(hashedPassword), userID,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	// Revoke all refresh tokens for this user
	if _, err := h.db.Exec(r.Context(),
		"UPDATE refresh_tokens SET revoked = true, revoked_at = now() WHERE user_id = $1 AND revoked = false",
		userID,
	); err != nil {
		log.Printf("reset-password: failed to revoke refresh tokens: %v", err)
	}

	httputil.WriteJSON(w, http.StatusOK, messageResponse{Message: "Password updated successfully"})
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/ -v`
Expected: PASS

**Step 5: Commit**

```
feat: add reset-password endpoint
```

---

### Task 5: Wire routes and config

**Files:**
- Modify: `internal/server/server.go`
- Modify: `cmd/sendrec/main.go`

**Step 1: Add routes in `server.go`**

Add the two new routes inside the existing `/api/auth` route group in `routes()`:

```go
r.Post("/forgot-password", s.authHandler.ForgotPassword)
r.Post("/reset-password", s.authHandler.ResetPassword)
```

Add `EmailSender` config fields to `Config` struct and wire in `New()`:

```go
// In Config struct:
EmailSender    auth.EmailSender
```

In `New()`, after creating `s.authHandler`:

```go
if cfg.EmailSender != nil {
	s.authHandler.SetEmailSender(cfg.EmailSender, baseURL)
}
```

**Step 2: Wire in `main.go`**

Add import for `"github.com/sendrec/sendrec/internal/email"` and create the email client:

```go
emailClient := email.New(email.Config{
	BaseURL:    os.Getenv("LISTMONK_URL"),
	Username:   getEnv("LISTMONK_USER", "admin"),
	Password:   os.Getenv("LISTMONK_PASSWORD"),
	TemplateID: int(getEnvInt64("LISTMONK_TEMPLATE_ID", 0)),
})
```

Pass `EmailSender: emailClient` to `server.Config`.

**Step 3: Verify build and tests**

Run: `go build ./... && go test ./...`
Expected: clean build, all tests pass

**Step 4: Commit**

```
feat: wire password reset routes and email config
```

---

### Task 6: Frontend — `ForgotPassword` page

**Files:**
- Create: `web/src/pages/ForgotPassword.tsx`
- Modify: `web/src/pages/Login.tsx`
- Modify: `web/src/App.tsx`

**Step 1: Create `ForgotPassword.tsx`**

```tsx
import { type FormEvent, useState } from "react";
import { Link } from "react-router-dom";

export function ForgotPassword() {
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [sent, setSent] = useState(false);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");
    setLoading(true);

    try {
      const response = await fetch("/api/auth/forgot-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });

      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error || "Something went wrong");
      }

      setSent(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }

  if (sent) {
    return (
      <div style={{ maxWidth: 400, margin: "80px auto", padding: 24 }}>
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 24,
            textAlign: "center",
          }}
        >
          <h1
            style={{
              color: "var(--color-text)",
              fontSize: 24,
              marginBottom: 16,
            }}
          >
            Check your email
          </h1>
          <p
            style={{
              color: "var(--color-text-secondary)",
              fontSize: 14,
              marginBottom: 24,
            }}
          >
            If an account with that email exists, we&apos;ve sent a password
            reset link. The link expires in 1 hour.
          </p>
          <Link
            to="/login"
            style={{ color: "var(--color-accent)", fontSize: 14 }}
          >
            Back to sign in
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 400, margin: "80px auto", padding: 24 }}>
      <h1
        style={{
          color: "var(--color-text)",
          fontSize: 24,
          marginBottom: 24,
          textAlign: "center",
        }}
      >
        Reset password
      </h1>
      <form
        onSubmit={handleSubmit}
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: 8,
          padding: 24,
          display: "flex",
          flexDirection: "column",
          gap: 16,
        }}
      >
        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span
            style={{ color: "var(--color-text-secondary)", fontSize: 14 }}
          >
            Email
          </span>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            style={{
              background: "var(--color-bg)",
              border: "1px solid var(--color-border)",
              borderRadius: 4,
              color: "var(--color-text)",
              padding: "8px 12px",
              fontSize: 14,
            }}
          />
        </label>

        {error && (
          <p style={{ color: "var(--color-error)", fontSize: 14, margin: 0 }}>
            {error}
          </p>
        )}

        <button
          type="submit"
          disabled={loading}
          style={{
            background: "var(--color-accent)",
            color: "var(--color-text)",
            borderRadius: 4,
            padding: "10px 16px",
            fontSize: 14,
            fontWeight: 600,
            opacity: loading ? 0.7 : 1,
          }}
        >
          {loading ? "Sending..." : "Send reset link"}
        </button>

        <p
          style={{
            color: "var(--color-text-secondary)",
            fontSize: 14,
            textAlign: "center",
            margin: 0,
          }}
        >
          <Link to="/login" style={{ color: "var(--color-accent)" }}>
            Back to sign in
          </Link>
        </p>
      </form>
    </div>
  );
}
```

**Step 2: Add "Forgot password?" link to Login.tsx**

In `Login.tsx`, update the `footer` prop of `AuthForm` to include the forgot password link:

```tsx
footer={
  <>
    <Link to="/forgot-password" style={{ display: "block", marginBottom: 8 }}>
      Forgot password?
    </Link>
    Don&apos;t have an account? <Link to="/register">Sign up</Link>
  </>
}
```

**Step 3: Add route to `App.tsx`**

Add import: `import { ForgotPassword } from "./pages/ForgotPassword";`

Add route inside `<Routes>` after the register route:

```tsx
<Route path="/forgot-password" element={<ForgotPassword />} />
```

**Step 4: Verify frontend builds**

Run: `cd web && pnpm typecheck && pnpm build`
Expected: clean typecheck and build

**Step 5: Commit**

```
feat: add forgot-password page with link from login
```

---

### Task 7: Frontend — `ResetPassword` page

**Files:**
- Create: `web/src/pages/ResetPassword.tsx`
- Modify: `web/src/App.tsx`

**Step 1: Create `ResetPassword.tsx`**

```tsx
import { type FormEvent, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";

export function ResetPassword() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get("token");

  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState(false);

  if (!token) {
    return (
      <div style={{ maxWidth: 400, margin: "80px auto", padding: 24 }}>
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 24,
            textAlign: "center",
          }}
        >
          <h1
            style={{
              color: "var(--color-text)",
              fontSize: 24,
              marginBottom: 16,
            }}
          >
            Invalid reset link
          </h1>
          <p
            style={{
              color: "var(--color-text-secondary)",
              fontSize: 14,
              marginBottom: 24,
            }}
          >
            This password reset link is invalid. Please request a new one.
          </p>
          <Link
            to="/forgot-password"
            style={{ color: "var(--color-accent)", fontSize: 14 }}
          >
            Request new reset link
          </Link>
        </div>
      </div>
    );
  }

  if (success) {
    return (
      <div style={{ maxWidth: 400, margin: "80px auto", padding: 24 }}>
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 24,
            textAlign: "center",
          }}
        >
          <h1
            style={{
              color: "var(--color-text)",
              fontSize: 24,
              marginBottom: 16,
            }}
          >
            Password updated
          </h1>
          <p
            style={{
              color: "var(--color-text-secondary)",
              fontSize: 14,
              marginBottom: 24,
            }}
          >
            Your password has been reset successfully.
          </p>
          <Link
            to="/login"
            style={{ color: "var(--color-accent)", fontSize: 14 }}
          >
            Sign in
          </Link>
        </div>
      </div>
    );
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");

    if (password !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }

    if (password.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }

    setLoading(true);

    try {
      const response = await fetch("/api/auth/reset-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token, password }),
      });

      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error || "Something went wrong");
      }

      setSuccess(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={{ maxWidth: 400, margin: "80px auto", padding: 24 }}>
      <h1
        style={{
          color: "var(--color-text)",
          fontSize: 24,
          marginBottom: 24,
          textAlign: "center",
        }}
      >
        Set new password
      </h1>
      <form
        onSubmit={handleSubmit}
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: 8,
          padding: 24,
          display: "flex",
          flexDirection: "column",
          gap: 16,
        }}
      >
        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span
            style={{ color: "var(--color-text-secondary)", fontSize: 14 }}
          >
            New password
          </span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
            style={{
              background: "var(--color-bg)",
              border: "1px solid var(--color-border)",
              borderRadius: 4,
              color: "var(--color-text)",
              padding: "8px 12px",
              fontSize: 14,
            }}
          />
        </label>

        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span
            style={{ color: "var(--color-text-secondary)", fontSize: 14 }}
          >
            Confirm password
          </span>
          <input
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
            minLength={8}
            style={{
              background: "var(--color-bg)",
              border: "1px solid var(--color-border)",
              borderRadius: 4,
              color: "var(--color-text)",
              padding: "8px 12px",
              fontSize: 14,
            }}
          />
        </label>

        {error && (
          <p style={{ color: "var(--color-error)", fontSize: 14, margin: 0 }}>
            {error}
          </p>
        )}

        <button
          type="submit"
          disabled={loading}
          style={{
            background: "var(--color-accent)",
            color: "var(--color-text)",
            borderRadius: 4,
            padding: "10px 16px",
            fontSize: 14,
            fontWeight: 600,
            opacity: loading ? 0.7 : 1,
          }}
        >
          {loading ? "Updating..." : "Reset password"}
        </button>

        <p
          style={{
            color: "var(--color-text-secondary)",
            fontSize: 14,
            textAlign: "center",
            margin: 0,
          }}
        >
          <Link
            to="/forgot-password"
            style={{ color: "var(--color-accent)" }}
          >
            Request new reset link
          </Link>
        </p>
      </form>
    </div>
  );
}
```

**Step 2: Add route to `App.tsx`**

Add import: `import { ResetPassword } from "./pages/ResetPassword";`

Add route after forgot-password:

```tsx
<Route path="/reset-password" element={<ResetPassword />} />
```

**Step 3: Verify frontend builds**

Run: `cd web && pnpm typecheck && pnpm build`
Expected: clean typecheck and build

**Step 4: Commit**

```
feat: add reset-password page
```

---

### Task 8: Final verification

**Step 1: Run all Go tests**

Run: `go test ./... -count=1`
Expected: all pass

**Step 2: Run frontend checks**

Run: `cd web && pnpm typecheck && pnpm build`
Expected: clean

**Step 3: Run go vet**

Run: `go vet ./...`
Expected: clean

**Step 4: Commit any remaining changes and summarize**

Review the full set of changes and ensure everything is committed.

---

## Files Modified/Created Summary

| File | Action |
|------|--------|
| `migrations/000008_create_password_resets.up.sql` | Create |
| `migrations/000008_create_password_resets.down.sql` | Create |
| `internal/email/email.go` | Create |
| `internal/email/email_test.go` | Create |
| `internal/auth/auth.go` | Modify |
| `internal/auth/auth_test.go` | Modify |
| `internal/server/server.go` | Modify |
| `cmd/sendrec/main.go` | Modify |
| `web/src/pages/ForgotPassword.tsx` | Create |
| `web/src/pages/ResetPassword.tsx` | Create |
| `web/src/pages/Login.tsx` | Modify |
| `web/src/App.tsx` | Modify |

## Deployment Notes

After deploying, you need to:
1. Create a transactional email template in Listmonk admin
2. Add to `/opt/sendrec/.env`:
   - `LISTMONK_URL=http://listmonk:9000` (Docker internal)
   - `LISTMONK_USER=admin`
   - `LISTMONK_PASSWORD=<your-listmonk-admin-password>`
   - `LISTMONK_TEMPLATE_ID=<template-id-from-listmonk>`
3. Restart the app container
