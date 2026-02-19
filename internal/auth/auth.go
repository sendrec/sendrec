package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/httputil"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userIDKey contextKey = "userID"

type EmailSender interface {
	SendPasswordReset(ctx context.Context, toEmail, toName, resetLink string) error
	SendConfirmation(ctx context.Context, toEmail, toName, confirmLink string) error
}

type Handler struct {
	db            database.DBTX
	jwtSecret     string
	secureCookies bool
	emailSender   EmailSender
	baseURL       string
}

func NewHandler(db database.DBTX, jwtSecret string, secureCookies bool) *Handler {
	return &Handler{db: db, jwtSecret: jwtSecret, secureCookies: secureCookies}
}

func (h *Handler) SetEmailSender(sender EmailSender, baseURL string) {
	h.emailSender = sender
	h.baseURL = baseURL
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken string `json:"accessToken"`
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type messageResponse struct {
	Message string `json:"message"`
}

const resetTokenExpiry = 1 * time.Hour
const confirmTokenExpiry = 24 * time.Hour

func generateSecureToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	return raw, hash, nil
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" || req.Name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "email, password, and name are required")
		return
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid email address")
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

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	var userID string
	err = h.db.QueryRow(r.Context(),
		"INSERT INTO users (email, password, name) VALUES ($1, $2, $3) RETURNING id",
		req.Email, string(hashedPassword), req.Name,
	).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httputil.WriteError(w, http.StatusConflict, "could not create account")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	if _, err := h.db.Exec(r.Context(),
		"UPDATE email_confirmations SET used_at = now() WHERE user_id = $1 AND used_at IS NULL",
		userID,
	); err != nil {
		log.Printf("register: failed to invalidate old confirmation tokens: %v", err)
	}

	rawToken, tokenHash, err := generateSecureToken()
	if err != nil {
		log.Printf("register: failed to generate confirmation token: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create account")
		return
	}

	if _, err := h.db.Exec(r.Context(),
		"INSERT INTO email_confirmations (token_hash, user_id, expires_at) VALUES ($1, $2, $3)",
		tokenHash, userID, time.Now().Add(confirmTokenExpiry),
	); err != nil {
		log.Printf("register: failed to store confirmation token: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create account")
		return
	}

	confirmLink := h.baseURL + "/confirm-email?token=" + rawToken
	if h.emailSender != nil {
		if err := h.emailSender.SendConfirmation(r.Context(), req.Email, req.Name, confirmLink); err != nil {
			log.Printf("register: failed to send confirmation email: %v", err)
		}
	}

	httputil.WriteJSON(w, http.StatusCreated, messageResponse{Message: "Account created. Check your email to confirm your address."})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		httputil.WriteError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	var userID, hashedPassword string
	var emailVerified bool
	err := h.db.QueryRow(r.Context(),
		"SELECT id, password, email_verified FROM users WHERE email = $1", req.Email,
	).Scan(&userID, &hashedPassword, &emailVerified)
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if !emailVerified {
		httputil.WriteJSON(w, http.StatusForbidden, map[string]string{
			"error": "email_not_verified",
		})
		return
	}

	accessToken, refreshToken, err := h.issueTokens(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	h.setRefreshTokenCookie(w, refreshToken)
	httputil.WriteJSON(w, http.StatusOK, tokenResponse{AccessToken: accessToken})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "refresh token not found")
		return
	}

	claims, err := ValidateToken(h.jwtSecret, cookie.Value)
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	if claims.TokenType != "refresh" {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	if claims.TokenID == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	if err := h.validateStoredRefreshToken(r.Context(), claims.UserID, claims.TokenID); err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	if err := h.revokeRefreshToken(r.Context(), claims.TokenID); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to revoke refresh token")
		return
	}

	accessToken, refreshToken, err := h.issueTokens(r.Context(), claims.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	h.setRefreshTokenCookie(w, refreshToken)
	httputil.WriteJSON(w, http.StatusOK, tokenResponse{AccessToken: accessToken})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		if claims, err := ValidateToken(h.jwtSecret, cookie.Value); err == nil && claims.TokenType == "refresh" && claims.TokenID != "" {
			_ = h.revokeRefreshToken(r.Context(), claims.TokenID)
		}
	}
	// Clear cookie at both current and legacy paths
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
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
		httputil.WriteJSON(w, http.StatusOK, response)
		return
	}

	if _, err := h.db.Exec(r.Context(),
		"UPDATE password_resets SET used_at = now() WHERE user_id = $1 AND used_at IS NULL",
		userID,
	); err != nil {
		log.Printf("forgot-password: failed to invalidate old tokens: %v", err)
	}

	rawToken, tokenHash, err := generateSecureToken()
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

	tokenHash := hashToken(req.Token)

	var userID string
	err := h.db.QueryRow(r.Context(),
		"SELECT user_id FROM password_resets WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()",
		tokenHash,
	).Scan(&userID)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid or expired reset link")
		return
	}

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

	if _, err := h.db.Exec(r.Context(),
		"UPDATE refresh_tokens SET revoked = true, revoked_at = now() WHERE user_id = $1 AND revoked = false",
		userID,
	); err != nil {
		log.Printf("reset-password: failed to revoke refresh tokens: %v", err)
	}

	httputil.WriteJSON(w, http.StatusOK, messageResponse{Message: "Password updated successfully"})
}

