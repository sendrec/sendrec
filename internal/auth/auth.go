package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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

type Handler struct {
	db            database.DBTX
	jwtSecret     string
	secureCookies bool
}

func NewHandler(db database.DBTX, jwtSecret string, secureCookies bool) *Handler {
	return &Handler{db: db, jwtSecret: jwtSecret, secureCookies: secureCookies}
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

	accessToken, refreshToken, err := h.issueTokens(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	h.setRefreshTokenCookie(w, refreshToken)
	httputil.WriteJSON(w, http.StatusCreated, tokenResponse{AccessToken: accessToken})
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
	err := h.db.QueryRow(r.Context(),
		"SELECT id, password FROM users WHERE email = $1", req.Email,
	).Scan(&userID, &hashedPassword)
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid email or password")
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
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
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

func (h *Handler) setRefreshTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/auth",
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
