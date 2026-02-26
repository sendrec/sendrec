package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"golang.org/x/crypto/bcrypt"
)

const testSecret = "test-jwt-secret-key"

func newTestHandler(t *testing.T) (*Handler, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	handler := NewHandler(mock, testSecret, false)
	return handler, mock
}

func expectInsertRefreshToken(mock pgxmock.PgxPoolIface, userID string) {
	mock.ExpectExec(`INSERT INTO refresh_tokens`).
		WithArgs(pgxmock.AnyArg(), userID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
}

func expectRefreshValidation(mock pgxmock.PgxPoolIface, tokenID, userID string, expiresAt time.Time, revoked bool) {
	mock.ExpectQuery(`SELECT revoked, expires_at FROM refresh_tokens`).
		WithArgs(tokenID, userID).
		WillReturnRows(pgxmock.NewRows([]string{"revoked", "expires_at"}).
			AddRow(revoked, expiresAt))
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked`).
		WithArgs(tokenID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
}

func decodeTokenResponse(t *testing.T, rec *httptest.ResponseRecorder) tokenResponse {
	t.Helper()
	var resp tokenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func decodeErrorResponse(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	return body.Error
}

func decodeMessageResponse(t *testing.T, rec *httptest.ResponseRecorder) messageResponse {
	t.Helper()
	var resp messageResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func findCookieWithPath(cookies []*http.Cookie, name, path string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name && c.Path == path {
			return c
		}
	}
	return nil
}

// --- Register ---

func TestRegister_Success(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs("alice@example.com", pgxmock.AnyArg(), "Alice").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("user-uuid-1"))

	mock.ExpectExec(`UPDATE email_confirmations SET used_at`).
		WithArgs("user-uuid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	mock.ExpectExec(`INSERT INTO email_confirmations`).
		WithArgs(pgxmock.AnyArg(), "user-uuid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := `{"email":"alice@example.com","password":"strongpass123","name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	resp := decodeMessageResponse(t, rec)
	if resp.Message == "" {
		t.Error("expected non-empty message")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestRegister_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing email", `{"password":"strongpass123","name":"Alice"}`},
		{"missing password", `{"email":"alice@example.com","name":"Alice"}`},
		{"missing name", `{"email":"alice@example.com","password":"strongpass123"}`},
		{"all empty", `{"email":"","password":"","name":""}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler, mock := newTestHandler(t)
			defer mock.Close()

			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			handler.Register(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
}

func TestRegister_InvalidEmail(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	body := `{"email":"not-an-email","password":"strongpass123","name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "invalid email address" {
		t.Errorf("expected error %q, got %q", "invalid email address", errMsg)
	}
}

func TestRegister_PasswordTooShort(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	body := `{"email":"alice@example.com","password":"short","name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "password must be at least 8 characters" {
		t.Errorf("expected password length error, got %q", errMsg)
	}
}

func TestRegister_PasswordTooLong(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	longPassword := strings.Repeat("a", 73)
	body := `{"email":"alice@example.com","password":"` + longPassword + `","name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "password must be at most 72 characters" {
		t.Errorf("expected password max length error, got %q", errMsg)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs("alice@example.com", pgxmock.AnyArg(), "Alice").
		WillReturnError(&pgconn.PgError{Code: "23505"})

	body := `{"email":"alice@example.com","password":"strongpass123","name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "could not create account" {
		t.Errorf("expected duplicate email error, got %q", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestRegister_InvalidJSON(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader("{invalid"))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestRegister_DBError(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs("alice@example.com", pgxmock.AnyArg(), "Alice").
		WillReturnError(pgx.ErrTxClosed)

	body := `{"email":"alice@example.com","password":"strongpass123","name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestRegister_SendsConfirmationEmail(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	emailSender := &mockEmailSender{}
	handler.SetEmailSender(emailSender, "https://app.sendrec.eu")

	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs("alice@example.com", pgxmock.AnyArg(), "Alice").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("user-uuid-1"))

	mock.ExpectExec(`UPDATE email_confirmations SET used_at`).
		WithArgs("user-uuid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	mock.ExpectExec(`INSERT INTO email_confirmations`).
		WithArgs(pgxmock.AnyArg(), "user-uuid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := `{"email":"alice@example.com","password":"strongpass123","name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	if emailSender.lastEmail != "alice@example.com" {
		t.Errorf("expected email sent to alice@example.com, got %q", emailSender.lastEmail)
	}
	if !strings.Contains(emailSender.lastConfirmLink, "/confirm-email?token=") {
		t.Errorf("expected confirm link to contain /confirm-email?token=, got %q", emailSender.lastConfirmLink)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// --- ConfirmEmail ---

func TestConfirmEmail_Success(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	emailSender := &mockEmailSender{}
	handler.SetEmailSender(emailSender, "https://app.sendrec.eu")

	rawToken, tokenHash, _ := generateSecureToken()

	mock.ExpectQuery(`SELECT user_id FROM email_confirmations`).
		WithArgs(tokenHash).
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow("user-uuid-1"))

	mock.ExpectExec(`UPDATE email_confirmations SET used_at`).
		WithArgs(tokenHash).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec(`UPDATE users SET email_verified`).
		WithArgs("user-uuid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectQuery(`SELECT email, name FROM users WHERE id`).
		WithArgs("user-uuid-1").
		WillReturnRows(pgxmock.NewRows([]string{"email", "name"}).AddRow("alice@example.com", "Alice"))

	body := `{"token":"` + rawToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/confirm-email", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ConfirmEmail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	resp := decodeMessageResponse(t, rec)
	if !strings.Contains(resp.Message, "confirmed") {
		t.Errorf("expected message to contain 'confirmed', got %q", resp.Message)
	}

	if !emailSender.welcomeCalled {
		t.Error("expected welcome email to be sent after confirmation")
	}
	if emailSender.lastEmail != "alice@example.com" {
		t.Errorf("expected welcome email to alice@example.com, got %q", emailSender.lastEmail)
	}
	if emailSender.lastDashboardURL != "https://app.sendrec.eu/dashboard" {
		t.Errorf("expected dashboard URL, got %q", emailSender.lastDashboardURL)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestConfirmEmail_InvalidToken(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT user_id FROM email_confirmations`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	body := `{"token":"invalid-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/confirm-email", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ConfirmEmail(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "invalid or expired confirmation link" {
		t.Errorf("expected invalid token error, got %q", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestConfirmEmail_MissingToken(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	body := `{"token":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/confirm-email", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ConfirmEmail(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	_ = mock
}

// --- ResendConfirmation ---

func TestResendConfirmation_Success(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	emailSender := &mockEmailSender{}
	handler.SetEmailSender(emailSender, "https://app.sendrec.eu")

	mock.ExpectQuery(`SELECT id, name, email_verified FROM users WHERE email`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "email_verified"}).AddRow("user-uuid-1", "Alice", false))

	mock.ExpectExec(`UPDATE email_confirmations SET used_at`).
		WithArgs("user-uuid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	mock.ExpectExec(`INSERT INTO email_confirmations`).
		WithArgs(pgxmock.AnyArg(), "user-uuid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := `{"email":"alice@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-confirmation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ResendConfirmation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if emailSender.lastEmail != "alice@example.com" {
		t.Errorf("expected email sent to alice@example.com, got %q", emailSender.lastEmail)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestResendConfirmation_UnknownEmail_StillReturns200(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	emailSender := &mockEmailSender{}
	handler.SetEmailSender(emailSender, "https://app.sendrec.eu")

	mock.ExpectQuery(`SELECT id, name, email_verified FROM users WHERE email`).
		WithArgs("nobody@example.com").
		WillReturnError(pgx.ErrNoRows)

	body := `{"email":"nobody@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-confirmation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ResendConfirmation(rec, req)

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

func TestResendConfirmation_AlreadyVerified_StillReturns200(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	emailSender := &mockEmailSender{}
	handler.SetEmailSender(emailSender, "https://app.sendrec.eu")

	mock.ExpectQuery(`SELECT id, name, email_verified FROM users WHERE email`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "email_verified"}).AddRow("user-uuid-1", "Alice", true))

	body := `{"email":"alice@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-confirmation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ResendConfirmation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if emailSender.lastEmail != "" {
		t.Error("should not send email for already verified user")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestResendConfirmation_MissingEmail(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	body := `{"email":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-confirmation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ResendConfirmation(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	_ = mock
}

// --- Login ---

func TestLogin_Success(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)

	mock.ExpectQuery(`SELECT id, password, email_verified FROM users WHERE email`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "password", "email_verified"}).
			AddRow("user-uuid-1", string(hashedPassword), true))

	expectInsertRefreshToken(mock, "user-uuid-1")

	body := `{"email":"alice@example.com","password":"correctpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	resp := decodeTokenResponse(t, rec)
	if resp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}

	claims, err := ValidateToken(testSecret, resp.AccessToken)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}
	if claims.UserID != "user-uuid-1" {
		t.Errorf("expected userID %q, got %q", "user-uuid-1", claims.UserID)
	}

	cookie := findCookieWithPath(rec.Result().Cookies(), "refresh_token", "/")
	if cookie == nil {
		t.Fatal("expected refresh_token cookie to be set")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestLogin_UnverifiedEmail(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)

	mock.ExpectQuery(`SELECT id, password, email_verified FROM users WHERE email`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "password", "email_verified"}).
			AddRow("user-uuid-1", string(hashedPassword), false))

	body := `{"email":"alice@example.com","password":"correctpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "email_not_verified" {
		t.Errorf("expected email_not_verified error, got %q", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing email", `{"password":"strongpass123"}`},
		{"missing password", `{"email":"alice@example.com"}`},
		{"both empty", `{"email":"","password":""}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler, mock := newTestHandler(t)
			defer mock.Close()

			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			handler.Login(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
}

func TestLogin_WrongEmail(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT id, password, email_verified FROM users WHERE email`).
		WithArgs("nobody@example.com").
		WillReturnError(pgx.ErrNoRows)

	body := `{"email":"nobody@example.com","password":"somepassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "invalid email or password" {
		t.Errorf("expected generic auth error, got %q", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)

	mock.ExpectQuery(`SELECT id, password, email_verified FROM users WHERE email`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "password", "email_verified"}).
			AddRow("user-uuid-1", string(hashedPassword), true))

	body := `{"email":"alice@example.com","password":"wrongpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "invalid email or password" {
		t.Errorf("expected generic auth error, got %q", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("not json"))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// --- Refresh ---

func TestRefresh_Success(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	tokenID := "rt-123"
	refreshToken, err := GenerateRefreshToken(testSecret, "user-uuid-1", tokenID)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	expectRefreshValidation(mock, tokenID, "user-uuid-1", time.Now().Add(RefreshTokenDuration), false)
	expectInsertRefreshToken(mock, "user-uuid-1")

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshToken})
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	resp := decodeTokenResponse(t, rec)
	if resp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}

	claims, err := ValidateToken(testSecret, resp.AccessToken)
	if err != nil {
		t.Fatalf("validate new access token: %v", err)
	}
	if claims.UserID != "user-uuid-1" {
		t.Errorf("expected userID %q, got %q", "user-uuid-1", claims.UserID)
	}

	cookie := findCookieWithPath(rec.Result().Cookies(), "refresh_token", "/")
	if cookie == nil {
		t.Fatal("expected new refresh_token cookie to be set")
	}

	newRefreshClaims, err := ValidateToken(testSecret, cookie.Value)
	if err != nil {
		t.Fatalf("validate new refresh token: %v", err)
	}
	if newRefreshClaims.TokenType != "refresh" {
		t.Errorf("expected refresh token type, got %q", newRefreshClaims.TokenType)
	}
	if newRefreshClaims.UserID != "user-uuid-1" {
		t.Errorf("expected userID %q, got %q", "user-uuid-1", newRefreshClaims.UserID)
	}

	_ = mock
}

func TestRefresh_NoCookie(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "refresh token not found" {
		t.Errorf("expected missing cookie error, got %q", errMsg)
	}

	_ = mock
}

func TestRefresh_InvalidToken(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "invalid-token-string"})
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	_ = mock
}

func TestRefresh_AccessTokenUsedAsRefresh(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	accessToken, err := GenerateAccessToken(testSecret, "user-uuid-1")
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: accessToken})
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "invalid refresh token" {
		t.Errorf("expected invalid refresh token error, got %q", errMsg)
	}

	_ = mock
}

// --- Logout ---

func TestLogout_Returns204(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}

	_ = mock
}

func TestLogout_ClearsRefreshTokenCookie(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	cookie := findCookieWithPath(rec.Result().Cookies(), "refresh_token", "/")
	if cookie == nil {
		t.Fatal("expected refresh_token cookie in response")
	}
	if cookie.MaxAge != -1 {
		t.Errorf("expected MaxAge -1, got %d", cookie.MaxAge)
	}
	if cookie.Value != "" {
		t.Errorf("expected empty cookie value, got %q", cookie.Value)
	}

	legacyCookie := findCookieWithPath(rec.Result().Cookies(), "refresh_token", "/api/auth")
	if legacyCookie == nil {
		t.Fatal("expected legacy refresh_token cookie to be cleared")
	}
	if legacyCookie.MaxAge != -1 {
		t.Errorf("expected legacy cookie MaxAge -1, got %d", legacyCookie.MaxAge)
	}

	_ = mock
}

func TestLogout_CookieSecureFlagMatchesConfig(t *testing.T) {
	tests := []struct {
		name          string
		secureCookies bool
	}{
		{"secure cookies enabled", true},
		{"secure cookies disabled", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("create pgxmock pool: %v", err)
			}
			defer mock.Close()

			handler := NewHandler(mock, testSecret, tc.secureCookies)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
			rec := httptest.NewRecorder()

			handler.Logout(rec, req)

			cookie := findCookieWithPath(rec.Result().Cookies(), "refresh_token", "/")
			if cookie == nil {
				t.Fatal("expected refresh_token cookie in response")
			}
			if cookie.Secure != tc.secureCookies {
				t.Errorf("expected Secure=%v, got %v", tc.secureCookies, cookie.Secure)
			}
		})
	}
}

// --- Middleware ---

func TestMiddleware_NoAuthorizationHeader(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	rec := httptest.NewRecorder()

	handler.Middleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if nextCalled {
		t.Error("next handler should not have been called")
	}

	_ = mock
}

func TestMiddleware_InvalidFormat(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()

	handler.Middleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "invalid authorization header format" {
		t.Errorf("expected format error, got %q", errMsg)
	}
	if nextCalled {
		t.Error("next handler should not have been called")
	}

	_ = mock
}

func TestMiddleware_InvalidToken(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-string")
	rec := httptest.NewRecorder()

	handler.Middleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if nextCalled {
		t.Error("next handler should not have been called")
	}

	_ = mock
}

func TestMiddleware_RefreshTokenUsedAsAccess(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	refreshToken, _ := GenerateRefreshToken(testSecret, "user-uuid-1", "rt-middleware")

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer "+refreshToken)
	rec := httptest.NewRecorder()

	handler.Middleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "invalid token type" {
		t.Errorf("expected token type error, got %q", errMsg)
	}
	if nextCalled {
		t.Error("next handler should not have been called")
	}

	_ = mock
}

func TestMiddleware_ValidAccessToken(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	accessToken, _ := GenerateAccessToken(testSecret, "user-uuid-1")

	var capturedUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	handler.Middleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if capturedUserID != "user-uuid-1" {
		t.Errorf("expected userID %q in context, got %q", "user-uuid-1", capturedUserID)
	}

	_ = mock
}

// --- UserIDFromContext ---

func TestUserIDFromContext_ReturnsUserID(t *testing.T) {
	ctx := context.WithValue(context.Background(), userIDKey, "user-uuid-1")
	userID := UserIDFromContext(ctx)
	if userID != "user-uuid-1" {
		t.Errorf("expected %q, got %q", "user-uuid-1", userID)
	}
}

func TestUserIDFromContext_ReturnsEmptyWhenNotSet(t *testing.T) {
	userID := UserIDFromContext(context.Background())
	if userID != "" {
		t.Errorf("expected empty string, got %q", userID)
	}
}

// --- Cookie attributes ---

func TestRegister_NoTokensIssued(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testSecret, true)

	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs("alice@example.com", pgxmock.AnyArg(), "Alice").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("user-uuid-1"))

	mock.ExpectExec(`UPDATE email_confirmations SET used_at`).
		WithArgs("user-uuid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	mock.ExpectExec(`INSERT INTO email_confirmations`).
		WithArgs(pgxmock.AnyArg(), "user-uuid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := `{"email":"alice@example.com","password":"strongpass123","name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	resp := decodeMessageResponse(t, rec)
	if resp.Message == "" {
		t.Error("expected non-empty message")
	}

	cookie := findCookieWithPath(rec.Result().Cookies(), "refresh_token", "/")
	if cookie != nil {
		t.Error("expected no refresh_token cookie to be set")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// --- mock email sender ---

type mockEmailSender struct {
	lastEmail        string
	lastName         string
	lastResetLink    string
	lastConfirmLink  string
	lastDashboardURL string
	welcomeCalled    bool
	sendErr          error
}

func (m *mockEmailSender) SendPasswordReset(_ context.Context, toEmail, toName, resetLink string) error {
	m.lastEmail = toEmail
	m.lastName = toName
	m.lastResetLink = resetLink
	return m.sendErr
}

func (m *mockEmailSender) SendConfirmation(_ context.Context, toEmail, toName, confirmLink string) error {
	m.lastEmail = toEmail
	m.lastName = toName
	m.lastConfirmLink = confirmLink
	return m.sendErr
}

func (m *mockEmailSender) SendWelcome(_ context.Context, toEmail, toName, dashboardURL string) error {
	m.welcomeCalled = true
	m.lastEmail = toEmail
	m.lastName = toName
	m.lastDashboardURL = dashboardURL
	return m.sendErr
}

// --- ForgotPassword ---

func TestForgotPassword_Success(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	emailSender := &mockEmailSender{}
	handler.SetEmailSender(emailSender, "https://app.sendrec.eu")

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
	handler.SetEmailSender(emailSender, "https://app.sendrec.eu")

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
	handler.SetEmailSender(emailSender, "https://app.sendrec.eu")

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

// --- ResetPassword ---

func TestResetPassword_Success(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	rawToken, tokenHash, _ := generateSecureToken()

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

// --- GetUser ---

func TestGetUser_Success(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT name, email, transcription_language FROM users WHERE id = \$1`).
		WithArgs("user-uuid-1").
		WillReturnRows(pgxmock.NewRows([]string{"name", "email", "transcription_language"}).AddRow("Alice", "alice@example.com", "auto"))

	req := httptest.NewRequest(http.MethodGet, "/api/user", nil)
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-uuid-1"))
	rec := httptest.NewRecorder()

	handler.GetUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Name != "Alice" {
		t.Errorf("expected name %q, got %q", "Alice", resp.Name)
	}
	if resp.Email != "alice@example.com" {
		t.Errorf("expected email %q, got %q", "alice@example.com", resp.Email)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT name, email, transcription_language FROM users WHERE id = \$1`).
		WithArgs("missing-id").
		WillReturnError(pgx.ErrNoRows)

	req := httptest.NewRequest(http.MethodGet, "/api/user", nil)
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "missing-id"))
	rec := httptest.NewRecorder()

	handler.GetUser(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

// --- UpdateUser ---

func TestUpdateUser_ChangeName(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectExec(`UPDATE users SET name = \$1, updated_at = now\(\) WHERE id = \$2`).
		WithArgs("Bob", "user-uuid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body := `{"name":"Bob"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/user", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-uuid-1"))
	rec := httptest.NewRecorder()

	handler.UpdateUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestUpdateUser_ChangePassword(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	oldHash, _ := bcrypt.GenerateFromPassword([]byte("oldpass123"), bcrypt.MinCost)

	mock.ExpectQuery(`SELECT password FROM users WHERE id = \$1`).
		WithArgs("user-uuid-1").
		WillReturnRows(pgxmock.NewRows([]string{"password"}).AddRow(string(oldHash)))

	mock.ExpectExec(`UPDATE users SET password = \$1, updated_at = now\(\) WHERE id = \$2`).
		WithArgs(pgxmock.AnyArg(), "user-uuid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body := `{"currentPassword":"oldpass123","newPassword":"newpass456"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/user", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-uuid-1"))
	rec := httptest.NewRecorder()

	handler.UpdateUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestUpdateUser_WrongCurrentPassword(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	oldHash, _ := bcrypt.GenerateFromPassword([]byte("oldpass123"), bcrypt.MinCost)

	mock.ExpectQuery(`SELECT password FROM users WHERE id = \$1`).
		WithArgs("user-uuid-1").
		WillReturnRows(pgxmock.NewRows([]string{"password"}).AddRow(string(oldHash)))

	body := `{"currentPassword":"wrongpass","newPassword":"newpass456"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/user", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-uuid-1"))
	rec := httptest.NewRecorder()

	handler.UpdateUser(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestUpdateUser_PasswordTooShort(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	body := `{"currentPassword":"oldpass123","newPassword":"short"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/user", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-uuid-1"))
	rec := httptest.NewRecorder()

	handler.UpdateUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpdateUser_NewPasswordWithoutCurrent(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	body := `{"newPassword":"newpass456"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/user", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-uuid-1"))
	rec := httptest.NewRecorder()

	handler.UpdateUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	errMsg := decodeErrorResponse(t, rec)
	if errMsg != "current password is required to set a new password" {
		t.Errorf("expected error about current password, got %q", errMsg)
	}
}

func TestUpdateUser_EmptyBody(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	body := `{}`
	req := httptest.NewRequest(http.MethodPatch, "/api/user", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-uuid-1"))
	rec := httptest.NewRecorder()

	handler.UpdateUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetUser_IncludesTranscriptionLanguage(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT name, email, transcription_language FROM users WHERE id = \$1`).
		WithArgs("user-uuid-1").
		WillReturnRows(pgxmock.NewRows([]string{"name", "email", "transcription_language"}).
			AddRow("Alice", "alice@example.com", "de"))

	req := httptest.NewRequest(http.MethodGet, "/api/user", nil)
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-uuid-1"))
	rec := httptest.NewRecorder()

	handler.GetUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp struct {
		TranscriptionLanguage string `json:"transcriptionLanguage"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.TranscriptionLanguage != "de" {
		t.Errorf("expected transcriptionLanguage %q, got %q", "de", resp.TranscriptionLanguage)
	}
}

func TestUpdateUser_ChangeTranscriptionLanguage(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	mock.ExpectExec(`UPDATE users SET transcription_language = \$1, updated_at = now\(\) WHERE id = \$2`).
		WithArgs("fr", "user-uuid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body := `{"transcriptionLanguage":"fr"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/user", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-uuid-1"))
	rec := httptest.NewRecorder()

	handler.UpdateUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestUpdateUser_InvalidTranscriptionLanguage(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	body := `{"transcriptionLanguage":"invalid"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/user", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-uuid-1"))
	rec := httptest.NewRecorder()

	handler.UpdateUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