type confirmEmailRequest struct {
	Token string `json:"token"`
}

func (h *Handler) ConfirmEmail(w http.ResponseWriter, r *http.Request) {
	var req confirmEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Token == "" {
		httputil.WriteError(w, http.StatusBadRequest, "token is required")
		return
	}

	tokenHash := hashToken(req.Token)

	var userID string
	err := h.db.QueryRow(r.Context(),
		"SELECT user_id FROM email_confirmations WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()",
		tokenHash,
	).Scan(&userID)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid or expired confirmation link")
		return
	}

	if _, err := h.db.Exec(r.Context(),
		"UPDATE email_confirmations SET used_at = now() WHERE token_hash = $1",
		tokenHash,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to process confirmation")
		return
	}

	if _, err := h.db.Exec(r.Context(),
		"UPDATE users SET email_verified = true, updated_at = now() WHERE id = $1",
		userID,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to verify email")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, messageResponse{Message: "Email confirmed successfully. You can now sign in."})
}

type resendConfirmationRequest struct {
	Email string `json:"email"`
}

func (h *Handler) ResendConfirmation(w http.ResponseWriter, r *http.Request) {
	var req resendConfirmationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" {
		httputil.WriteError(w, http.StatusBadRequest, "email is required")
		return
	}

	response := messageResponse{Message: "If an unverified account with that email exists, we've sent a confirmation link"}

	var userID, userName string
	var emailVerified bool
	err := h.db.QueryRow(r.Context(),
		"SELECT id, name, email_verified FROM users WHERE email = $1", req.Email,
	).Scan(&userID, &userName, &emailVerified)
	if err != nil {
		httputil.WriteJSON(w, http.StatusOK, response)
		return
	}

	if emailVerified {
		httputil.WriteJSON(w, http.StatusOK, response)
		return
	}

	if _, err := h.db.Exec(r.Context(),
		"UPDATE email_confirmations SET used_at = now() WHERE user_id = $1 AND used_at IS NULL",
		userID,
	); err != nil {
		log.Printf("resend-confirmation: failed to invalidate old tokens: %v", err)
	}

	rawToken, tokenHash, err := generateSecureToken()
	if err != nil {
		log.Printf("resend-confirmation: failed to generate token: %v", err)
		httputil.WriteJSON(w, http.StatusOK, response)
		return
	}

	if _, err := h.db.Exec(r.Context(),
		"INSERT INTO email_confirmations (token_hash, user_id, expires_at) VALUES ($1, $2, $3)",
		tokenHash, userID, time.Now().Add(confirmTokenExpiry),
	); err != nil {
		log.Printf("resend-confirmation: failed to store token: %v", err)
		httputil.WriteJSON(w, http.StatusOK, response)
		return
	}

	confirmLink := h.baseURL + "/confirm-email?token=" + rawToken
	if h.emailSender != nil {
		if err := h.emailSender.SendConfirmation(r.Context(), req.Email, userName, confirmLink); err != nil {
			log.Printf("resend-confirmation: failed to send email: %v", err)
		}
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

type userResponse struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	var resp userResponse
	err := h.db.QueryRow(r.Context(),
		"SELECT name, email FROM users WHERE id = $1", userID,
	).Scan(&resp.Name, &resp.Email)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "user not found")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

