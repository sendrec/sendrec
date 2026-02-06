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

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
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

	expectInsertRefreshToken(mock, "user-uuid-1")

	body := `{"email":"alice@example.com","password":"strongpass123","name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
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
	if claims.TokenType != "access" {
		t.Errorf("expected token type %q, got %q", "access", claims.TokenType)
	}

	cookie := findCookie(rec.Result().Cookies(), "refresh_token")
	if cookie == nil {
		t.Fatal("expected refresh_token cookie to be set")
	}
	if cookie.HttpOnly != true {
		t.Error("expected refresh_token cookie to be HttpOnly")
	}

	refreshClaims, err := ValidateToken(testSecret, cookie.Value)
	if err != nil {
		t.Fatalf("validate refresh token: %v", err)
	}
	if refreshClaims.TokenType != "refresh" {
		t.Errorf("expected refresh token type, got %q", refreshClaims.TokenType)
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

// --- Login ---

func TestLogin_Success(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)

	mock.ExpectQuery(`SELECT id, password FROM users WHERE email`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "password"}).
			AddRow("user-uuid-1", string(hashedPassword)))

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

	cookie := findCookie(rec.Result().Cookies(), "refresh_token")
	if cookie == nil {
		t.Fatal("expected refresh_token cookie to be set")
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

	mock.ExpectQuery(`SELECT id, password FROM users WHERE email`).
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

	mock.ExpectQuery(`SELECT id, password FROM users WHERE email`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "password"}).
			AddRow("user-uuid-1", string(hashedPassword)))

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

	cookie := findCookie(rec.Result().Cookies(), "refresh_token")
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

	cookie := findCookie(rec.Result().Cookies(), "refresh_token")
	if cookie == nil {
		t.Fatal("expected refresh_token cookie in response")
	}
	if cookie.MaxAge != -1 {
		t.Errorf("expected MaxAge -1, got %d", cookie.MaxAge)
	}
	if cookie.Value != "" {
		t.Errorf("expected empty cookie value, got %q", cookie.Value)
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

			cookie := findCookie(rec.Result().Cookies(), "refresh_token")
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

func TestRegister_RefreshCookieAttributes(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	handler := NewHandler(mock, testSecret, true)

	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs("alice@example.com", pgxmock.AnyArg(), "Alice").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("user-uuid-1"))

	expectInsertRefreshToken(mock, "user-uuid-1")

	body := `{"email":"alice@example.com","password":"strongpass123","name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	cookie := findCookie(rec.Result().Cookies(), "refresh_token")
	if cookie == nil {
		t.Fatal("expected refresh_token cookie")
	}
	if cookie.Path != "/api/auth" {
		t.Errorf("expected path %q, got %q", "/api/auth", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Error("expected HttpOnly cookie")
	}
	if !cookie.Secure {
		t.Error("expected Secure cookie when secureCookies=true")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("expected SameSiteStrictMode, got %v", cookie.SameSite)
	}

	expectedMaxAge := int(RefreshTokenDuration / time.Second)
	if cookie.MaxAge != expectedMaxAge {
		t.Errorf("expected MaxAge %d, got %d", expectedMaxAge, cookie.MaxAge)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

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