type updateUserRequest struct {
	Name            string `json:"name"`
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := UserIDFromContext(r.Context())
	hasNameChange := req.Name != ""
	hasPasswordChange := req.NewPassword != ""

	if !hasNameChange && !hasPasswordChange {
		httputil.WriteError(w, http.StatusBadRequest, "nothing to update")
		return
	}

	if hasPasswordChange {
		if req.CurrentPassword == "" {
			httputil.WriteError(w, http.StatusBadRequest, "current password is required to set a new password")
			return
		}

		if len(req.NewPassword) < 8 {
			httputil.WriteError(w, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}

		if len(req.NewPassword) > 72 {
			httputil.WriteError(w, http.StatusBadRequest, "password must be at most 72 characters")
			return
		}

		var currentHash string
		err := h.db.QueryRow(r.Context(),
			"SELECT password FROM users WHERE id = $1", userID,
		).Scan(&currentHash)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to verify password")
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.CurrentPassword)); err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, "current password is incorrect")
			return
		}

		newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}

		if _, err := h.db.Exec(r.Context(),
			"UPDATE users SET password = $1, updated_at = now() WHERE id = $2",
			string(newHash), userID,
		); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update password")
			return
		}
	}

	if hasNameChange {
		if _, err := h.db.Exec(r.Context(),
			"UPDATE users SET name = $1, updated_at = now() WHERE id = $2",
			req.Name, userID,
		); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update name")
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, messageResponse{Message: "Settings updated"})
}

func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			httputil.WriteError(w, http.StatusUnauthorized, "authorization header required")
			return
		}

		tokenStr, found := strings.CutPrefix(authHeader, "Bearer ")
		if !found {
			httputil.WriteError(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		claims, err := ValidateToken(h.jwtSecret, tokenStr)
		if err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		if claims.TokenType != "access" {
			httputil.WriteError(w, http.StatusUnauthorized, "invalid token type")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserIDFromContext(ctx context.Context) string {
	userID, _ := ctx.Value(userIDKey).(string)
	return userID
}

func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func (h *Handler) setRefreshTokenCookie(w http.ResponseWriter, token string) {
	// Clear legacy cookie at old path to prevent duplicate cookies
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(RefreshTokenDuration / time.Second),
	})
}

func (h *Handler) issueTokens(ctx context.Context, userID string) (accessToken, refreshToken string, err error) {
	tokenID, err := newTokenID()
	if err != nil {
		return "", "", err
	}

	expiresAt := time.Now().Add(RefreshTokenDuration)
	if _, err := h.db.Exec(ctx, "INSERT INTO refresh_tokens (token_id, user_id, expires_at, revoked) VALUES ($1, $2, $3, false)", tokenID, userID, expiresAt); err != nil {
		return "", "", err
	}

	accessToken, err = GenerateAccessToken(h.jwtSecret, userID)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = GenerateRefreshToken(h.jwtSecret, userID, tokenID)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (h *Handler) validateStoredRefreshToken(ctx context.Context, userID, tokenID string) error {
	var revoked bool
	var expiresAt time.Time
	err := h.db.QueryRow(ctx, "SELECT revoked, expires_at FROM refresh_tokens WHERE token_id = $1 AND user_id = $2", tokenID, userID).Scan(&revoked, &expiresAt)
	if err != nil {
		return err
	}
	if revoked || time.Now().After(expiresAt) {
		return errors.New("token revoked or expired")
	}
	return nil
}

func (h *Handler) revokeRefreshToken(ctx context.Context, tokenID string) error {
	_, err := h.db.Exec(ctx, "UPDATE refresh_tokens SET revoked = true, revoked_at = now() WHERE token_id = $1", tokenID)
	return err
}

func newTokenID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
